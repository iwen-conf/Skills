from __future__ import annotations

import json
import os
import re
import shutil
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from arc_policy.detect import detect_languages, recommended_skill
from arc_policy.gitutil import dirty_paths, index_tree, tree_fingerprint
from arc_policy.policy import load_for_repo
from arc_policy.store import active_task, invalidate_receipt, save_receipt, task_lock

VERDICTS = frozenset({"PASS", "FAIL", "BLOCKED", "NOT_RUN"})


def _now() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat()


def _run(
    root: Path, argv: list[str], timeout: int = 600
) -> subprocess.CompletedProcess[str]:
    env = dict(os.environ)
    env["GIT_OPTIONAL_LOCKS"] = "0"
    return subprocess.run(
        argv,
        cwd=root,
        env=env,
        capture_output=True,
        text=True,
        timeout=timeout,
        check=False,
    )


def check_architecture(
    root: Path, policy: dict[str, Any], task: dict[str, Any]
) -> dict[str, Any]:
    languages = detect_languages(root, policy)
    expected = recommended_skill(languages, policy)
    actual = task.get("skill")
    if (
        languages
        and actual
        and actual != expected
        and actual
        not in {"arc-explore", "arc-plan", "arc-audit", "arc-verify", "arc-vet"}
    ):
        return {
            "id": "architecture",
            "verdict": "FAIL",
            "reason": f"task skill {actual!r} does not match detected stack skill {expected!r} ({', '.join(languages)})",
        }
    hint = ""
    specs = policy.get("languages") or {}
    for language in languages:
        value = (specs.get(language) or {}).get("verify_hint")
        if isinstance(value, str) and value:
            hint = value
            break
    return {
        "id": "architecture",
        "verdict": "PASS",
        "reason": hint or f"stack={languages or ['unknown']} skill={actual}",
    }


def _nearest_marker(path: Path, root: Path, marker: str) -> Path | None:
    start = path if path.is_dir() else path.parent
    for candidate in (start, *start.parents):
        try:
            candidate.relative_to(root)
        except ValueError:
            return None
        if (candidate / marker).is_file():
            return candidate
        if candidate == root:
            return None
    return None


def _venv_python(directory: Path) -> Path | None:
    python = directory / ".venv/bin/python"
    if python.is_file() and (python.parent / "pytest").is_file():
        return python
    return None


def _python_test_argv(project: Path, root: Path) -> list[str]:
    for candidate in (project, *project.parents):
        python = _venv_python(candidate)
        if python is not None:
            return [str(python), "-m", "pytest", "-q"]
        if candidate == root:
            break
    if shutil.which("pytest"):
        return ["pytest", "-q"]
    # uv cannot create a venv when a path segment contains ':'.
    if shutil.which("uv") and all(":" not in part for part in project.parts):
        return ["uv", "run", "--no-sync", "python", "-m", "pytest", "-q"]
    return [sys.executable, "-m", "pytest", "-q"]


def detect_test_commands(root: Path) -> list[tuple[list[str], Path]]:
    markers = ("pyproject.toml", "setup.py", "go.mod", "Cargo.toml", "package.json")
    projects: set[tuple[Path, str]] = {
        (root, marker) for marker in markers if (root / marker).is_file()
    }
    for rel in dirty_paths(root):
        path = root / rel
        for marker in markers:
            project = _nearest_marker(path, root, marker)
            if project is not None:
                projects.add((project, marker))
    commands: list[tuple[list[str], Path]] = []
    for project, marker in sorted(projects):
        if marker in {"pyproject.toml", "setup.py"}:
            argv = _python_test_argv(project, root)
        elif marker == "go.mod":
            argv = ["go", "test", "./..."]
        elif marker == "Cargo.toml":
            argv = ["cargo", "test", "--locked"]
        else:
            manager = "pnpm" if (project / "pnpm-lock.yaml").is_file() else "npm"
            argv = [manager, "test"]
        if (argv, project) not in commands:
            commands.append((argv, project))
    return commands


def _check_command(
    root: Path, argv: list[str], cwd: Path, *, configured: bool
) -> dict[str, Any]:
    label = " ".join(argv)
    if cwd != root:
        label = f"{label} (cwd {cwd.relative_to(root)})"
    if not configured and argv in (["npm", "test"], ["pnpm", "test"]):
        package = json.loads((cwd / "package.json").read_text(encoding="utf-8"))
        scripts = package.get("scripts") if isinstance(package, dict) else None
        if (
            not isinstance(scripts, dict)
            or not isinstance(scripts.get("test"), str)
            or not scripts["test"].strip()
        ):
            return {
                "verdict": "NOT_RUN",
                "reason": f"{label}: package.json has no scripts.test; configure the complete verification.test_commands plan",
            }
    if not configured and sys.platform == "darwin" and argv[:2] == ["cargo", "test"]:
        return {
            "verdict": "BLOCKED",
            "reason": f"{label}: Rust does not compile on this Mac (not even with cargo zigbuild); "
            "configure verification.test_commands that run the tests on the remote build host",
        }
    try:
        result = _run(cwd, argv)
    except subprocess.TimeoutExpired:
        return {"verdict": "FAIL", "reason": f"timeout running {label}"}
    except OSError as exc:
        return {"verdict": "NOT_RUN", "reason": f"cannot run {label}: {exc.strerror}"}
    if result.returncode != 0:
        tail = (result.stdout + "\n" + result.stderr).strip().splitlines()[-20:]
        return {
            "verdict": "FAIL",
            "reason": f"{label} exit {result.returncode}",
            "output": "\n".join(tail),
        }
    return {"verdict": "PASS", "reason": f"{label} exit 0"}


def check_tests(root: Path, policy: dict[str, Any]) -> dict[str, Any]:
    configured = (policy.get("verification") or {}).get("test_commands")
    if configured is None:
        commands = detect_test_commands(root)
    else:
        if not isinstance(configured, list) or not configured:
            raise ValueError(
                "verification.test_commands must be a nonempty list of {argv, cwd}"
            )
        commands = []
        for entry in configured:
            if not isinstance(entry, dict):
                raise ValueError("each test command must be {argv, cwd}")
            argv = entry.get("argv")
            rel = entry.get("cwd", ".")
            if (
                not isinstance(argv, list)
                or not argv
                or not all(isinstance(arg, str) and arg for arg in argv)
                or not isinstance(rel, str)
            ):
                raise ValueError(
                    "each test command needs nonempty string argv and a repository-relative cwd"
                )
            cwd = (root / rel).resolve()
            if not cwd.is_relative_to(root) or not cwd.is_dir():
                raise ValueError(
                    "test command cwd must be a directory inside the repository"
                )
            commands.append((argv, cwd))
    if not commands:
        return {
            "id": "tests",
            "verdict": "NOT_RUN",
            "reason": "no test commands detected; configure verification.test_commands",
        }
    results = [
        _check_command(root, argv, cwd, configured=configured is not None)
        for argv, cwd in commands
    ]
    verdict = next(
        (
            value
            for value in ("FAIL", "BLOCKED", "NOT_RUN")
            if any(item["verdict"] == value for item in results)
        ),
        "PASS",
    )
    return {
        "id": "tests",
        "verdict": verdict,
        "reason": f"{len(results)} test command(s)",
        "commands": results,
    }


def _graph_object(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    result = {}
    seen = set()
    for key, value in pairs:
        folded = key.casefold()
        if folded in seen:
            raise ValueError(f"duplicate graph proof field {key}")
        seen.add(folded)
        result[key] = value
    return result


# v1 graphs prove their planned rule tasks; v2 models prove every declared
# business obligation. Any other scope is not an acceptance proof.
PROOF_SCOPES = frozenset({"planned_rule_tasks", "complete_business_model_obligations"})


def check_business_graph(root: Path, policy: dict[str, Any]) -> dict[str, Any] | None:
    """Project-local business-graph gate; never contacts Turso Cloud.

    Automatic when .arc/verify/config.jsonc exists, or when the project
    explicitly configures verification.business_graph. No model -> BLOCKED.
    """
    config = (policy.get("verification") or {}).get("business_graph")
    declared = (root / ".arc/verify/config.jsonc").is_file()
    if config is None and not declared:
        return None
    if (
        not isinstance(config, dict)
        or not isinstance(config.get("graph"), str)
        or not config["graph"]
    ):
        return {
            "id": "business_graph",
            "verdict": "BLOCKED",
            "reason": "local verification configured but verification.business_graph.graph is missing",
        }
    db = config.get("db", ".arc/verify/data.db")
    if not isinstance(db, str) or not db or Path(db).is_absolute():
        return {
            "id": "business_graph",
            "verdict": "BLOCKED",
            "reason": "business_graph.db must be a repository-relative path",
        }
    target = (root / db).resolve()
    if not target.is_relative_to(root):
        return {
            "id": "business_graph",
            "verdict": "BLOCKED",
            "reason": "business_graph.db resolves outside the repository",
        }
    risk_review = config.get("risk_review", False)
    if not isinstance(risk_review, bool):
        return {
            "id": "business_graph",
            "verdict": "BLOCKED",
            "reason": "business_graph.risk_review must be true or false",
        }
    argv = ["arc-verify", "gate", "--db", db, "--graph", config["graph"]]
    if risk_review:
        argv.append("--risk-review")
    try:
        result = _run(root, argv)
    except (OSError, subprocess.TimeoutExpired) as exc:
        return {
            "id": "business_graph",
            "verdict": "BLOCKED",
            "reason": f"graph runner failed: {exc}",
        }
    try:
        proof = json.loads(result.stdout, object_pairs_hook=_graph_object)
    except (TypeError, ValueError):
        return {
            "id": "business_graph",
            "verdict": "BLOCKED",
            "reason": "graph runner emitted no valid structured proof; upgrade/rerun arc-verify",
        }
    if (
        not isinstance(proof, dict)
        or not isinstance(proof.get("verdict"), str)
        or proof["verdict"] not in VERDICTS
    ):
        return {
            "id": "business_graph",
            "verdict": "BLOCKED",
            "reason": "graph runner emitted an invalid verdict",
        }
    if result.returncode != 0 or proof["verdict"] != "PASS":
        return {
            "id": "business_graph",
            "verdict": "FAIL" if proof["verdict"] == "FAIL" else "BLOCKED",
            "reason": f"graph gate {proof['verdict']}, exit {result.returncode}",
            "output": result.stdout[-8000:],
        }
    context = proof.get("context")
    fields = {
        "graph",
        "environment",
        "target",
        "source_commit",
        "source_sha256",
        "config_sha256",
        "graph_sha256",
        "source_root",
        "config_path",
    }
    if (
        proof.get("proof_scope") not in PROOF_SCOPES
        or proof.get("inputs_unchanged") is not True
        or not isinstance(context, dict)
        or set(context) != fields
        or not all(
            isinstance(value, str) and value.strip() for value in context.values()
        )
    ):
        return {
            "id": "business_graph",
            "verdict": "BLOCKED",
            "reason": "graph proof lacks a current execution context and scope",
        }
    if (
        context["graph"] != config["graph"]
        or Path(context["source_root"]).resolve() != root.resolve()
        or Path(context["config_path"]).resolve()
        != (root / ".arc/verify/config.jsonc").resolve()
        or not re.fullmatch(r"(?:[0-9a-f]{40}|[0-9a-f]{64})", context["source_commit"])
        or any(
            not re.fullmatch(r"[0-9a-f]{64}", context[key])
            for key in ("source_sha256", "config_sha256", "graph_sha256")
        )
    ):
        return {
            "id": "business_graph",
            "verdict": "BLOCKED",
            "reason": "graph proof belongs to another candidate repository/config/graph",
        }
    return {
        "id": "business_graph",
        "verdict": "PASS",
        "reason": "current planned rule tasks verified; broader E2E/release claims need their own evidence",
        "proof_scope": proof["proof_scope"],
        "context": context,
    }


def summarize(checks: list[dict[str, Any]], policy: dict[str, Any]) -> str:
    required = list((policy.get("verification") or {}).get("required") or [])
    fail_closed = bool((policy.get("verification") or {}).get("fail_closed", True))
    by_id = {item["id"]: item for item in checks}
    if len(by_id) != len(checks) or any(
        item.get("verdict") not in VERDICTS for item in checks
    ):
        return "FAIL"
    for name in required:
        item = by_id.get(name)
        if item is None:
            return "FAIL"
        verdict = item["verdict"]
        if verdict == "FAIL":
            return "FAIL"
        if fail_closed and verdict in {"NOT_RUN", "BLOCKED"}:
            return "FAIL"
    if any(item["verdict"] == "FAIL" for item in checks):
        return "FAIL"
    return "PASS"


def verify_task(
    root: Path, policy: dict[str, Any] | None = None, *, run_tests: bool = True
) -> dict[str, Any]:
    with task_lock(root):
        return _verify_task(root, policy, run_tests=run_tests)


def _verify_task(
    root: Path, policy: dict[str, Any] | None, *, run_tests: bool
) -> dict[str, Any]:
    task = invalidate_receipt(root)
    fingerprint = tree_fingerprint(root)
    tree = index_tree(root)
    if policy is None:
        policy = load_for_repo(root, languages=detect_languages(root))
    checks = [check_architecture(root, policy, task)]
    # A graph FAIL must never prevent arc:code from fixing the defect.
    # Only arc:verify (and explicitly opted-in other skills) must close
    # the business graph before receiving a delivery receipt.
    graph_skills = {"arc-verify"}
    graph_config = (policy.get("verification") or {}).get("business_graph")
    if isinstance(graph_config, dict):
        additional_skills = graph_config.get("require_for_skills", [])
        if not isinstance(additional_skills, list) or any(
            not isinstance(name, str) or not name.strip() for name in additional_skills
        ):
            raise ValueError(
                "verification.business_graph.require_for_skills must be a list of skill names"
            )
        graph_skills.update(additional_skills)
    business_check = (
        check_business_graph(root, policy)
        if task.get("skill") in graph_skills
        else None
    )
    if business_check is not None:
        checks.append(business_check)
    if run_tests:
        checks.append(check_tests(root, policy))
    else:
        checks.append(
            {"id": "tests", "verdict": "NOT_RUN", "reason": "skipped (--no-tests)"}
        )
    unchanged = fingerprint == tree_fingerprint(root)
    current = active_task(root)
    same_task = current is not None and current.get("id") == task["id"]
    checks.append(
        {
            "id": "snapshot",
            "verdict": "PASS" if unchanged and same_task else "FAIL",
            "reason": "tree and task unchanged during verification"
            if unchanged and same_task
            else "tree or active task changed during verification; run verify again",
        }
    )
    required = list((policy.get("verification") or {}).get("required") or [])
    if business_check is not None and "business_graph" not in required:
        required.append("business_graph")
    pre_policy = {
        **policy,
        "verification": {
            **(policy.get("verification") or {}),
            "required": [name for name in required if name != "cli_receipt"],
        },
    }
    overall = summarize(checks, pre_policy)
    receipt = {
        "slug": task["slug"],
        "skill": task["skill"],
        "task_id": task["id"],
        "created_at": _now(),
        "head": fingerprint["head"],
        "dirty": fingerprint["dirty"],
        "digest": fingerprint["digest"],
        "tree": tree,
        "verdict": overall,
        "checks": checks,
    }
    if overall == "PASS":
        checks.append(
            {
                "id": "cli_receipt",
                "verdict": "PASS",
                "reason": "receipt bound to tree fingerprint",
            }
        )
        receipt["checks"] = checks
        receipt["verdict"] = "PASS"
        save_receipt(root, receipt)
    else:
        checks.append(
            {
                "id": "cli_receipt",
                "verdict": "FAIL",
                "reason": "receipt not written because required checks did not PASS",
            }
        )
        receipt["checks"] = checks
        receipt["verdict"] = "FAIL"
    return receipt


def authorize_task(root: Path, quote: str) -> dict[str, Any]:
    """Bind the user's explicit go-ahead to commit and push this tree without verification.

    The receipt records the user's words; it is not evidence that any check passed.
    """
    quote = " ".join(quote.split())
    if not quote:
        raise ValueError("--quote must hold the user's own words confirming the commit")
    with task_lock(root):
        task = active_task(root)
        if not task:
            raise RuntimeError(
                "no active task; run `arc-policy task begin --skill …` first"
            )
        fingerprint = tree_fingerprint(root)
        receipt = {
            "slug": task["slug"],
            "skill": task["skill"],
            "task_id": task["id"],
            "created_at": _now(),
            "head": fingerprint["head"],
            "dirty": fingerprint["dirty"],
            "digest": fingerprint["digest"],
            "tree": index_tree(root),
            "verdict": "AUTHORIZED",
            "authorization": {"by": "user", "quote": quote},
            "checks": [
                {
                    "id": "user_authorization",
                    "verdict": "NOT_RUN",
                    "reason": "the user confirmed delivery without `task verify`; no check ran",
                }
            ],
        }
        save_receipt(root, receipt)
        return receipt
