from __future__ import annotations

import json
import subprocess
import sys
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path
from threading import Event

import pytest

from arc_policy.classify import classify
from arc_policy.cli import main
from arc_policy.gitutil import dirty_paths
from arc_policy.hook import run_hook
from arc_policy.store import (
    active_task,
    begin_task,
    current_receipt,
    delivered_receipt,
    load_state,
    receipt_matches_tree,
    save_state,
)
from arc_policy.verify import verify_task
from test_arc_policy import ENV, PLUGIN, git, make_repo, policy_for, write_overlay


def receipt_policy(repo: Path) -> dict:
    write_overlay(repo, {"verification": {"required": ["cli_receipt"]}})
    return policy_for(repo)


@pytest.mark.parametrize("name", ["README.txt", "未跟踪 文件\n.py"])
def test_receipt_invalidated_by_same_path_edit(tmp_path: Path, name: str) -> None:
    repo = make_repo(tmp_path)
    policy = receipt_policy(repo)
    (repo / name).write_text("first\n")
    begin_task(repo, skill="arc-code", slug="demo")
    assert verify_task(repo, policy, run_tests=False)["verdict"] == "PASS"
    assert receipt_matches_tree(repo, current_receipt(repo))
    (repo / name).write_text("second\n")
    assert not receipt_matches_tree(repo, current_receipt(repo))


def test_receipt_invalidated_by_staging(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    policy = receipt_policy(repo)
    (repo / "README.txt").write_text("changed\n")
    begin_task(repo, skill="arc-code", slug="demo")
    verify_task(repo, policy, run_tests=False)
    git(repo, "add", "README.txt")
    assert not receipt_matches_tree(repo, current_receipt(repo))


def test_dirty_paths_preserve_unicode_newlines_and_untracked_files(
    tmp_path: Path,
) -> None:
    repo = make_repo(tmp_path)
    name = "新目录/带 空格\n文件.py"
    (repo / name).parent.mkdir()
    (repo / name).write_text("pass\n")
    begin_task(repo, skill="arc-code", slug="demo")
    assert dirty_paths(repo) == [name]


def go_task(repo: Path) -> dict:
    (repo / "go.mod").write_text("module example\n")
    begin_task(repo, skill="arc-code", slug="demo")
    return policy_for(repo)


def test_failed_reverification_invalidates_old_pass(
    tmp_path: Path, monkeypatch
) -> None:
    repo = make_repo(tmp_path)
    policy = go_task(repo)
    monkeypatch.setattr("arc_policy.verify.shutil.which", lambda name: f"/bin/{name}")
    monkeypatch.setattr(
        "arc_policy.verify._run",
        lambda *args: subprocess.CompletedProcess(args, 0, "", ""),
    )
    assert verify_task(repo, policy)["verdict"] == "PASS"
    monkeypatch.setattr(
        "arc_policy.verify._run",
        lambda *args: subprocess.CompletedProcess(args, 1, "assertion failed", ""),
    )
    assert verify_task(repo, policy)["verdict"] == "FAIL"
    assert not receipt_matches_tree(repo, current_receipt(repo))


@pytest.mark.parametrize("change", ["tree", "task"])
def test_verify_rejects_changes_during_tests(
    tmp_path: Path, monkeypatch, change: str
) -> None:
    repo = make_repo(tmp_path)
    policy = go_task(repo)
    monkeypatch.setattr("arc_policy.verify.shutil.which", lambda name: f"/bin/{name}")

    def run(*args):
        if change == "tree":
            (repo / "README.txt").write_text("untested change\n")
        else:
            state = load_state(repo)
            state["active"]["id"] = "out-of-band-replacement"
            save_state(repo, state)
        return subprocess.CompletedProcess(args, 0, "", "")

    monkeypatch.setattr("arc_policy.verify._run", run)
    assert verify_task(repo, policy)["verdict"] == "FAIL"
    assert not receipt_matches_tree(repo, current_receipt(repo))


@pytest.mark.parametrize(
    "command",
    [
        ["task", "verify"],
        ["task", "begin", "--skill", "arc-code", "--slug", "demo", "--force"],
        ["task", "end", "--incomplete"],
    ],
)
def test_task_mutations_cannot_overlap_verification(
    tmp_path: Path, monkeypatch, command: list[str]
) -> None:
    repo = make_repo(tmp_path)
    policy = go_task(repo)
    task = active_task(repo)
    assert task is not None
    task_id = task["id"]
    started, release = Event(), Event()

    def run(cwd, argv):
        started.set()
        assert release.wait(15), "concurrent task command did not finish"
        return subprocess.CompletedProcess(argv, 1, "assertion failed", "")

    monkeypatch.setattr("arc_policy.verify._run", run)
    with ThreadPoolExecutor(max_workers=1) as executor:
        pending = executor.submit(verify_task, repo, policy)
        try:
            assert started.wait(5), "verification did not start"
            competing = subprocess.run(
                [sys.executable, "-m", "arc_policy", "--cwd", str(repo), *command],
                env={**ENV, "PYTHONPATH": str(PLUGIN / "lib")},
                capture_output=True,
                text=True,
                timeout=10,
                check=False,
            )
        finally:
            release.set()
        result = pending.result(timeout=5)
    assert competing.returncode == 2, competing.stdout + competing.stderr
    assert "locked" in competing.stderr
    assert result["verdict"] == "FAIL"
    task = active_task(repo)
    assert task is not None and task["id"] == task_id
    assert current_receipt(repo) is None
    monkeypatch.setattr(
        "arc_policy.verify._run",
        lambda cwd, argv: subprocess.CompletedProcess(argv, 0, "", ""),
    )
    assert verify_task(repo, policy)["verdict"] == "PASS"


def test_javascript_without_test_script_cannot_hide_behind_python(
    tmp_path: Path, monkeypatch
) -> None:
    repo = make_repo(tmp_path)
    (repo / "pyproject.toml").write_text("[project]\nname='root'\n")
    web = repo / "web"
    web.mkdir()
    (web / "package.json").write_text('{"name":"web"}\n')
    (web / "index.js").write_text("export const changed = true;\n")
    begin_task(repo, skill="arc-code", slug="demo")
    calls = []

    def run(cwd, argv):
        calls.append(cwd)
        return subprocess.CompletedProcess(argv, 0, "", "")

    monkeypatch.setattr("arc_policy.verify._run", run)
    result = verify_task(repo, policy_for(repo))
    assert result["verdict"] == "FAIL"
    tests = next(check for check in result["checks"] if check["id"] == "tests")
    assert tests["verdict"] == "NOT_RUN"
    assert any("scripts.test" in entry["reason"] for entry in tests["commands"])
    assert calls == [repo]
    assert current_receipt(repo) is None


def test_cli_cannot_verify_old_plan_against_changed_policy(
    tmp_path: Path, monkeypatch, capsys
) -> None:
    import arc_policy.policy as policies

    repo = make_repo(tmp_path)
    old = [{"argv": [sys.executable, "-c", "pass"]}]
    new = [{"argv": [sys.executable, "-c", "raise SystemExit(7)"]}]
    write_overlay(repo, {"verification": {"test_commands": old}})
    begin_task(repo, skill="arc-code", slug="demo")
    original = policies._read_yaml
    swapped = False

    def read(path):
        nonlocal swapped
        data = original(path)
        if path == repo / ".arc/policy.yaml" and not swapped:
            swapped = True
            write_overlay(repo, {"verification": {"test_commands": new}})
        return data

    monkeypatch.setattr(policies, "_read_yaml", read)
    assert main(["--cwd", str(repo), "task", "verify"]) == 1
    assert swapped
    result = json.loads(capsys.readouterr().out)
    snapshot = next(check for check in result["checks"] if check["id"] == "snapshot")
    assert snapshot["verdict"] == "FAIL"
    assert current_receipt(repo) is None


def test_verify_runs_every_changed_module(tmp_path: Path, monkeypatch) -> None:
    repo = make_repo(tmp_path)
    projects = {
        "后端": {"go.mod": "module example\n", "main.go": "package main\n"},
        "frontend": {
            "Cargo.toml": "[package]\nname='demo'\n",
            "src/main.rs": "fn main() {}\n",
        },
        "another": {"go.mod": "module another\n", "main.go": "package main\n"},
    }
    for project, files in projects.items():
        for name, content in files.items():
            path = repo / project / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(content)
    begin_task(repo, skill="arc-code", slug="demo")
    calls = []
    monkeypatch.setattr("arc_policy.verify.shutil.which", lambda name: f"/bin/{name}")
    monkeypatch.setattr(sys, "platform", "linux")

    def run(cwd, argv):
        calls.append((cwd, argv))
        return subprocess.CompletedProcess(argv, 0, "", "")

    monkeypatch.setattr("arc_policy.verify._run", run)
    assert verify_task(repo, policy_for(repo))["verdict"] == "PASS"
    assert {cwd for cwd, _ in calls} == {repo / project for project in projects}
    assert all("--lib" not in argv for _, argv in calls)


@pytest.mark.parametrize(
    "command",
    [
        "arc-policy task status; git commit --no-verify -m wip",
        "git commit -m wip; arc-policy task status",
        "env LANG=C git commit -m wip",
        "git -c core.quotePath=false commit -m wip",
    ],
)
def test_compound_and_wrapped_commits_are_guarded(tmp_path: Path, command: str) -> None:
    policy = policy_for(make_repo(tmp_path))
    decision = classify(
        tool_name="exec_command",
        tool_input={"cmd": command},
        policy=policy,
        has_task=True,
        receipt_ok=False,
    )
    assert decision["action"] == "deny"


def test_stop_recursion_only_reminds(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    write_overlay(repo, {"stop": {"mode": "block"}})
    begin_task(repo, skill="arc-code", slug="demo")
    (repo / "README.txt").write_text("changed\n")
    result = json.loads(
        run_hook(
            json.dumps(
                {
                    "hook_event_name": "Stop",
                    "stop_hook_active": True,
                    "last_assistant_message": "已完成，可以提交。",
                }
            ),
            cwd=repo,
        )
    )
    assert result.get("decision") != "block"
    assert "task verify" in result["systemMessage"]


@pytest.mark.parametrize("slug", ["../../escape", "/tmp/escape", "..", "bad\nslug"])
def test_cli_rejects_unsafe_task_slugs(tmp_path: Path, capsys, slug: str) -> None:
    repo = make_repo(tmp_path)
    assert (
        main(
            ["--cwd", str(repo), "task", "begin", "--skill", "arc-code", "--slug", slug]
        )
        == 2
    )
    assert "slug" in capsys.readouterr().err


@pytest.mark.parametrize("state", ['{"active": {}}', "{broken json"])
def test_corrupt_state_does_not_allow_controlled_operations(
    tmp_path: Path, state: str
) -> None:
    repo = make_repo(tmp_path)
    state_file = repo / ".arc/policy/state.json"
    state_file.parent.mkdir(parents=True)
    state_file.write_text(state)
    for tool_name, tool_input in [
        ("Write", {"path": "main.go"}),
        ("Bash", {"command": "git commit -m wip"}),
    ]:
        result = json.loads(
            run_hook(
                json.dumps(
                    {
                        "hook_event_name": "PreToolUse",
                        "tool_name": tool_name,
                        "tool_input": tool_input,
                    }
                ),
                cwd=repo,
            )
            or "{}"
        )
        assert result.get("permission") == "deny"


@pytest.mark.parametrize(
    "message", ["The bug is not fixed.", "尚未交付，需要继续验证。"]
)
def test_completion_judgment_understands_negation(message: str, monkeypatch) -> None:
    from arc_policy.classify import looks_like_completion

    monkeypatch.setattr(
        subprocess,
        "run",
        lambda *args, **kwargs: subprocess.CompletedProcess(
            args,
            0,
            json.dumps({"answers": {"completion": {"type": "noul", "noul": 0.02}}}),
            "",
        ),
    )
    assert not looks_like_completion(message, ["fixed", "交付"])


def test_unknown_check_status_is_not_pass() -> None:
    from arc_policy.verify import summarize

    policy = {"verification": {"required": ["tests"], "fail_closed": True}}
    assert summarize([{"id": "tests", "verdict": "BOGUS"}], policy) == "FAIL"


def _python_which(name: str) -> str | None:
    return "/opt/homebrew/bin/uv" if name == "uv" else None


def test_python_colon_path_reuses_parent_venv(tmp_path: Path, monkeypatch) -> None:
    from arc_policy.verify import detect_test_commands

    repo = make_repo(tmp_path)
    (repo / "pyproject.toml").write_text("[project]\nname='root'\n")
    venv_python = repo / ".venv/bin/python"
    venv_python.parent.mkdir(parents=True)
    venv_python.write_text("#!/usr/bin/env python3\n")
    (repo / ".venv/bin/pytest").write_text("#!/usr/bin/env python3\n")
    nested = repo / "Arc" / "arc:verify" / "scripts" / "dbqual"
    nested.mkdir(parents=True)
    (nested / "pyproject.toml").write_text("[project]\nname='dbqual'\n")
    (nested / "test_ok.py").write_text("def test_ok():\n    assert True\n")
    begin_task(repo, skill="arc-code", slug="demo")
    monkeypatch.setattr("arc_policy.verify.shutil.which", _python_which)
    commands = detect_test_commands(repo)
    nested_argv = [argv for argv, cwd in commands if cwd == nested]
    assert nested_argv == [[str(venv_python), "-m", "pytest", "-q"]]
    assert all(argv[:1] != ["uv"] for argv, _ in commands)


def test_python_colon_path_skips_broken_local_venv(tmp_path: Path, monkeypatch) -> None:
    from arc_policy.verify import detect_test_commands

    repo = make_repo(tmp_path)
    venv_python = repo / ".venv/bin/python"
    venv_python.parent.mkdir(parents=True)
    venv_python.write_text("#!/usr/bin/env python3\n")
    (repo / ".venv/bin/pytest").write_text("#!/usr/bin/env python3\n")
    nested = repo / "arc:verify"
    nested.mkdir()
    (nested / "pyproject.toml").write_text("[project]\nname='nested'\n")
    local = nested / ".venv/bin/python"
    local.parent.mkdir(parents=True)
    local.write_text("#!/usr/bin/env python3\n")
    (nested / "test_ok.py").write_text("def test_ok():\n    assert True\n")
    begin_task(repo, skill="arc-code", slug="demo")
    monkeypatch.setattr("arc_policy.verify.shutil.which", _python_which)
    commands = detect_test_commands(repo)
    nested_argv = [argv for argv, cwd in commands if cwd == nested]
    assert nested_argv == [[str(venv_python), "-m", "pytest", "-q"]]


def test_python_colon_path_skips_uv_without_venv(tmp_path: Path, monkeypatch) -> None:
    from arc_policy.verify import detect_test_commands

    repo = make_repo(tmp_path)
    nested = repo / "arc:verify"
    nested.mkdir()
    (nested / "pyproject.toml").write_text("[project]\nname='nested'\n")
    (nested / "test_ok.py").write_text("def test_ok():\n    assert True\n")
    begin_task(repo, skill="arc-code", slug="demo")
    monkeypatch.setattr("arc_policy.verify.shutil.which", _python_which)
    commands = detect_test_commands(repo)
    nested_argv = [argv for argv, cwd in commands if cwd == nested]
    assert nested_argv == [[sys.executable, "-m", "pytest", "-q"]]
    assert all("uv" not in argv for argv, _ in commands)


def test_missing_runner_in_one_module_blocks_receipt(
    tmp_path: Path, monkeypatch
) -> None:
    repo = make_repo(tmp_path)
    for project, marker in [("backend", "go.mod"), ("front", "Cargo.toml")]:
        (repo / project).mkdir()
        (repo / project / marker).write_text("module fixture\n")
    begin_task(repo, skill="arc-code", slug="demo")
    monkeypatch.setattr(sys, "platform", "linux")
    calls = []

    def run(cwd, argv):
        calls.append(cwd)
        if argv[0] == "cargo":
            raise FileNotFoundError(2, "No such file or directory")
        return subprocess.CompletedProcess(argv, 0, "", "")

    monkeypatch.setattr("arc_policy.verify._run", run)
    result = verify_task(repo, policy_for(repo))
    assert result["verdict"] == "FAIL"
    checks = next(item for item in result["checks"] if item["id"] == "tests")
    assert checks["verdict"] == "NOT_RUN"
    assert len(calls) == 2
    assert not receipt_matches_tree(repo, current_receipt(repo))


def test_macos_rust_is_blocked_without_project_runner(
    tmp_path: Path, monkeypatch
) -> None:
    repo = make_repo(tmp_path)
    (repo / "Cargo.toml").write_text("[package]\nname='fixture'\n")
    begin_task(repo, skill="arc-code", slug="demo")
    monkeypatch.setattr(sys, "platform", "darwin")
    monkeypatch.setattr(
        "arc_policy.verify._run",
        lambda *args: pytest.fail("must not compile Rust with cargo test on macOS"),
    )
    result = verify_task(repo, policy_for(repo))
    assert result["verdict"] == "FAIL"
    assert (
        next(item for item in result["checks"] if item["id"] == "tests")["verdict"]
        == "BLOCKED"
    )


@pytest.mark.parametrize(
    "commands", [[], [{"argv": []}], [{"argv": ["true"], "cwd": ".."}]]
)
def test_invalid_explicit_test_plan_is_rejected(tmp_path: Path, commands: list) -> None:
    repo = make_repo(tmp_path)
    write_overlay(repo, {"verification": {"test_commands": commands}})
    begin_task(repo, skill="arc-code", slug="demo")
    with pytest.raises(ValueError):
        verify_task(repo, policy_for(repo))
    assert current_receipt(repo) is None


def test_explicit_test_commands_all_run(tmp_path: Path, monkeypatch) -> None:
    repo = make_repo(tmp_path)
    (repo / "nested").mkdir()
    plan = [
        {"argv": ["project-test", "backend"]},
        {"argv": ["project-test", "front"], "cwd": "nested"},
    ]
    write_overlay(repo, {"verification": {"test_commands": plan}})
    begin_task(repo, skill="arc-code", slug="demo")
    calls = []

    def run(cwd, argv):
        calls.append((cwd, argv))
        return subprocess.CompletedProcess(argv, 0, "", "")

    monkeypatch.setattr("arc_policy.verify._run", run)
    assert verify_task(repo, policy_for(repo))["verdict"] == "PASS"
    assert calls == [
        (repo, ["project-test", "backend"]),
        (repo / "nested", ["project-test", "front"]),
    ]


def test_policy_change_invalidates_receipt_even_if_gitignored(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    (repo / ".gitignore").write_text(".arc/\n")
    policy = receipt_policy(repo)
    begin_task(repo, skill="arc-code", slug="demo")
    verify_task(repo, policy, run_tests=False)
    write_overlay(repo, {"verification": {"required": ["tests", "cli_receipt"]}})
    assert not receipt_matches_tree(repo, current_receipt(repo))


@pytest.mark.parametrize(
    "case",
    [
        "subdirectory",
        "git_option",
        "cd",
        "env_short",
        "env_long",
        "env_short_attached",
        "env_long_attached",
    ],
)
def test_commit_uses_target_repository_receipt(tmp_path: Path, case: str) -> None:
    first_parent = tmp_path / "first"
    first_parent.mkdir()
    first = make_repo(first_parent)
    policy = receipt_policy(first)
    begin_task(first, skill="arc-code", slug="demo")
    verify_task(first, policy, run_tests=False)
    second_parent = tmp_path / "second"
    second_parent.mkdir()
    second = make_repo(second_parent)
    command = "git commit -m fixture"
    tool_input = {"cmd": command, "workdir": str(first)}
    if case == "subdirectory":
        tool_input["workdir"] = str(second)
    elif case == "git_option":
        tool_input["cmd"] = f'git -C "{second}" commit -m fixture'
    elif case == "cd":
        tool_input["cmd"] = f'cd "{second}" && git commit -m fixture'
    else:
        prefix = {
            "env_short": f'env -C "{second}"',
            "env_long": f'env --chdir "{second}"',
            "env_short_attached": f'env -C"{second}"',
            "env_long_attached": f'env --chdir="{second}"',
        }[case]
        tool_input["cmd"] = f"{prefix} git commit -m fixture"
    result = json.loads(
        run_hook(
            json.dumps(
                {
                    "hook_event_name": "PreToolUse",
                    "tool_name": "exec_command",
                    "tool_input": tool_input,
                    "cwd": str(first),
                }
            )
        )
        or "{}"
    )
    assert result.get("permission") == "deny"


@pytest.mark.parametrize(
    "expression",
    [
        '(cd "{second}"); git commit -m fixture',
        '(cd "{second}" && true); git push',
        'cd "{second}" | cat; git commit -m fixture',
        'cd "{second}" & git commit -m fixture',
        'false && cd "{second}"; git commit -m fixture',
    ],
)
def test_shell_cd_cannot_lend_another_repository_receipt(
    tmp_path: Path, expression: str
) -> None:
    first_parent = tmp_path / "first"
    first_parent.mkdir()
    first = make_repo(first_parent)
    begin_task(first, skill="arc-code", slug="unverified")
    second_parent = tmp_path / "second"
    second_parent.mkdir()
    second = make_repo(second_parent)
    policy = receipt_policy(second)
    begin_task(second, skill="arc-code", slug="verified")
    verify_task(second, policy, run_tests=False)
    command = expression.format(second=second)
    result = json.loads(
        run_hook(
            json.dumps(
                {
                    "hook_event_name": "PreToolUse",
                    "tool_name": "Bash",
                    "tool_input": {"command": command},
                    "cwd": str(first),
                }
            )
        )
        or "{}"
    )
    assert result.get("permission") == "deny"


@pytest.mark.parametrize(
    "failure", ["missing", "timeout", "exit", "invalid", "uncertain"]
)
def test_failed_completion_judgment_falls_back_to_markers(
    tmp_path: Path, monkeypatch, failure: str
) -> None:
    import arc_policy.classify as classifier

    repo = make_repo(tmp_path)
    begin_task(repo, skill="arc-code", slug="demo")
    (repo / "README.txt").write_text("changed\n")
    original = subprocess.run

    def run(argv, *args, **kwargs):
        if argv[0] != "jev":
            return original(argv, *args, **kwargs)
        if failure == "missing":
            raise FileNotFoundError(2, "jev not found")
        if failure == "timeout":
            raise subprocess.TimeoutExpired(argv, 5)
        response = (
            {"answers": {"completion": {"type": "noul", "noul": 0.5}}}
            if failure == "uncertain"
            else {}
        )
        return subprocess.CompletedProcess(
            argv, 1 if failure == "exit" else 0, json.dumps(response), ""
        )

    monkeypatch.setattr(classifier.subprocess, "run", run)

    def stop(message: str) -> str:
        return run_hook(
            json.dumps({"hook_event_name": "Stop", "last_assistant_message": message}),
            cwd=repo,
        )

    result = json.loads(stop("Done."))
    assert result["decision"] == "block"
    assert "task verify" in result["reason"]
    assert stop("Still tracing the failing route.") == ""
    assert current_receipt(repo) is None


def test_stop_ignores_preexisting_dirty_work_without_task_edits(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    (repo / "README.txt").write_text("preexisting change\n")
    write_overlay(repo, {"stop": {"mode": "block"}})
    begin_task(repo, skill="arc-code", slug="demo")
    assert run_hook(json.dumps({"hook_event_name": "Stop"}), cwd=repo) == ""


def test_cursor_stop_blocks_once_without_assistant_message(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    begin_task(repo, skill="arc-code", slug="demo")
    (repo / "README.txt").write_text("changed\n")
    payload = {
        "hook_event_name": "stop",
        "status": "completed",
        "loop_count": 0,
        "workspace_roots": [str(repo)],
    }
    result = json.loads(run_hook(json.dumps(payload)))
    assert result["decision"] == "block"
    assert "continue" not in result
    payload["loop_count"] = 1
    result = json.loads(run_hook(json.dumps(payload)))
    assert result.get("decision") != "block"
    assert "systemMessage" in result


def test_git_relative_directory_uses_tool_cwd(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    (repo / "nested").mkdir()
    policy = receipt_policy(repo)
    begin_task(repo, skill="arc-code", slug="demo")
    verify_task(repo, policy, run_tests=False)
    assert (
        run_hook(
            json.dumps(
                {
                    "hook_event_name": "PreToolUse",
                    "tool_name": "exec_command",
                    "cwd": str(repo),
                    "tool_input": {
                        "cmd": "git -C .. commit -m fixture",
                        "workdir": str(repo / "nested"),
                    },
                }
            )
        )
        == ""
    )


@pytest.mark.parametrize(
    "command, receipt_ok",
    [
        ('git commit -m "--no-verify"', True),
        ('git commit -m "-new message"', True),
        ('echo ";" git commit -m fixture', False),
        ('echo "alembic upgrade head"', False),
        ("echo ignored # ; git commit -m fixture", False),
    ],
)
def test_quoted_arguments_are_not_control_flags(
    tmp_path: Path, command: str, receipt_ok: bool
) -> None:
    decision = classify(
        tool_name="Bash",
        tool_input={"command": command},
        policy=policy_for(make_repo(tmp_path)),
        has_task=True,
        receipt_ok=receipt_ok,
    )
    assert decision["action"] == "allow"


@pytest.mark.parametrize(
    "command",
    [
        "echo literal#text; git commit -m fixture",
        "echo ignored # comment\ngit commit -m fixture",
        "g\\\nit commit -m fixture",
        "git commit -n -m fixture",
    ],
)
def test_shell_comments_continuations_and_short_flags_are_guarded(
    tmp_path: Path, command: str
) -> None:
    assert (
        classify(
            tool_name="Bash",
            tool_input={"command": command},
            policy=policy_for(make_repo(tmp_path)),
            has_task=True,
            receipt_ok=False,
        )["action"]
        == "deny"
    )


def deliver(repo: Path) -> None:
    write_overlay(
        repo, {"verification": {"required": ["cli_receipt"]}, "stop": {"mode": "block"}}
    )
    begin_task(repo, skill="arc-code", slug="delivered")
    (repo / "feature.txt").write_text("done\n")
    assert verify_task(repo, policy_for(repo), run_tests=False)["verdict"] == "PASS"
    git(repo, "add", "feature.txt")
    git(repo, "commit", "-q", "-m", "feature")


def test_stop_reports_delivered_task_instead_of_charging_new_work_to_it(
    tmp_path: Path,
) -> None:
    repo = make_repo(tmp_path)
    deliver(repo)
    assert run_hook(json.dumps({"hook_event_name": "Stop"}), cwd=repo) == ""
    (repo / "next.txt").write_text("new work\n")
    result = json.loads(run_hook(json.dumps({"hook_event_name": "Stop"}), cwd=repo))
    assert result["decision"] == "block"
    assert "delivered was delivered at" in result["reason"]
    assert "task begin" in result["reason"]
    assert "next.txt" in result["reason"]
    assert "is unverified" not in result["reason"]
    start = json.loads(
        run_hook(json.dumps({"hook_event_name": "SessionStart"}), cwd=repo)
    )
    assert "was delivered at" in start["systemMessage"]


def test_delivered_task_requires_new_begin_and_is_archived(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    deliver(repo)
    write = json.dumps(
        {
            "hook_event_name": "PreToolUse",
            "tool_name": "Write",
            "tool_input": {"path": "next.txt", "contents": "x\n"},
        }
    )
    denied = json.loads(run_hook(write, cwd=repo))
    assert denied["permission"] == "deny"
    assert "was delivered at" in denied["agent_message"]
    assert main(["--cwd", str(repo), "task", "begin", "--skill", "arc-code", "--slug", "next"]) == 0
    assert run_hook(write, cwd=repo) == ""
    state = load_state(repo)
    assert state["active"]["slug"] == "next"
    assert [task["slug"] for task in state["history"]] == ["delivered"]


def test_delivered_task_ends_without_incomplete(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    deliver(repo)
    assert main(["--cwd", str(repo), "task", "end"]) == 0
    assert active_task(repo) is None


def test_other_commit_does_not_deliver_uncommitted_task_work(tmp_path: Path) -> None:
    repo = make_repo(tmp_path)
    write_overlay(
        repo, {"verification": {"required": ["cli_receipt"]}, "stop": {"mode": "block"}}
    )
    begin_task(repo, skill="arc-code", slug="demo")
    (repo / "feature.txt").write_text("in progress\n")
    assert verify_task(repo, policy_for(repo), run_tests=False)["verdict"] == "PASS"
    (repo / "other.txt").write_text("another session\n")
    git(repo, "add", "other.txt")
    git(repo, "commit", "-q", "-m", "other")
    assert delivered_receipt(repo, active_task(repo)) is None
    result = json.loads(run_hook(json.dumps({"hook_event_name": "Stop"}), cwd=repo))
    assert "demo is unverified" in result["reason"]


def pre_tool_use(cwd: Path, tool: str, tool_input: dict) -> dict:
    raw = run_hook(
        json.dumps(
            {
                "hook_event_name": "PreToolUse",
                "tool_name": tool,
                "tool_input": tool_input,
                "cwd": str(cwd),
            }
        )
    )
    return json.loads(raw) if raw else {}


def workspace_repo(workspace: Path, name: str) -> Path:
    parent = workspace / f"{name}-parent"
    parent.mkdir()
    repo = make_repo(parent)
    return repo.rename(workspace / name)


@pytest.mark.parametrize(
    "tool, key",
    [
        ("search_replace", "file_path"),
        ("write", "file_path"),
        ("Edit", "file_path"),
        ("NotebookEdit", "notebook_path"),
        ("edit", "path"),
        ("apply_patch", "command"),
    ],
)
def test_edit_in_non_git_workspace_uses_target_repository_task(
    tmp_path: Path, tool: str, key: str
) -> None:
    repo = workspace_repo(tmp_path, "A133BMS")
    target = repo / "frontend" / "about.tsx"
    rel = str(target.relative_to(tmp_path))
    tool_input = {key: str(target)}
    if key == "path":
        tool_input = {"path": rel, "oldText": "a", "newText": "b"}
    elif key == "command":
        tool_input = {
            "command": f"*** Begin Patch\n*** Update File: {rel}\n@@\n-a\n+b\n"
        }
    assert pre_tool_use(tmp_path, tool, tool_input).get("permission") == "deny"
    begin_task(repo, skill="arc-code", slug="web-react-stack")
    assert pre_tool_use(tmp_path, tool, tool_input) == {}


def test_working_directory_task_does_not_cover_another_repository(
    tmp_path: Path,
) -> None:
    first = workspace_repo(tmp_path, "first")
    second = workspace_repo(tmp_path, "second")
    begin_task(first, skill="arc-code", slug="demo")
    target = second / "src" / "main.rs"
    denied = pre_tool_use(first, "Write", {"file_path": str(target), "content": "x\n"})
    assert denied.get("permission") == "deny"
    assert f"arc-policy --cwd {second} task begin" in denied["agent_message"]
    assert (
        pre_tool_use(
            first, "Write", {"file_path": str(first / "a.txt"), "content": "x\n"}
        )
        == {}
    )


def test_patch_across_repositories_needs_every_task(tmp_path: Path) -> None:
    first = workspace_repo(tmp_path, "first")
    second = workspace_repo(tmp_path, "second")
    begin_task(first, skill="arc-code", slug="demo")
    patch = (
        "*** Begin Patch\n*** Add File: first/a.txt\n+x\n"
        "*** Update File: first/b.txt\n*** Move to: second/b.txt\n@@\n-a\n+b\n*** End Patch\n"
    )
    denied = pre_tool_use(tmp_path, "apply_patch", {"command": patch})
    assert denied.get("permission") == "deny"
    assert str(second) in denied["agent_message"]
    begin_task(second, skill="arc-code", slug="demo")
    assert pre_tool_use(tmp_path, "apply_patch", {"command": patch}) == {}


def test_edit_outside_every_git_repository_has_no_task_scope(tmp_path: Path) -> None:
    workspace = tmp_path / "ditie"
    workspace.mkdir()
    workspace_repo(workspace, "A133BMS")
    target = workspace / "bms-web" / "index.html"
    assert (
        pre_tool_use(workspace, "write", {"file_path": str(target), "content": "x\n"})
        == {}
    )


def test_edit_without_recognizable_target_in_non_git_workspace_is_denied(
    tmp_path: Path,
) -> None:
    workspace_repo(tmp_path, "A133BMS")
    denied = pre_tool_use(tmp_path, "Write", {"contents": "x\n"})
    assert denied.get("permission") == "deny"
    assert "cannot find the edited path" in denied["agent_message"]


def test_stop_in_non_git_workspace_checks_child_repository_tasks(
    tmp_path: Path,
) -> None:
    repo = workspace_repo(tmp_path, "A133BMS")
    workspace_repo(tmp_path, "idle")
    write_overlay(repo, {"stop": {"mode": "block"}})
    begin_task(repo, skill="arc-code", slug="web-react-stack")
    (repo / "about.tsx").write_text("changed\n")
    result = json.loads(run_hook(json.dumps({"hook_event_name": "Stop"}), cwd=tmp_path))
    assert result["decision"] == "block"
    assert "web-react-stack is unverified" in result["reason"]
    assert str(repo) in result["reason"]


def test_session_start_in_non_git_workspace_points_to_repository_cli(
    tmp_path: Path,
) -> None:
    repo = workspace_repo(tmp_path, "A133BMS")
    begin_task(repo, skill="arc-code", slug="web-react-stack")
    start = json.loads(
        run_hook(json.dumps({"hook_event_name": "SessionStart"}), cwd=tmp_path)
    )
    text = start["systemMessage"]
    assert "is not a Git repository" in text
    assert "arc-policy --cwd <repo> task begin" in text
    assert f"{repo}: web-react-stack" in text
