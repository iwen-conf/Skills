from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
from pathlib import Path

from arc_policy import __version__
from arc_policy.capabilities import check_pinned_skill, resolve_skill, skill_catalog
from arc_policy.detect import detect_languages, recommended_skill, resolve_skill_name
from arc_policy.githook import (
    HOOK_EVENTS,
    config_hooks_supported,
    install_git_hooks,
    refusal,
)
from arc_policy.gitutil import tree_fingerprint
from arc_policy.hook import run_hook
from arc_policy.install import install_runtime, persistent_cli
from arc_policy.paths import plugin_root, repo_root
from arc_policy.policy import load_for_repo
from arc_policy.store import (
    active_task,
    begin_task,
    current_receipt,
    end_task,
    receipt_matches_tree,
    switch_task,
)
from arc_policy.verify import authorize_task, verify_task


def _print_json(data: object) -> int:
    json.dump(data, sys.stdout, indent=2, ensure_ascii=False)
    sys.stdout.write("\n")
    return 0


def cmd_version(_: argparse.Namespace) -> int:
    print(f"arc-policy {__version__} — Coding Agent Policy Runtime")
    return 0


def cmd_detect(args: argparse.Namespace) -> int:
    root = repo_root(Path(args.cwd) if args.cwd else None)
    policy = load_for_repo(root)
    languages = detect_languages(root, policy)
    return _print_json(
        {
            "repo_root": str(root),
            "languages": languages,
            "recommended_skill": recommended_skill(languages, policy),
            "selection_basis": "stack_hint_only; choose the phase by the requested result",
            "plugin_root": str(plugin_root()),
        }
    )


def cmd_skills(args: argparse.Namespace) -> int:
    if args.name:
        entry = resolve_skill(args.name)
        if entry["name"] == "arch-diagram":
            check_pinned_skill(Path(entry["directory"]))
            entry["source_check"] = "PASS"
        return _print_json(entry)
    return _print_json({"plugin_root": str(plugin_root()), "skills": skill_catalog()})


def cmd_diagram(args: argparse.Namespace) -> int:
    from arc_policy.diagram import build_diagram

    root = repo_root(Path(args.cwd) if args.cwd else None)

    def path(value: str) -> Path:
        result = Path(value).expanduser()
        return result if result.is_absolute() else root / result

    result = build_diagram(
        snapshot=path(args.snapshot),
        out=path(args.out),
        presentation=path(args.presentation) if args.presentation else None,
        view=args.view,
        root=root,
        check=not args.no_check,
    )
    _print_json(result)
    built_draft = args.no_check and result["readiness"]["build"] == "READY"
    return 0 if result["status"] == "READY" or built_draft else 1


def cmd_policy_show(args: argparse.Namespace) -> int:
    root = repo_root(Path(args.cwd) if args.cwd else None)
    languages = detect_languages(root)
    policy = load_for_repo(root, languages=languages)
    return _print_json(policy)


def cmd_task_begin(args: argparse.Namespace) -> int:
    root = repo_root(Path(args.cwd) if args.cwd else None)
    policy = load_for_repo(root, languages=detect_languages(root))
    skill = resolve_skill_name(args.skill, policy)
    slug = args.slug or skill
    # Remove retired per-commit receipt hooks, preserving unrelated hooks.
    install_git_hooks(root)
    task = begin_task(root, skill=skill, slug=slug, force=args.force)
    return _print_json(task)


def cmd_task_switch(args: argparse.Namespace) -> int:
    root = repo_root(Path(args.cwd) if args.cwd else None)
    policy = load_for_repo(root, languages=detect_languages(root))
    skill = resolve_skill_name(args.skill, policy)
    # Cleanup is idempotent; no Git verify gate is installed on phase switch.
    install_git_hooks(root)
    task = switch_task(
        root,
        skill=skill,
        slug=args.slug or skill,
        incomplete=args.incomplete,
        reason=args.reason,
    )
    return _print_json(task)


def cmd_install_git_hooks(args: argparse.Namespace) -> int:
    root = repo_root(Path(args.cwd) if args.cwd else None)
    install_git_hooks(root)
    print(str(root))
    return 0


def cmd_git_hook(args: argparse.Namespace) -> int:
    # During `git commit -a` Git points hooks at a temporary index; receipts bind the real one.
    os.environ.pop("GIT_INDEX_FILE", None)
    root = repo_root(Path(args.cwd) if args.cwd else None)
    # pre-push receives `<local ref> <local sha> <remote ref> <remote sha>` lines on stdin.
    updates = (
        sys.stdin.read()
        if args.event == "pre-push" and sys.stdin and not sys.stdin.isatty()
        else ""
    )
    reason = refusal(root, args.event, os.environ, updates)
    if reason:
        print(reason, file=sys.stderr)
        return 1
    return 0


def cmd_task_status(args: argparse.Namespace) -> int:
    root = repo_root(Path(args.cwd) if args.cwd else None)
    task = active_task(root)
    receipt = current_receipt(root)
    return _print_json(
        {
            "repo_root": str(root),
            "task": task,
            "receipt_ok": receipt_matches_tree(root, receipt),
            "tree": tree_fingerprint(root),
        }
    )


def cmd_task_verify(args: argparse.Namespace) -> int:
    root = repo_root(Path(args.cwd) if args.cwd else None)
    try:
        receipt = verify_task(root, run_tests=not args.no_tests)
    except RuntimeError as exc:
        print(f"arc-policy: {exc}", file=sys.stderr)
        return 2
    _print_json(receipt)
    return 0 if receipt.get("verdict") == "PASS" else 1


def cmd_task_authorize(args: argparse.Namespace) -> int:
    root = repo_root(Path(args.cwd) if args.cwd else None)
    return _print_json(authorize_task(root, args.quote))


def cmd_task_end(args: argparse.Namespace) -> int:
    root = repo_root(Path(args.cwd) if args.cwd else None)
    ended = end_task(root, require_verified=not (args.force or args.incomplete))
    if ended is None:
        print("arc-policy: no active task", file=sys.stderr)
        return 2
    return _print_json(ended)


def cmd_hook(args: argparse.Namespace) -> int:
    raw = sys.stdin.read()
    cwd = Path(args.cwd) if args.cwd else None
    sys.stdout.write(run_hook(raw, cwd=cwd))
    if sys.stdout.isatty():
        sys.stdout.write("\n")
    return 0


def cmd_install(args: argparse.Namespace) -> int:
    bin_dir = (
        Path(args.bin_dir).expanduser()
        if args.bin_dir
        else Path.home() / ".local" / "bin"
    )
    install_runtime(plugin_root(), bin_dir=bin_dir)
    dest = bin_dir / "arc-policy"
    print(str(dest))
    return 0


def _arc_engines(arc_binary: str | None) -> dict[str, bool]:
    """Engines are private to arc (libexec/arc), so ask arc rather than PATH."""
    if not arc_binary:
        return {}
    try:
        result = subprocess.run(
            [arc_binary, "capabilities", "--json"],
            capture_output=True,
            text=True,
            timeout=10,
            check=False,
        )
        engines = json.loads(result.stdout).get("engines") or {}
    except (OSError, subprocess.TimeoutExpired, ValueError, AttributeError):
        return {}
    return {
        name: bool(isinstance(info, dict) and info.get("available"))
        for name, info in engines.items()
    }


def cmd_doctor(args: argparse.Namespace) -> int:
    import shutil

    root = repo_root(Path(args.cwd) if args.cwd else None)
    policy_ok = True
    error = None
    try:
        load_for_repo(root, languages=detect_languages(root))
    except Exception as exc:  # noqa: BLE001 -- doctor reports malformed policy loaders.
        policy_ok = False
        error = str(exc)
    graph_declared = (
        (root / ".arc/verify/config.jsonc").is_file()
        or bool(
            (
                load_for_repo(root, languages=detect_languages(root)).get(
                    "verification"
                )
                or {}
            ).get("business_graph")
        )
        if policy_ok
        else (root / ".arc/verify/config.jsonc").is_file()
    )
    arc_binary = shutil.which("arc")
    engines = _arc_engines(arc_binary)
    legacy_verify = shutil.which("arc-verify")
    verify_argv: list[str] | None
    if arc_binary and engines.get("arc-verify"):
        verify_argv = [arc_binary, "verify"]
    elif legacy_verify:
        verify_argv = [legacy_verify]
    else:
        verify_argv = None
    turso_binary = shutil.which("tursodb")  # optional manual inspection only
    native_db_ok = False
    native_db_error = None
    if graph_declared and verify_argv:
        try:
            selftest = subprocess.run(
                [*verify_argv, "selftest"],
                capture_output=True,
                text=True,
                timeout=30,
                check=False,
            )
            native_db_ok = selftest.returncode == 0
            if not native_db_ok:
                native_db_error = (selftest.stderr or selftest.stdout).strip()[-600:]
        except (OSError, subprocess.TimeoutExpired) as exc:
            native_db_error = str(exc)
    report = {
        "graph_configured": graph_declared,
        "arc_on_path": arc_binary is not None,
        "verify_engine_ready": bool(verify_argv),
        "tursodb_on_path": bool(turso_binary),
        "graph_runtime_ready": not graph_declared or bool(verify_argv and native_db_ok),
        "native_turso_selftest": native_db_ok if graph_declared else None,
        "native_turso_error": native_db_error,
        "version": __version__,
        "repo_root": str(root),
        "plugin_root": str(plugin_root()),
        "cli_on_path": shutil.which("arc-policy") is not None,
        "code_index_ready": bool(engines.get("arc-idx"))
        or shutil.which("arc-idx") is not None,
        "completion_judge": "jev" if shutil.which("jev") else "completion_markers",
        "policy_ok": policy_ok,
        "policy_error": error,
        "active_task": active_task(root),
        "diagram": {
            "skill_bundled": (plugin_root() / "skills/arch-diagram/SKILL.md").is_file(),
            "node_on_path": shutil.which("node") is not None,
            "layout_check": "NOT_RUN: run diagram against a selected snapshot; Chrome/Chromium/Edge required",
        },
    }
    required_skills = (
        "arc-explore",
        "arc-plan",
        "arc-code",
        "arc-verify",
        "arc-audit",
        "arc-vet",
    )
    missing = [
        name
        for name in required_skills
        if not (plugin_root() / "skills" / name / "SKILL.md").is_file()
    ]
    supports_config = config_hooks_supported()
    from arc_policy.gitutil import git

    custom_hooks = git(root, "config", "--get", "core.hooksPath").returncode == 0
    report.update(
        engine_ready=policy_ok,
        skills_ready=not missing,
        missing_skills=missing,
        persistent_cli=str(persistent_cli()),
        cli_ready=persistent_cli().is_file() and report["cli_on_path"],
        git_config_hooks_supported=supports_config,
        git_gate_installable=supports_config or not custom_hooks,
        host_hooks_ready=None,
        host_hooks_status="NOT_RUN: verify enabled/trusted hooks and an actual denied call in each CLI",
    )
    report["workflow_ready"] = all(
        report[key]
        for key in (
            "engine_ready",
            "skills_ready",
            "cli_ready",
            "code_index_ready",
            "graph_runtime_ready",
            "git_gate_installable",
        )
    )
    _print_json(report)
    return 0 if report["workflow_ready"] else 1


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="arc-policy", description="Arc Coding Agent Policy Runtime"
    )
    parser.add_argument("--cwd", help="repository root (default: detect from cwd)")
    sub = parser.add_subparsers(dest="cmd", required=True)

    sub.add_parser("version").set_defaults(func=cmd_version)
    sub.add_parser("doctor").set_defaults(func=cmd_doctor)
    sub.add_parser("detect").set_defaults(func=cmd_detect)
    skills = sub.add_parser("skills", help="resolve this installation's bundled skill entries")
    skills.add_argument("--name", help="one skill, including canonical arc:* aliases")
    skills.set_defaults(func=cmd_skills)
    diagram = sub.add_parser("diagram", help="render a read-only business snapshot with pinned arch-diagram")
    diagram.add_argument("--snapshot", required=True, help="existing v2 snapshot.json; no DB access")
    diagram.add_argument("--out", required=True, help="new artifact directory or HTML path")
    diagram.add_argument("--presentation", help="graph-bound Chinese labels and package source evidence")
    diagram.add_argument("--view", choices=("flows", "capabilities"), default="flows")
    diagram.add_argument("--no-check", action="store_true", help="draft only; explicitly record layout NOT_RUN")
    diagram.set_defaults(func=cmd_diagram)
    install = sub.add_parser(
        "install", help="install an offline persistent runtime and PATH launcher"
    )
    install.add_argument("--bin-dir", help="launcher directory (default: ~/.local/bin)")
    install.set_defaults(func=cmd_install)

    policy_p = sub.add_parser("policy")
    policy_sub = policy_p.add_subparsers(dest="policy_cmd", required=True)
    policy_sub.add_parser("show").set_defaults(func=cmd_policy_show)

    task = sub.add_parser("task")
    task_sub = task.add_subparsers(dest="task_cmd", required=True)
    begin = task_sub.add_parser("begin")
    begin.add_argument("--skill", required=True)
    begin.add_argument("--slug")
    begin.add_argument("--force", action="store_true")
    begin.set_defaults(func=cmd_task_begin)
    switch = task_sub.add_parser(
        "switch", help="archive the active phase and start the next"
    )
    switch.add_argument("--skill", required=True)
    switch.add_argument("--slug")
    switch.add_argument("--incomplete", action="store_true")
    switch.add_argument("--reason", help="why the previous phase is incomplete")
    switch.set_defaults(func=cmd_task_switch)
    task_sub.add_parser("status").set_defaults(func=cmd_task_status)
    verify = task_sub.add_parser("verify")
    verify.add_argument("--no-tests", action="store_true")
    verify.set_defaults(func=cmd_task_verify)
    authorize = task_sub.add_parser(
        "authorize",
        help="record explicit user approval to close/deliver a task without verifying",
    )
    authorize.add_argument(
        "--quote", required=True, help="the user's own words from this conversation"
    )
    authorize.set_defaults(func=cmd_task_authorize)
    end = task_sub.add_parser("end")
    end.add_argument("--force", action="store_true")
    end.add_argument("--incomplete", action="store_true")
    end.set_defaults(func=cmd_task_end)

    hook = sub.add_parser("hook", help="read one hook event JSON from stdin")
    hook.set_defaults(func=cmd_hook)

    sub.add_parser(
        "install-git-hooks",
        help="remove retired Arc-managed Git commit/push verification hooks",
    ).set_defaults(func=cmd_install_git_hooks)
    git_hook = sub.add_parser(
        "git-hook", help="legacy hook entrypoint (commit/push are no longer gated)"
    )
    git_hook.add_argument("event", choices=HOOK_EVENTS)
    git_hook.add_argument("hook_args", nargs="*")
    git_hook.set_defaults(func=cmd_git_hook)
    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    try:
        return args.func(args)
    except (OSError, RuntimeError, ValueError) as exc:
        print(f"arc-policy: {exc}", file=sys.stderr)
        return 2
