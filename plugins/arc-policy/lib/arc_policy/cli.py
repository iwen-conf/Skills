from __future__ import annotations

import argparse
import json
import os
import sys
from pathlib import Path

from arc_policy import __version__
from arc_policy.detect import detect_languages, recommended_skill, resolve_skill_name
from arc_policy.githook import HOOK_EVENTS, install_git_hooks, refusal
from arc_policy.gitutil import tree_fingerprint
from arc_policy.hook import run_hook
from arc_policy.paths import plugin_root, repo_root
from arc_policy.policy import load_for_repo
from arc_policy.store import (
    active_task,
    begin_task,
    current_receipt,
    end_task,
    receipt_matches_tree,
)
from arc_policy.verify import verify_task


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
            "plugin_root": str(plugin_root()),
        }
    )


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
    task = begin_task(root, skill=skill, slug=slug, force=args.force)
    install_git_hooks(root)
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
    reason = refusal(root, args.event, os.environ)
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
    dest = Path.home() / ".local" / "bin" / "arc-policy"
    src = plugin_root() / "bin" / "arc-policy"
    if not src.is_file():
        print(f"arc-policy: missing CLI at {src}", file=sys.stderr)
        return 2
    dest.parent.mkdir(parents=True, exist_ok=True)
    if dest.exists() or dest.is_symlink():
        dest.unlink()
    dest.symlink_to(src)
    dest.chmod(0o755)
    print(str(dest))
    return 0


def cmd_doctor(args: argparse.Namespace) -> int:
    import shutil

    root = repo_root(Path(args.cwd) if args.cwd else None)
    policy_ok = True
    error = None
    try:
        load_for_repo(root, languages=detect_languages(root))
    except Exception as exc:
        policy_ok = False
        error = str(exc)
    report = {
        "version": __version__,
        "repo_root": str(root),
        "plugin_root": str(plugin_root()),
        "cli_on_path": shutil.which("arc-policy") is not None,
        "arc_idx_on_path": shutil.which("arc-idx") is not None,
        "completion_judge": "jev" if shutil.which("jev") else "completion_markers",
        "policy_ok": policy_ok,
        "policy_error": error,
        "active_task": active_task(root),
    }
    _print_json(report)
    return 0 if policy_ok else 1


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="arc-policy", description="Arc Coding Agent Policy Runtime"
    )
    parser.add_argument("--cwd", help="repository root (default: detect from cwd)")
    sub = parser.add_subparsers(dest="cmd", required=True)

    sub.add_parser("version").set_defaults(func=cmd_version)
    sub.add_parser("doctor").set_defaults(func=cmd_doctor)
    sub.add_parser("detect").set_defaults(func=cmd_detect)
    sub.add_parser(
        "install", help="symlink CLI to ~/.local/bin/arc-policy"
    ).set_defaults(func=cmd_install)

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
    task_sub.add_parser("status").set_defaults(func=cmd_task_status)
    verify = task_sub.add_parser("verify")
    verify.add_argument("--no-tests", action="store_true")
    verify.set_defaults(func=cmd_task_verify)
    end = task_sub.add_parser("end")
    end.add_argument("--force", action="store_true")
    end.add_argument("--incomplete", action="store_true")
    end.set_defaults(func=cmd_task_end)

    hook = sub.add_parser("hook", help="read one hook event JSON from stdin")
    hook.set_defaults(func=cmd_hook)

    sub.add_parser(
        "install-git-hooks",
        help="register the agent commit/push gate in this repository",
    ).set_defaults(func=cmd_install_git_hooks)
    git_hook = sub.add_parser(
        "git-hook", help="run by Git before an agent commit or push"
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
