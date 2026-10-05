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
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Iterable

STATUS_VALUES = {"PASS", "FAIL", "BLOCKED", "NOT_RUN", "FLAKY"}


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
