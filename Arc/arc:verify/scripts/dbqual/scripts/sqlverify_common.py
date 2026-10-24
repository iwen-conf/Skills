#!/usr/bin/env python3
from __future__ import annotations

import hashlib
import json
import os
import platform
import re
import shutil
import subprocess
import sys
import time
import uuid
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Iterable

STATUS_VALUES = {"PASS", "FAIL", "BLOCKED", "NOT_RUN", "FLAKY"}
# Every runner stamps this run identity into its evidence; gates reject evidence from other runs.
RUN_ID_ENV = "SQLVERIFY_RUN_ID"
RUN_MANIFEST = "run-manifest.json"
# Arc's evidence store is written while the suite runs; it is never part of the candidate.
DEFAULT_FINGERPRINT_EXCLUDES = (".arc/artifacts",)
# Bytecode caches appear when the harness imports modules mid-run; they carry no candidate behavior.
_CACHE_EXCLUDE = ":(exclude,glob)**/__pycache__/**"


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
    tmp.write_text(json.dumps(data, ensure_ascii=False, indent=2, sort_keys=True, default=_json_default) + "\n", encoding="utf-8")
    tmp.replace(path)


def json_load(path: Path) -> Any:
    return json.loads(path.read_text(encoding="utf-8"))


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


def run(cmd: list[str], *, cwd: Path | None = None, env: dict[str, str] | None = None, timeout: float | None = None, check: bool = False) -> subprocess.CompletedProcess[str]:
    merged = os.environ.copy()
    if env:
        merged.update(env)
    cp = subprocess.run(cmd, cwd=str(cwd) if cwd else None, env=merged, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=timeout)
    if check and cp.returncode != 0:
        raise RuntimeError(f"command failed ({cp.returncode}): {cmd}\nstdout:\n{cp.stdout}\nstderr:\n{cp.stderr}")
    return cp


def which(name: str) -> str | None:
    return shutil.which(name)


def git_metadata(root: Path) -> dict[str, Any]:
    if not which("git"):
        return {"available": False, "reason": "git executable not found"}
    if not (root / ".git").exists() and run(["git", "-C", str(root), "rev-parse", "--is-inside-work-tree"]).returncode != 0:
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
    out: dict[str, Any] = {"git_commit": meta.get("commit") if meta.get("available") else None,
                           "worktree_changes": {}, "config_sha256": {}}
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


def archive_previous_evidence(evidence_dir: Path) -> Path | None:
    """Move everything an earlier run left in evidence_dir aside, so no gate reads it as current."""
    leftovers = sorted(p for p in evidence_dir.iterdir() if p.name != "superseded")
    if not leftovers:
        return None
    previous = None
    manifest = evidence_dir / RUN_MANIFEST
    if manifest.is_file():
        try:
            previous = json_load(manifest).get("run_id")
        except Exception:
            previous = None
    stamp = re.sub(r"[^0-9A-Za-z]", "", now_iso())[:21]
    dest = evidence_dir / "superseded" / f"{stamp}-{previous or 'unbound'}"
    dest.mkdir(parents=True)
    for p in leftovers:
        p.replace(dest / p.name)
    return dest


def begin_run(evidence_dir: Path, root: Path, config_files: Iterable[Path], exclude: Iterable[Path]) -> dict[str, Any]:
    """Archive earlier evidence, then write the manifest binding this run to its candidate."""
    root = root.resolve()
    evidence_dir.mkdir(parents=True, exist_ok=True)
    archived = archive_previous_evidence(evidence_dir)
    configs = [safe_relpath(p, root) for p in config_files]
    excludes = sorted({*DEFAULT_FINGERPRINT_EXCLUDES, *(safe_relpath(p, root) for p in exclude)})
    manifest = {
        "schema_version": 1,
        "run_id": uuid.uuid4().hex,
        "created_at": now_iso(),
        "config_files": configs,
        "fingerprint_exclude": excludes,
        "candidate": candidate_fingerprint(root, configs, excludes),
        "archived_previous_evidence": str(archived) if archived else None,
    }
    json_dump(evidence_dir / RUN_MANIFEST, manifest)
    return manifest


def load_run_manifest(evidence_dir: Path) -> tuple[dict[str, Any] | None, list[dict[str, Any]]]:
    path = evidence_dir / RUN_MANIFEST
    if not path.is_file():
        return None, [{"type": "missing_run_manifest", "path": str(path),
                       "detail": "evidence is not bound to a run; produce it with run_suite.py"}]
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
        return [{"type": "candidate_unidentified", "detail": "candidate is not a git work tree; evidence cannot be bound to a commit"}]
    changed = sorted(k for k in ("git_commit", "worktree_changes", "config_sha256") if current.get(k) != recorded.get(k))
    if not changed:
        return []
    paths = {}
    for key in ("worktree_changes", "config_sha256"):
        before, after = recorded.get(key) or {}, current.get(key) or {}
        paths.update({p: {"recorded": before.get(p), "current": after.get(p)}
                      for p in sorted(set(before) | set(after)) if before.get(p) != after.get(p)})
    return [{"type": "candidate_mismatch", "changed": changed, "git_commit": {"recorded": recorded.get("git_commit"),
             "current": current["git_commit"]}, "paths": paths}]


def load_evidence_dir(directory: Path, *, run_id: str | None, skip_ids: Iterable[str] = ()) -> tuple[list[dict[str, Any]], list[dict[str, Any]]]:
    """Read evidence records; unreadable, duplicate and other-run files are reported, never skipped silently.

    run_id=None reads a directory whose integrity is checked elsewhere (a hashed baseline).
    """
    skip = set(skip_ids)
    evidence: list[dict[str, Any]] = []
    issues: list[dict[str, Any]] = []
    seen: dict[str, str] = {}
    for p in sorted(directory.glob("*.json")):
        if p.name == RUN_MANIFEST:
            continue
        try:
            obj = json_load(p)
        except Exception as exc:
            issues.append({"type": "unreadable_evidence", "file": p.name, "error": repr(exc)})
            continue
        if not isinstance(obj, dict) or not obj.get("id") or "status" not in obj:
            continue  # auxiliary JSON, not an evidence record
        eid = obj["id"]
        if eid in skip:
            continue
        if eid in seen:
            issues.append({"type": "duplicate_evidence_id", "id": eid, "files": [seen[eid], p.name]})
            continue
        seen[eid] = p.name
        if run_id is not None and obj.get("run_id") != run_id:
            issues.append({"type": "stale_evidence", "id": eid, "file": p.name,
                           "run_id": obj.get("run_id"), "expected_run_id": run_id})
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
