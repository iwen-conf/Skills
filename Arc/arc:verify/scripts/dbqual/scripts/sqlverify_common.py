#!/usr/bin/env python3
from __future__ import annotations

import hashlib
import json
import math
import os
import operator
import platform
import re
import shutil
import signal
import subprocess
import sys
import time
import uuid
from pathlib import Path
from typing import Any, BinaryIO, Iterable, NotRequired, TypedDict

from jsonschema import Draft202012Validator

STATUS_VALUES = {"PASS", "FAIL", "BLOCKED", "NOT_RUN", "FLAKY"}
# Every runner stamps this run identity into its evidence; gates reject evidence from other runs.
RUN_ID_ENV = "SQLVERIFY_RUN_ID"
RUN_MANIFEST = "run-manifest.json"
# Arc's evidence store is written while the suite runs; it is never part of the candidate.
DEFAULT_FINGERPRINT_EXCLUDES = (".arc/artifacts",)
# Bytecode caches appear when the harness imports modules mid-run; they carry no candidate behavior.
_CACHE_EXCLUDE = ":(exclude,glob)**/__pycache__/**"
SCALAR_OPERATORS = {
    "==": operator.eq,
    "!=": operator.ne,
    ">": operator.gt,
    ">=": operator.ge,
    "<": operator.lt,
    "<=": operator.le,
}


class SessionOutcome(TypedDict):
    sqlstates: list[str]
    steps: list[dict[str, object]]
    error: NotRequired[str | None]
    cleanup_error: NotRequired[str]
    session: NotRequired[str]


def scalar_value(cursor) -> Any:
    row = cursor.fetchone()
    if not isinstance(row, (dict, tuple, list)) or len(row) != 1:
        raise ValueError("scalar oracle must return exactly one row and one column")
    if cursor.fetchone() is not None:
        raise ValueError("scalar oracle returned more than one row")
    return next(iter(row.values())) if isinstance(row, dict) else row[0]


def command_verdict(returncode: int, failure_returncodes, success_returncodes=(0,)) -> str:
    success = list(success_returncodes)
    failure = list(failure_returncodes)
    if (
        not success
        or any(type(code) is not int or code < 0 for code in success + failure)
        or set(success) & set(failure)
    ):
        raise ValueError("command success/failure results must be explicit, disjoint nonnegative integer exit codes")
    if type(returncode) is not int or returncode < 0:
        return "BLOCKED"
    return "PASS" if returncode in success else "FAIL" if returncode in failure else "BLOCKED"


def now_iso() -> str:
    import datetime as dt

    return dt.datetime.now(dt.timezone.utc).isoformat()


def _json_default(value: Any):
    import datetime as dt
    import decimal
    import uuid

    if isinstance(value, decimal.Decimal):
        return float(value)
    if isinstance(value, (dt.datetime, dt.date, dt.time)):
        return value.isoformat()
    if isinstance(value, uuid.UUID):
        return str(value)
    if isinstance(value, bytes):
        return value.hex()
    return str(value)


def json_dump(path: Path, data: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_suffix(path.suffix + ".tmp")
    tmp.write_text(
        json.dumps(data, ensure_ascii=False, indent=2, sort_keys=True, default=_json_default) + "\n", encoding="utf-8"
    )
    tmp.replace(path)


def json_load(path: Path) -> Any:
    def unique_fields(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
        result: dict[str, Any] = {}
        for key, value in pairs:
            if key in result:
                raise ValueError(f"duplicate JSON field: {key}")
            result[key] = value
        return result

    def invalid_constant(value: str) -> None:
        raise ValueError(f"non-finite JSON number: {value}")

    return json.loads(
        path.read_text(encoding="utf-8"), object_pairs_hook=unique_fields, parse_constant=invalid_constant
    )


_EVIDENCE_VALIDATOR = Draft202012Validator(
    json_load(Path(__file__).resolve().parents[1] / "schemas" / "evidence.schema.json")
)


def evidence_files(directory: Path) -> list[Path]:
    return sorted(
        p
        for p in directory.rglob("*")
        if p.is_file() and "superseded" not in p.relative_to(directory).parts and p.name != "baseline-manifest.json"
    )


def evidence_hashes(directory: Path) -> dict[str, str]:
    return {p.relative_to(directory).as_posix(): sha256_file(p) for p in evidence_files(directory)}


def sha256_bytes(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def sha256_file(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def sha256_tree(root: Path, include_suffixes: tuple[str, ...] | None = None) -> str:
    h = hashlib.sha256()
    for p in sorted(x for x in root.rglob("*") if x.is_file()):
        if include_suffixes and p.suffix not in include_suffixes:
            continue
        rel = p.relative_to(root).as_posix().encode()
        h.update(rel + b"\0")
        h.update(p.read_bytes())
        h.update(b"\0")
    return h.hexdigest()


def stop_process_tree(proc: subprocess.Popen[bytes]) -> str | None:
    errors: list[str] = []
    deadline = time.monotonic() + 5
    descendants: set[int] = {proc.pid}
    if os.name == "nt":
        try:
            stopped = subprocess.run(
                ["taskkill", "/PID", str(proc.pid), "/T", "/F"], capture_output=True, text=True, check=False, timeout=5
            )
            if stopped.returncode != 0:
                errors.append(f"taskkill returned {stopped.returncode}: {stopped.stderr.strip()}")
        except (OSError, subprocess.SubprocessError) as exc:
            errors.append(str(exc))
        if errors and proc.poll() is None:
            try:
                proc.kill()
            except OSError as exc:
                errors.append(str(exc))
    else:
        try:
            if proc.poll() is None:
                os.kill(proc.pid, signal.SIGSTOP)
                while time.monotonic() < deadline:
                    snapshot = subprocess.run(
                        ["ps", "-axo", "pid=,ppid="],
                        capture_output=True,
                        text=True,
                        check=True,
                        timeout=max(0.001, min(1, deadline - time.monotonic())),
                    )
                    parents = {
                        int(pid): int(parent) for pid, parent in (row.split() for row in snapshot.stdout.splitlines())
                    }
                    added = {pid for pid, parent in parents.items() if parent in descendants and pid not in descendants}
                    if not added:
                        break
                    descendants.update(added)
                    for pid in added:
                        try:
                            os.kill(pid, signal.SIGSTOP)
                        except ProcessLookupError:
                            pass
                else:
                    errors.append("process-tree capture exhausted the teardown budget")
        except ProcessLookupError:
            pass
        except (OSError, subprocess.SubprocessError, ValueError) as exc:
            errors.append(str(exc))
        for pid in descendants:
            try:
                os.kill(pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            except OSError as exc:
                errors.append(str(exc))
        try:
            os.killpg(proc.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        except OSError as exc:
            errors.append(str(exc))
    try:
        proc.wait(timeout=max(0, deadline - time.monotonic()))
    except subprocess.TimeoutExpired:
        errors.append("killed child could not be reaped within the teardown bound")
    return "; ".join(errors) or None


def wait_process(proc: subprocess.Popen[bytes], timeout: float) -> dict[str, Any]:
    if not math.isfinite(timeout):
        raise ValueError("process execution budget must be finite")
    timed_out = False
    cleanup_error = None
    residual = False
    try:
        proc.wait(timeout=max(0, timeout))
    except subprocess.TimeoutExpired:
        timed_out = True
    if timed_out:
        cleanup_error = stop_process_tree(proc)
    elif os.name == "posix":
        try:
            os.killpg(proc.pid, 0)
        except ProcessLookupError:
            pass
        except OSError as exc:
            cleanup_error = str(exc)
        else:
            residual = True
            cleanup_error = stop_process_tree(proc)
    return {
        "returncode": proc.returncode,
        "timeout": timed_out,
        "residual_processes": residual,
        "cleanup_error": cleanup_error,
        "blocked": timed_out or residual or cleanup_error is not None or proc.returncode is None or proc.returncode < 0,
    }


def output_tail(stream: BinaryIO) -> str:
    stream.seek(0, os.SEEK_END)
    stream.seek(max(0, stream.tell() - 3000))
    return stream.read().decode("utf-8", errors="replace")


def run(
    cmd: list[str],
    *,
    cwd: Path | None = None,
    env: dict[str, str] | None = None,
    timeout: float | None = None,
    check: bool = False,
) -> subprocess.CompletedProcess[str]:
    merged = os.environ.copy()
    if env:
        merged.update(env)
    import tempfile

    if timeout is not None and (not math.isfinite(timeout) or timeout <= 0):
        raise ValueError("command execution budget must be finite and positive")
    with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
        proc = subprocess.Popen(
            cmd, cwd=str(cwd) if cwd else None, env=merged, stdout=out, stderr=err, start_new_session=os.name != "nt"
        )
        if timeout is None:
            proc.wait()
            result = wait_process(proc, 0)
        else:
            result = wait_process(proc, timeout)
        out.seek(0)
        err.seek(0)
        stdout, stderr = out.read().decode("utf-8", errors="replace"), err.read().decode("utf-8", errors="replace")
    if result["timeout"] and timeout is not None:
        raise subprocess.TimeoutExpired(cmd, timeout, output=stdout, stderr=stderr)
    if result["blocked"]:
        raise RuntimeError(f"command interrupted or left unfinished descendants: {cmd}: {result}; stderr={stderr}")
    if proc.returncode is None:
        raise RuntimeError("command did not yield an exit code")
    cp = subprocess.CompletedProcess(cmd, proc.returncode, stdout, stderr)
    if check and cp.returncode != 0:
        raise RuntimeError(f"command failed ({cp.returncode}): {cmd}\nstdout:\n{cp.stdout}\nstderr:\n{cp.stderr}")
    return cp


def which(name: str) -> str | None:
    return shutil.which(name)


def git_metadata(root: Path) -> dict[str, Any]:
    if not which("git"):
        return {"available": False, "reason": "git executable not found"}
    if (
        not (root / ".git").exists()
        and run(["git", "-C", str(root), "rev-parse", "--is-inside-work-tree"]).returncode != 0
    ):
        return {"available": False}

    def g(*args: str) -> str | None:
        cp = run(["git", "-C", str(root), *args])
        return cp.stdout.strip() if cp.returncode == 0 else None

    return {
        "available": True,
        "commit": g("rev-parse", "HEAD"),
        "branch": g("rev-parse", "--abbrev-ref", "HEAD"),
        "dirty": bool(g("status", "--porcelain")),
    }


def host_metadata() -> dict[str, Any]:
    return {
        "platform": platform.platform(),
        "machine": platform.machine(),
        "processor": platform.processor(),
        "python": sys.version.split()[0],
        "cpu_count": os.cpu_count(),
    }


def percentile(values: list[float], p: float) -> float | None:
    if not values:
        return None
    vals = sorted(values)
    if len(vals) == 1:
        return vals[0]
    k = (len(vals) - 1) * p
    lo = int(k)
    hi = min(lo + 1, len(vals) - 1)
    frac = k - lo
    return vals[lo] * (1 - frac) + vals[hi] * frac


def median(values: list[float]) -> float | None:
    return percentile(values, 0.5)


def mean(values: list[float]) -> float | None:
    return sum(values) / len(values) if values else None


def ratio_error(estimated: float | int | None, actual: float | int | None) -> float | None:
    if estimated is None or actual is None:
        return None
    e = float(estimated)
    a = float(actual)
    if e == 0 and a == 0:
        return 1.0
    if e <= 0 or a <= 0:
        return float("inf")
    return max(e / a, a / e)


def redact_dsn(dsn: str) -> str:
    # URI style
    dsn = re.sub(r"(postgres(?:ql)?://[^:/@\s]+:)[^@\s]+@", r"\1***@", dsn)
    # keyword style
    dsn = re.sub(r"(?i)(password\s*=\s*)('[^']*'|\"[^\"]*\"|[^\s]+)", r"\1***", dsn)
    return dsn


def evidence_base(kind: str, evidence_id: str, *, command: list[str] | None = None) -> dict[str, Any]:
    return {
        "schema_version": 1,
        "kind": kind,
        "id": evidence_id,
        "run_id": os.environ.get(RUN_ID_ENV),
        "status": "NOT_RUN",
        "started_at": now_iso(),
        "finished_at": None,
        "elapsed_ms": None,
        "command": command or [],
        "observed": {},
        "violations": [],
        "warnings": [],
        "reproduction": {},
    }


def finalize_evidence(ev: dict[str, Any], started_monotonic: float, status: str) -> dict[str, Any]:
    if status not in STATUS_VALUES:
        raise ValueError(f"invalid status: {status}")
    ev["status"] = status
    ev["finished_at"] = now_iso()
    ev["elapsed_ms"] = round((time.monotonic() - started_monotonic) * 1000, 3)
    return ev


def safe_relpath(path: Path, root: Path) -> str:
    try:
        return path.resolve().relative_to(root.resolve()).as_posix()
    except Exception:
        return str(path.resolve())


def _git_bytes(cwd: Path, *args: str) -> bytes:
    env = os.environ.copy()
    env["GIT_LITERAL_PATHSPECS"] = "0"  # the exclude pathspecs use pathspec magic
    cp = subprocess.run(["git", "-C", str(cwd), *args], env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if cp.returncode != 0:
        raise RuntimeError(f"git {args[0]} failed: {cp.stderr.decode('utf-8', 'replace').strip()}")
    return cp.stdout


def _path_digest(path: Path) -> str:
    if path.is_symlink():
        return "link:" + sha256_bytes(os.fsencode(os.readlink(path)))
    return sha256_file(path) if path.is_file() else "missing"


def candidate_fingerprint(root: Path, config_files: Iterable[str], exclude: Iterable[str]) -> dict[str, Any]:
    """Identity of the code and configuration under test; equal fingerprints mean the same candidate.

    HEAD plus the content digest of every changed or untracked (not ignored) path in the repository.
    config_files and exclude are paths relative to root (absolute paths stay absolute).
    """
    root = root.resolve()
    meta = git_metadata(root)
    out: dict[str, Any] = {
        "git_commit": meta.get("commit") if meta.get("available") else None,
        "worktree_changes": {},
        "config_sha256": {},
    }
    if out["git_commit"]:
        top = Path(_git_bytes(root, "rev-parse", "--show-toplevel").decode().strip()).resolve()
        spec = ["--", ".", _CACHE_EXCLUDE]
        for item in exclude:
            target = (root / item).resolve()
            if target == top or top in target.parents:
                spec.append(":(exclude)" + target.relative_to(top).as_posix())
        changed = _git_bytes(top, "diff", "--name-only", "--no-renames", "-z", "HEAD", *spec)
        untracked = _git_bytes(top, "ls-files", "--others", "--exclude-standard", "-z", *spec)
        for rel in sorted({os.fsdecode(x) for x in (changed + b"\0" + untracked).split(b"\0") if x}):
            out["worktree_changes"][rel] = _path_digest(top / rel)
    for name in config_files:
        out["config_sha256"][name] = _path_digest(root / name)
    return out


def validate_output_paths(root: Path, inputs: Iterable[Path], outputs: Iterable[Path]) -> None:
    root = root.resolve()
    reserved = [p.resolve() for p in inputs]
    destinations = sorted({p.resolve() for p in outputs})
    for output in destinations:
        if root == output or root.is_relative_to(output):
            raise ValueError(f"output directory overlaps the candidate root: {output}")
        for source in reserved:
            if source == output or source.is_relative_to(output) or output.is_relative_to(source):
                raise ValueError(f"output directory overlaps an input: {output}")
    for i, output in enumerate(destinations):
        for other in destinations[i + 1 :]:
            if output.is_relative_to(other) or other.is_relative_to(output):
                raise ValueError(f"output directories overlap: {output} and {other}")
    meta = git_metadata(root)
    if meta.get("available"):
        top = Path(_git_bytes(root, "rev-parse", "--show-toplevel").decode().strip()).resolve()
        for name in _git_bytes(top, "ls-files", "-z").split(b"\0"):
            if name:
                tracked = top / os.fsdecode(name)
                if any(tracked.is_relative_to(output) for output in destinations):
                    raise ValueError(f"output directory contains a tracked candidate file: {tracked}")


def validate_output_owner(evidence_dir: Path) -> None:
    if not evidence_dir.exists():
        return
    if not evidence_dir.is_dir():
        raise ValueError(f"output path is not a directory: {evidence_dir}")
    leftovers = sorted(p for p in evidence_dir.iterdir() if p.name != "superseded")
    if not leftovers:
        return
    manifest = evidence_dir / RUN_MANIFEST
    owner = json_load(manifest) if manifest.is_file() else None
    if not isinstance(owner, dict) or owner.get("output_owner") != "sqlverify-v1":
        raise ValueError(f"nonempty output directory has no SQLVerify ownership manifest: {evidence_dir}")
    owned = {owner.get("evidence_dir"), owner.get("report_dir")}
    previous = owner.get("run_id")
    if (
        str(evidence_dir.resolve()) not in owned
        or not isinstance(previous, str)
        or not re.fullmatch(r"[0-9A-Za-z_-]{1,128}", previous)
    ):
        raise ValueError(f"output ownership does not match this directory: {evidence_dir}")
    history = evidence_dir / "superseded"
    if history.is_symlink() or history.exists() and not history.is_dir():
        raise ValueError(f"output history is not a safe directory: {history}")


def archive_previous_evidence(evidence_dir: Path) -> Path | None:
    """Move owned earlier output aside, so no gate reads it as current."""
    validate_output_owner(evidence_dir)
    leftovers = sorted(p for p in evidence_dir.iterdir() if p.name != "superseded")
    if not leftovers:
        return None
    previous = json_load(evidence_dir / RUN_MANIFEST)["run_id"]
    stamp = re.sub(r"[^0-9A-Za-z]", "", now_iso())[:21]
    dest = evidence_dir / "superseded" / f"{stamp}-{previous or 'unbound'}"
    dest.mkdir(parents=True)
    for p in leftovers:
        p.replace(dest / p.name)
    return dest


def begin_run(
    evidence_dir: Path,
    root: Path,
    config_files: Iterable[Path],
    exclude: Iterable[Path],
    *,
    report_dir: Path | None = None,
) -> dict[str, Any]:
    """Archive earlier evidence, then write the manifest binding this run to its candidate."""
    root = root.resolve()
    evidence_dir = evidence_dir.resolve()
    config_files = list(config_files)
    exclude = list(exclude)
    outputs = [evidence_dir, *exclude, *([report_dir] if report_dir else [])]
    validate_output_paths(root, config_files, outputs)
    if report_dir:
        report_dir = report_dir.resolve()
    for output in [evidence_dir, *([report_dir] if report_dir else [])]:
        validate_output_owner(output)
    configs = [safe_relpath(p, root) for p in config_files]
    excludes = sorted({*DEFAULT_FINGERPRINT_EXCLUDES, *(safe_relpath(p, root) for p in exclude)})
    candidate = candidate_fingerprint(root, configs, excludes)
    evidence_dir.mkdir(parents=True, exist_ok=True)
    archived = archive_previous_evidence(evidence_dir)
    report_archive = None
    if report_dir:
        report_dir.mkdir(parents=True, exist_ok=True)
        report_archive = archive_previous_evidence(report_dir)
    manifest = {
        "schema_version": 1,
        "output_owner": "sqlverify-v1",
        "root": str(root),
        "evidence_dir": str(evidence_dir),
        "report_dir": str(report_dir) if report_dir else None,
        "run_id": uuid.uuid4().hex,
        "created_at": now_iso(),
        "config_files": configs,
        "fingerprint_exclude": excludes,
        "candidate": candidate,
        "archived_previous_evidence": str(archived) if archived else None,
        "archived_previous_reports": str(report_archive) if report_archive else None,
    }
    json_dump(evidence_dir / RUN_MANIFEST, manifest)
    if report_dir:
        json_dump(report_dir / RUN_MANIFEST, manifest)
    return manifest


def load_run_manifest(evidence_dir: Path) -> tuple[dict[str, Any] | None, list[dict[str, Any]]]:
    path = evidence_dir / RUN_MANIFEST
    if not path.is_file():
        return None, [
            {
                "type": "missing_run_manifest",
                "path": str(path),
                "detail": "evidence is not bound to a run; produce it with run_suite.py",
            }
        ]
    try:
        manifest = json_load(path)
    except Exception as exc:
        return None, [{"type": "unreadable_run_manifest", "path": str(path), "error": repr(exc)}]
    if not isinstance(manifest, dict) or not manifest.get("run_id"):
        return None, [{"type": "unreadable_run_manifest", "path": str(path), "error": "run_id missing"}]
    return manifest, []


def verify_candidate(manifest: dict[str, Any], root: Path) -> list[dict[str, Any]]:
    """Recompute the candidate fingerprint; evidence of another commit, work tree or config is stale."""
    recorded = manifest.get("candidate") or {}
    try:
        current = candidate_fingerprint(root, manifest.get("config_files", []), manifest.get("fingerprint_exclude", []))
    except Exception as exc:
        return [{"type": "candidate_unidentified", "detail": repr(exc)}]
    if not current["git_commit"] or not recorded.get("git_commit"):
        return [
            {
                "type": "candidate_unidentified",
                "detail": "candidate is not a git work tree; evidence cannot be bound to a commit",
            }
        ]
    changed = sorted(
        k for k in ("git_commit", "worktree_changes", "config_sha256") if current.get(k) != recorded.get(k)
    )
    if not changed:
        return []
    paths = {}
    for key in ("worktree_changes", "config_sha256"):
        before, after = recorded.get(key) or {}, current.get(key) or {}
        paths.update(
            {
                p: {"recorded": before.get(p), "current": after.get(p)}
                for p in sorted(set(before) | set(after))
                if before.get(p) != after.get(p)
            }
        )
    return [
        {
            "type": "candidate_mismatch",
            "changed": changed,
            "git_commit": {"recorded": recorded.get("git_commit"), "current": current["git_commit"]},
            "paths": paths,
        }
    ]


def load_evidence_dir(
    directory: Path, *, run_id: str | None, skip_ids: Iterable[str] = ()
) -> tuple[list[dict[str, Any]], list[dict[str, Any]]]:
    """Read evidence records; unreadable, duplicate and other-run files are reported, never skipped silently.

    run_id=None reads a directory whose integrity is checked elsewhere (a hashed baseline).
    """
    skip = set(skip_ids)
    evidence: list[dict[str, Any]] = []
    issues: list[dict[str, Any]] = []
    seen: dict[str, str] = {}
    for p in sorted(directory.glob("*.json")):
        if p.name in {RUN_MANIFEST, "baseline-manifest.json"}:
            continue
        try:
            obj = json_load(p)
        except Exception as exc:
            issues.append({"type": "unreadable_evidence", "file": p.name, "error": repr(exc)})
            continue
        problems = list(_EVIDENCE_VALIDATOR.iter_errors(obj))
        if problems:
            issues.append(
                {"type": "invalid_evidence", "file": p.name, "errors": [problem.message for problem in problems]}
            )
            continue
        eid = obj["id"]
        if eid in skip:
            continue
        if eid in seen:
            issues.append({"type": "duplicate_evidence_id", "id": eid, "files": [seen[eid], p.name]})
            continue
        seen[eid] = p.name
        if run_id is not None and obj.get("run_id") != run_id:
            issues.append(
                {
                    "type": "stale_evidence",
                    "id": eid,
                    "file": p.name,
                    "run_id": obj.get("run_id"),
                    "expected_run_id": run_id,
                }
            )
            continue
        evidence.append(obj)
    return evidence, issues


def require_test_database_name(dbname: str, pattern: str, allow_destructive: bool) -> None:
    if re.search(pattern, dbname):
        return
    if allow_destructive and os.environ.get("SQLVERIFY_ALLOW_DESTRUCTIVE") == "1":
        return
    raise RuntimeError(
        f"database '{dbname}' does not match safe_database_pattern={pattern!r}; "
        "refusing potentially destructive test. Use an isolated *_test/*_bench database, or set both "
        "allow_destructive=true and SQLVERIFY_ALLOW_DESTRUCTIVE=1 explicitly."
    )
