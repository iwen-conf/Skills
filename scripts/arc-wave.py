#!/usr/bin/env python3
"""Code + .arc/arc.db wave compiler and stateless cross-agent handoff CLI.

Python 3 standard library only. Works alongside `arc` (0.5+) and `git`.
Subcommands:
  arc-wave status  [--repo DIR] [--json]
  arc-wave plan    [--repo DIR] [--json] <anchor_or_path>...
  arc-wave handoff [--repo DIR] --task ID --agent NAME [--note TEXT] [--scope p1,p2]
"""

from __future__ import annotations

import argparse
import contextlib
import datetime as dt
import json
import os
import re
import shutil
import sqlite3
import subprocess
from pathlib import Path
from typing import Any

CONTRACT_RE = re.compile(
    r"(?:openapi|swagger).*\.(?:ya?ml|json)$|\.proto$|migrations?/.*\.sql$|schema\.sql$",
    re.IGNORECASE,
)
HUB_NAMES = {
    "main.go",
    "router.go",
    "routes.go",
    "wire.go",
    "wire_gen.go",
    "package.json",
    "pnpm-lock.yaml",
    "go.mod",
    "go.sum",
    "Cargo.toml",
    "index.ts",
    "index.tsx",
    "App.tsx",
    "App.swift",
}
SKIP_DIRS = {
    ".git",
    "node_modules",
    ".arc",
    ".venv",
    ".ai-code-index",
    "vendor",
    "dist",
    "build",
    "__pycache__",
    ".mypy_cache",
    ".pytest_cache",
    ".ruff_cache",
}
GO_IMPORT_SINGLE_RE = re.compile(r'^\s*import\s+(?:[A-Za-z_.\w]*\s+)?"([^"]+)"', re.M)
GO_IMPORT_BLOCK_RE = re.compile(r"^\s*import\s*\((.*?)\)", re.M | re.S)
GO_IMPORT_LINE_RE = re.compile(r'^\s*(?:[A-Za-z_.\w]*\s+)?"([^"]+)"', re.M)
JS_IMPORT_RE = re.compile(
    r'^\s*import\s+(?:[\w*{}\s,]+\s+from\s+)?["\']([^"\']+)["\']|require\(\s*["\']([^"\']+)["\']\s*\)',
    re.M,
)
PY_IMPORT_RE = re.compile(
    r"^\s*(?:from\s+(\.+[\w.]*|[\w.]+)\s+import\b|import\s+([\w.]+))", re.M
)


def utc_now() -> str:
    return dt.datetime.now(dt.timezone.utc).isoformat().replace("+00:00", "Z")


def run_quiet(cmd: list[str], cwd: Path) -> tuple[int, str]:
    try:
        proc = subprocess.run(
            cmd,
            cwd=str(cwd),
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=15,
            check=False,
        )
        return proc.returncode, (proc.stdout or "").strip()
    except Exception:
        return 127, ""


def find_repo_root(start: Path) -> Path:
    cur = start.resolve()
    for candidate in (cur, *cur.parents):
        if (candidate / ".git").exists():
            return candidate
    for candidate in (cur, *cur.parents):
        if (candidate / ".arc").is_dir():
            return candidate
    return cur


def is_arc_active(root: Path) -> bool:
    manifest = root / ".arc" / "arc.json"
    if not manifest.is_file():
        return False
    try:
        data = json.loads(manifest.read_text(encoding="utf-8"))
        return bool(data.get("active", True))
    except Exception:
        return False


def read_arc_db(root: Path, task_id: str | None = None) -> dict[str, Any]:
    db_path = root / ".arc" / "arc.db"
    active = is_arc_active(root)
    info: dict[str, Any] = {
        "present": db_path.is_file(),
        "active": active,
        "tasks": [],
        "pending_outbox": 0,
        "recent_events": [],
    }
    if not db_path.is_file():
        return info
    try:
        uri = f"{db_path.resolve().as_uri()}?mode=ro"
        with contextlib.closing(sqlite3.connect(uri, uri=True, timeout=2.0)) as conn:
            conn.row_factory = sqlite3.Row
            tables = {
                r[0]
                for r in conn.execute(
                    "SELECT name FROM sqlite_master WHERE type='table'"
                )
            }
            if "arc_tasks" in tables:
                cols = {r[1] for r in conn.execute("PRAGMA table_info(arc_tasks)")}
                sel = [
                    c
                    for c in ("id", "title", "status", "version", "updated")
                    if c in cols
                ]
                if sel:
                    rows = conn.execute(
                        f"SELECT {', '.join(sel)} FROM arc_tasks "
                        f"ORDER BY CASE WHEN status != 'done' THEN 0 ELSE 1 END, rowid DESC LIMIT 50"
                    ).fetchall()
                    info["tasks"] = [dict(r) for r in rows]
                    if task_id:
                        row = conn.execute(
                            f"SELECT {', '.join(sel)} FROM arc_tasks WHERE id=?",
                            (task_id,),
                        ).fetchone()
                        info["requested_task"] = dict(row) if row else None
            if "arc_outbox" in tables:
                row = conn.execute(
                    "SELECT COUNT(*) FROM arc_outbox WHERE status='pending'"
                ).fetchone()
                info["pending_outbox"] = int(row[0]) if row else 0
            if "arc_events" in tables:
                rows = conn.execute(
                    "SELECT id, aggregate, aggregate_id, kind, created FROM arc_events ORDER BY rowid DESC LIMIT 8"
                ).fetchall()
                info["recent_events"] = [dict(r) for r in rows]
    except sqlite3.Error as exc:
        info["db_warning"] = str(exc)
    return info


def read_handoffs(root: Path) -> list[dict[str, Any]]:
    hdir = root / ".arc" / "handoff"
    if not hdir.is_dir():
        return []
    packets: list[dict[str, Any]] = []
    for p in sorted(hdir.glob("*.json"), key=lambda x: x.stat().st_mtime, reverse=True)[
        :10
    ]:
        try:
            packets.append(json.loads(p.read_text(encoding="utf-8")))
        except Exception:
            continue
    return packets


def latest_gate_report(root: Path) -> str | None:
    art = root / ".arc" / "artifacts"
    if not art.is_dir():
        return None
    reports = sorted(
        art.glob("*/reports/gate-*.txt"), key=lambda x: x.stat().st_mtime, reverse=True
    )
    return str(reports[0].relative_to(root)) if reports else None


def cmd_status(args: argparse.Namespace) -> int:
    root = find_repo_root(Path(args.repo))
    _, branch = run_quiet(["git", "rev-parse", "--abbrev-ref", "HEAD"], root)
    _, head = run_quiet(["git", "rev-parse", "--short", "HEAD"], root)
    _, status_lines = run_quiet(["git", "status", "--porcelain", "-uall"], root)
    _, wt_raw = run_quiet(["git", "worktree", "list", "--porcelain"], root)
    worktrees = [
        line.split(" ", 1)[1]
        for line in wt_raw.splitlines()
        if line.startswith("worktree ")
    ]
    dirty = [line for line in status_lines.splitlines() if line.strip()]
    db_info = read_arc_db(root)
    handoffs = read_handoffs(root)
    payload = {
        "repo_root": str(root),
        "git": {
            "branch": branch or "UNBORN",
            "head": head or "NONE",
            "dirty_count": len(dirty),
            "dirty": dirty[:30],
        },
        "worktrees": worktrees,
        "arc_cli": shutil.which("arc") is not None,
        "arc_db": db_info,
        "handoffs": handoffs,
        "latest_gate_report": latest_gate_report(root),
    }
    if args.json:
        print(json.dumps(payload, ensure_ascii=False, indent=2))
        return 0
    print(
        f"Repo: {root} ({payload['git']['branch']} @ {payload['git']['head']}, dirty={len(dirty)}, worktrees={len(worktrees)})"
    )
    print(
        f"arc.db: {'active' if db_info['present'] else 'absent'} | pending_outbox={db_info['pending_outbox']}"
    )
    for t in [x for x in db_info["tasks"] if x.get("status") != "done"][:8]:
        print(
            f"  - {t.get('id')} [{t.get('status')} v{t.get('version', '?')}]: {t.get('title', '')}"
        )
    if handoffs:
        h = handoffs[0]
        print(
            f"Latest Handoff: task={h.get('task_id')} from={h.get('agent')} at={h.get('timestamp')} head={h.get('head')}"
        )
        if h.get("note"):
            print(f"  Note: {h['note']}")
        if h.get("write_scope"):
            print(f"  Scope: {', '.join(h['write_scope'])}")
    if payload["latest_gate_report"]:
        print(f"Latest Gate: {payload['latest_gate_report']}")
    return 0


def sibling_tests_for(rel_file: Path, root: Path) -> list[str]:
    parent = root / rel_file.parent
    if not parent.is_dir():
        return []
    stem = rel_file.stem
    targets = {
        f"{stem}_test.go",
        f"test_{stem}.py",
        f"{stem}_test.py",
        f"{stem}.test.ts",
        f"{stem}.test.tsx",
        f"{stem}.spec.ts",
    }
    return sorted(
        str(c.relative_to(root))
        for c in parent.iterdir()
        if c.is_file() and c.name in targets
    )


def resolve_anchor_files(anchor: str, root: Path) -> list[str]:
    resolved_root = root.resolve()
    cand_path = Path(anchor).expanduser()
    if not cand_path.is_absolute():
        cwd_cand = Path.cwd() / cand_path
        root_cand = resolved_root / cand_path
        cand_path = cwd_cand if cwd_cand.exists() else root_cand
    p: Path | None = None
    try:
        cand_resolved = cand_path.resolve()
        cand_resolved.relative_to(resolved_root)
        p = cand_resolved
    except (OSError, ValueError):
        p = None
    files: set[str] = set()
    if p is not None and p.is_file():
        rel = p.relative_to(resolved_root)
        return sorted({str(rel), *sibling_tests_for(rel, resolved_root)})
    if p is not None and p.is_dir():
        return sorted(
            str(f.relative_to(resolved_root))
            for f in p.rglob("*")
            if f.is_file()
            and not any(
                part in SKIP_DIRS or part.startswith(".")
                for part in f.relative_to(resolved_root).parts
            )
        )
    if shutil.which("arc"):
        rc, out = run_quiet(["arc", "idx", "files", anchor], resolved_root)
        if rc == 0 and out:
            for line in out.splitlines():
                cand = resolved_root / line.strip()
                if cand.is_file():
                    rel = cand.relative_to(resolved_root)
                    files.update({str(rel), *sibling_tests_for(rel, resolved_root)})
    if not files:
        rc, out = run_quiet(["git", "grep", "-l", "-F", anchor], resolved_root)
        if rc == 0 and out:
            for line in out.splitlines()[:15]:
                cand = resolved_root / line.strip()
                if cand.is_file():
                    rel = cand.relative_to(resolved_root)
                    files.update({str(rel), *sibling_tests_for(rel, resolved_root)})
    if not files:
        for cand in sorted(resolved_root.rglob("*")):
            if not cand.is_file() or cand.stat().st_size > 256_000:
                continue
            if any(
                pt.startswith(".") or pt in SKIP_DIRS
                for pt in cand.relative_to(resolved_root).parts
            ):
                continue
            if anchor in cand.read_text(encoding="utf-8", errors="ignore"):
                rel = cand.relative_to(resolved_root)
                files.update({str(rel), *sibling_tests_for(rel, resolved_root)})
                if len(files) >= 15:
                    break
    return sorted(files) or [anchor]


def resolve_py_import(raw_mod: str, file_dir: Path) -> str:
    if raw_mod.startswith("."):
        dots = len(raw_mod) - len(raw_mod.lstrip("."))
        tail = raw_mod[dots:].replace(".", "/").strip("/")
        base = file_dir
        for _ in range(max(0, dots - 1)):
            if base == Path(".") or base == base.parent:
                break
            base = base.parent
        joined = (base / tail).as_posix() if tail else base.as_posix()
        return "" if joined == "." else joined.strip("/")
    return raw_mod.replace(".", "/").strip("/")


def extract_local_imports(files: list[str], root: Path) -> set[str]:
    tokens: set[str] = set()
    for rel in files:
        fp = root / rel
        if not fp.is_file() or fp.stat().st_size > 512_000:
            continue
        txt = fp.read_text(encoding="utf-8", errors="ignore")
        ext = fp.suffix.lower()
        file_dir = Path(rel).parent
        if ext == ".go":
            for m in GO_IMPORT_SINGLE_RE.findall(txt):
                val = m.strip().strip("/")
                if val:
                    tokens.add(val)
            for block in GO_IMPORT_BLOCK_RE.findall(txt):
                for m in GO_IMPORT_LINE_RE.findall(block):
                    val = m.strip().strip("/")
                    if val:
                        tokens.add(val)
        elif ext in {".py", ".pyi"}:
            for from_mod, imp_mod in PY_IMPORT_RE.findall(txt):
                raw_mod = (from_mod or imp_mod).strip()
                resolved = resolve_py_import(raw_mod, file_dir)
                if resolved:
                    tokens.add(resolved)
        else:
            for m1, m2 in JS_IMPORT_RE.findall(txt):
                raw = (m1 or m2).strip()
                if raw.startswith("."):
                    norm = os.path.normpath((file_dir / raw).as_posix()).lstrip("./")
                    if norm and norm != ".":
                        tokens.add(norm)
                elif raw:
                    tokens.add(raw.strip("/"))
    return {tok for tok in tokens if tok}


def cmd_plan(args: argparse.Namespace) -> int:
    root = find_repo_root(Path(args.repo))
    raw_units: list[dict[str, Any]] = []
    for idx, anchor in enumerate(args.anchors):
        matched = resolve_anchor_files(anchor, root)
        contracts = [f for f in matched if CONTRACT_RE.search(f)]
        hubs = [f for f in matched if Path(f).name in HUB_NAMES and f not in contracts]
        exclusive = [f for f in matched if f not in contracts and f not in hubs]
        raw_units.append(
            {
                "id": f"U{idx + 1}",
                "anchors": [anchor],
                "contracts": contracts,
                "exclusive": exclusive
                or ([anchor] if not contracts and not hubs else []),
                "hubs": hubs,
            }
        )

    units: list[dict[str, Any]] = []
    for item in raw_units:
        s_item = set(item["exclusive"])
        merged = False
        for ex in units:
            s_ex = set(ex["exclusive"])
            if (
                s_item
                and s_ex
                and (len(s_item & s_ex) / max(1, min(len(s_item), len(s_ex)))) >= 0.4
            ):
                ex["anchors"].extend(item["anchors"])
                ex["contracts"] = sorted(set(ex["contracts"]) | set(item["contracts"]))
                ex["exclusive"] = sorted(s_ex | s_item)
                ex["hubs"] = sorted(set(ex["hubs"]) | set(item["hubs"]))
                merged = True
                break
        if not merged:
            units.append(item)

    deps: dict[str, set[str]] = {u["id"]: set() for u in units}
    unit_dirs = {
        u["id"]: {str(Path(f).parent) for f in u["exclusive"] if "/" in f}
        for u in units
    }
    unit_imports = {u["id"]: extract_local_imports(u["exclusive"], root) for u in units}
    for caller in units:
        for provider in units:
            if caller["id"] != provider["id"] and any(
                d
                and d != "."
                and any(
                    imp == d or imp.endswith("/" + d) or d.endswith("/" + imp)
                    for imp in unit_imports[caller["id"]]
                )
                for d in unit_dirs[provider["id"]]
            ):
                deps[caller["id"]].add(provider["id"])

    waves: list[dict[str, Any]] = []
    all_contracts = sorted({f for u in units for f in u["contracts"]})
    if all_contracts:
        waves.append(
            {
                "wave": 0,
                "kind": "contract-freeze",
                "parallel": False,
                "tasks": [{"id": "W0-CONTRACT", "write_scope": all_contracts}],
            }
        )

    remaining = [u for u in units if u["exclusive"]]
    placed: set[str] = set()
    scope_owners: dict[str, str] = {}
    wave_num = 1 if all_contracts else 0
    while remaining:
        batch, used, nxt = [], set(), []
        for u in remaining:
            overlap_owners = {
                scope_owners[f] for f in u["exclusive"] if f in scope_owners
            }
            if overlap_owners:
                deps[u["id"]].update(overlap_owners)
            if not (deps[u["id"]] - placed) and not (set(u["exclusive"]) & used):
                batch.append(u)
                used |= set(u["exclusive"])
            else:
                nxt.append(u)
        if not batch:
            batch.append(nxt.pop(0))
        for u in batch:
            for f in u["exclusive"]:
                scope_owners[f] = u["id"]
        waves.append(
            {
                "wave": wave_num,
                "kind": "implementation",
                "parallel": len(batch) > 1,
                "tasks": [
                    {
                        "id": u["id"],
                        "anchors": u["anchors"],
                        "write_scope": u["exclusive"],
                        "depends_on": sorted(deps[u["id"]]),
                    }
                    for u in batch
                ],
            }
        )
        placed.update(u["id"] for u in batch)
        remaining = nxt
        wave_num += 1

    all_hubs = sorted({f for u in units for f in u["hubs"]})
    if all_hubs:
        waves.append(
            {
                "wave": wave_num,
                "kind": "hub-wiring",
                "parallel": False,
                "tasks": [{"id": f"W{wave_num}-WIRE", "write_scope": all_hubs}],
            }
        )

    if args.json:
        print(
            json.dumps(
                {"repo_root": str(root), "waves": waves}, ensure_ascii=False, indent=2
            )
        )
    else:
        for w in waves:
            print(
                f"Wave {w['wave']} [{w['kind']} | {'PARALLEL worktrees' if w['parallel'] else 'SERIAL'}]:"
            )
            for t in w["tasks"]:
                print(f"  - {t['id']}: scope={t['write_scope']}")
    return 0


def git_dirty_paths(root: Path) -> list[str]:
    try:
        proc = subprocess.run(
            ["git", "status", "--porcelain=v1", "-z", "-uall"],
            cwd=str(root),
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=15,
            check=False,
        )
    except Exception:
        return []
    if proc.returncode != 0:
        return []
    fields = proc.stdout.split(b"\0")
    paths: list[str] = []
    i = 0
    while i < len(fields):
        rec = fields[i]
        i += 1
        if len(rec) < 4:
            continue
        xy = rec[:2].decode("ascii", "replace")
        path = os.fsdecode(rec[3:])
        if "R" in xy or "C" in xy:
            i += 1
        if path == ".arc" or path.startswith(".arc/"):
            continue
        paths.append(path)
    return paths


def cmd_handoff(args: argparse.Namespace) -> int:
    root = find_repo_root(Path(args.repo))
    hdir = root / ".arc" / "handoff"
    hdir.mkdir(parents=True, exist_ok=True)
    _, branch = run_quiet(["git", "rev-parse", "--abbrev-ref", "HEAD"], root)
    _, head = run_quiet(["git", "rev-parse", "HEAD"], root)
    dirty = git_dirty_paths(root)
    db_info = read_arc_db(root, task_id=args.task)
    task_row = db_info.get("requested_task") or next(
        (t for t in db_info["tasks"] if t.get("id") == args.task), None
    )
    scope = [s.strip() for s in (args.scope or "").split(",") if s.strip()] or dirty
    packet = {
        "task_id": args.task,
        "agent": args.agent,
        "timestamp": utc_now(),
        "branch": branch or "UNBORN",
        "head": head or "NONE",
        "arc_task": task_row,
        "write_scope": scope,
        "dirty_files": dirty,
        "latest_gate_report": latest_gate_report(root),
        "note": args.note or "",
    }
    out_path = hdir / f"{re.sub(r'[^A-Za-z0-9._-]+', '_', args.task)}.json"
    out_path.write_text(
        json.dumps(packet, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )
    if (
        shutil.which("arc")
        and is_arc_active(root)
        and (root / ".arc" / "arc.db").is_file()
    ):
        run_quiet(["arc", "update", "--changed-only", "--repo", str(root)], root)
    print(
        json.dumps(
            {"saved": str(out_path.relative_to(root)), "packet": packet},
            ensure_ascii=False,
            indent=2,
        )
    )
    return 0


def main() -> int:
    ap = argparse.ArgumentParser(prog="arc-wave")
    sub = ap.add_subparsers(dest="sub", required=True)
    p_st = sub.add_parser("status")
    p_st.add_argument("--repo", default=".")
    p_st.add_argument("--json", action="store_true")
    p_pl = sub.add_parser("plan")
    p_pl.add_argument("--repo", default=".")
    p_pl.add_argument("--json", action="store_true")
    p_pl.add_argument("anchors", nargs="+")
    p_ho = sub.add_parser("handoff")
    p_ho.add_argument("--repo", default=".")
    p_ho.add_argument("--task", required=True)
    p_ho.add_argument("--agent", required=True)
    p_ho.add_argument("--note", default="")
    p_ho.add_argument("--scope", default="")
    ns = ap.parse_args()
    return {"status": cmd_status, "plan": cmd_plan, "handoff": cmd_handoff}[ns.sub](ns)


if __name__ == "__main__":
    raise SystemExit(main())
