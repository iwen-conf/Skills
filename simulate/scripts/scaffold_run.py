#!/usr/bin/env python3
"""
Scaffold a ui-ux-simulation run directory with professional test deliverable templates.

This script copies template packs from ../templates/packs/* into:
  <report_output_dir>/<run_id>/
Including a canonical accounts file:
  accounts.jsonc

Always run with --help first.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import secrets
from datetime import datetime
from pathlib import Path
from typing import Any


_PLACEHOLDER_RE = re.compile(r"\{\{([A-Z0-9_]+)\}\}")


def _render_template(text: str, mapping: dict[str, str]) -> str:
    def replace(match: re.Match[str]) -> str:
        key = match.group(1)
        return mapping.get(key, match.group(0))

    return _PLACEHOLDER_RE.sub(replace, text)


def _generate_run_id(now: datetime | None = None) -> str:
    now = now or datetime.now()
    ts = now.strftime("%Y-%m-%d_%H-%M-%S")
    short = "".join(secrets.choice("abcdefghijklmnopqrstuvwxyz0123456789") for _ in range(6))
    return f"{ts}_{short}"


def _load_personas(value: str | None) -> list[dict[str, Any]]:
    if not value:
        return []

    path = Path(value)
    raw = path.read_text(encoding="utf-8") if path.exists() else value
    personas = json.loads(raw)
    if not isinstance(personas, list):
        raise ValueError("personas must be a JSON array")
    return [p if isinstance(p, dict) else {"role": str(p)} for p in personas]


def _personas_to_markdown(personas: list[dict[str, Any]]) -> str:
    if not personas:
        return "* _<provide personas: role/user/pass/token>_"
    lines: list[str] = []
    for p in personas:
        role = str(p.get("role", "role"))
        user = str(p.get("user", "<user>"))
        password = str(p.get("pass", "<pass>"))
        token = p.get("token")
        token_part = f" token=`{token}`" if token is not None and str(token) != "" else ""
        lines.append(f"* **{role}**: user=`{user}` pass=`{password}`{token_part}")
    return "\n".join(lines)


def _personas_to_accounts(personas: list[dict[str, Any]]) -> list[dict[str, Any]]:
    if not personas:
        return [
            {
                "role": "<role>",
                "username": "<user>",
                "password": "<pass>",
                "token": "<token_or_empty>",
                "created_for_verification": False,
                "why": "",
                "created_at": "",
            }
        ]

    accounts: list[dict[str, Any]] = []
    for p in personas:
        role = str(p.get("role", "role"))
        username = p.get("user") or p.get("username") or "<user>"
        password = p.get("pass") or p.get("password") or "<pass>"
        token = p.get("token") or ""
        accounts.append(
            {
                "role": role,
                "username": str(username),
                "password": str(password),
                "token": str(token),
                "created_for_verification": False,
                "why": "",
                "created_at": "",
            }
        )
    return accounts


def _available_packs(packs_root: Path) -> list[str]:
    if not packs_root.exists():
        return []
    packs = []
    for p in packs_root.iterdir():
        if p.is_dir() and not p.name.startswith("."):
            packs.append(p.name)
    return sorted(packs)


def _copy_pack(
    *,
    pack_dir: Path,
    dest_dir: Path,
    mapping: dict[str, str],
    force: bool,
    dry_run: bool,
) -> None:
    for src in sorted(pack_dir.rglob("*")):
        if src.is_dir():
            continue

        rel = src.relative_to(pack_dir)
        out_rel = rel.with_suffix("") if rel.suffix == ".tpl" else rel
        out_path = dest_dir / out_rel

        if out_path.exists() and not force:
            raise FileExistsError(f"Refusing to overwrite existing file: {out_path}")

        if dry_run:
            print(f"[DRY] {src} -> {out_path}")
            continue

        out_path.parent.mkdir(parents=True, exist_ok=True)

        if src.suffix in {".tpl", ".md", ".json", ".jsonl", ".txt"}:
            content = src.read_text(encoding="utf-8")
            out_path.write_text(_render_template(content, mapping), encoding="utf-8")
        else:
            out_path.write_bytes(src.read_bytes())


def main() -> None:
    here = Path(__file__).resolve()
    skill_root = here.parent.parent
    packs_root = skill_root / "templates" / "packs"

    parser = argparse.ArgumentParser(
        description="Scaffold ui-ux-simulation run directory & test-company-grade deliverable templates",
    )
    parser.add_argument("--list-packs", action="store_true", help="List available template packs and exit")
    parser.add_argument(
        "--pack",
        action="append",
        default=[],
        help="Template pack(s) to apply (repeatable). Always includes 'e2e' implicitly unless --no-e2e.",
    )
    parser.add_argument("--no-e2e", action="store_true", help="Do not auto-include the required 'e2e' pack")
    parser.add_argument("--run-id", help="Run ID (default: auto-generate)")
    parser.add_argument("--objective", help="test_objective to embed into templates")
    parser.add_argument("--target-url", help="target_url to embed into templates")
    parser.add_argument("--personas", help="Personas JSON string or path to JSON file")
    parser.add_argument(
        "--report-output-dir",
        default="reports",
        help="Root output directory (default: reports/)",
    )
    parser.add_argument(
        "--formats",
        default="markdown",
        help='Comma-separated formats: markdown,jsonl (default: "markdown")',
    )
    parser.add_argument("--force", action="store_true", help="Overwrite existing files")
    parser.add_argument("--dry-run", action="store_true", help="Print actions without writing files")

    args = parser.parse_args()

    if args.list_packs:
        packs = _available_packs(packs_root)
        if not packs:
            print(f"No packs found under: {packs_root}")
            raise SystemExit(1)
        for p in packs:
            print(p)
        return

    run_id = args.run_id or _generate_run_id()
    report_output_dir = Path(args.report_output_dir)
    run_dir = report_output_dir / run_id

    objective = args.objective or "<test_objective>"
    target_url = args.target_url or "<target_url>"
    personas = _load_personas(args.personas)

    selected_packs: list[str] = [p.strip() for p in args.pack if p.strip()]
    if not args.no_e2e and "e2e" not in selected_packs:
        selected_packs.insert(0, "e2e")

    formats = {f.strip().lower() for f in str(args.formats).split(",") if f.strip()}

    available = set(_available_packs(packs_root))
    unknown = [p for p in selected_packs if p not in available]
    if unknown:
        raise SystemExit(f"Unknown pack(s): {', '.join(unknown)}. Use --list-packs.")

    mapping = {
        "RUN_ID": run_id,
        "OBJECTIVE": objective,
        "TARGET_URL": target_url,
        "PERSONAS_MARKDOWN": _personas_to_markdown(personas),
        "ACCOUNTS_JSON": json.dumps(_personas_to_accounts(personas), indent=2, ensure_ascii=False),
        "GENERATED_AT": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
        "PACKS": ", ".join(selected_packs) if selected_packs else "e2e",
        "START_TIME": "<start_time>",
        "END_TIME": "<end_time>",
        "RESULT": "<PASS|FAIL>",
        "ENV_NAME": "<env_name>",
        "BUILD_REF": "<build/commit/tag>",
        "VALIDATION_CONTAINER": "<docker_container_or_empty>",
        "SCENARIO_1": "<scenario_step_1>",
        "SCENARIO_2": "<scenario_step_2>",
        "SCENARIO_3": "<scenario_step_3>",
    }

    if run_dir.exists() and any(run_dir.iterdir()) and not args.force:
        raise SystemExit(f"Refusing to write into non-empty directory: {run_dir} (use --force)")

    if not args.dry_run:
        run_dir.mkdir(parents=True, exist_ok=True)

    # Required directories (per SKILL.md Phase 4)
    for d in ["screenshots", "failures", "db"]:
        p = run_dir / d
        if args.dry_run:
            print(f"[DRY] mkdir -p {p}")
        else:
            p.mkdir(parents=True, exist_ok=True)

    # Copy selected packs
    for pack in selected_packs:
        _copy_pack(
            pack_dir=packs_root / pack,
            dest_dir=run_dir,
            mapping=mapping,
            force=args.force,
            dry_run=args.dry_run,
        )

    # Optional machine-readable events log
    if "jsonl" in formats:
        events_path = run_dir / "events.jsonl"
        if args.dry_run:
            print(f"[DRY] touch {events_path}")
        else:
            events_path.touch(exist_ok=True)

    print(str(run_dir))


if __name__ == "__main__":
    main()
