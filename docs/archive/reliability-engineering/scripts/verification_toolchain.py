#!/usr/bin/env python3
"""Plan, install, verify, and lock reliability-testing tools.

This script is intentionally conservative:
- stdlib only;
- never uses sudo;
- never executes curl|sh;
- global tools are installed only into the current user's normal tool locations;
- project dependency changes require --modify-project;
- system prerequisites (Docker/Podman, compilers, kernel features) are detected but not installed.

Examples:
  python scripts/verification_toolchain.py doctor --root .
  python scripts/verification_toolchain.py plan --root . --capability coverage --capability mutation
  python scripts/verification_toolchain.py install --root . --tool cargo-llvm-cov --tool cargo-mutants --apply
  python scripts/verification_toolchain.py install --root . --tool proptest --apply --modify-project
"""
from __future__ import annotations

import argparse
import json
import os
import platform
import shutil
import subprocess
import sys
import time
from dataclasses import dataclass, asdict
from pathlib import Path
from typing import Callable, Iterable


@dataclass(frozen=True)
class Tool:
    id: str
    description: str
    scope: str  # builtin | user | project | container | prerequisite
    languages: tuple[str, ...]
    capabilities: tuple[str, ...]
    check: tuple[str, ...] | None
    verify: tuple[str, ...] | None
    install_kind: str | None
    install_args: tuple[str, ...] = ()
    requires: tuple[str, ...] = ()
    project_mutation: bool = False
    notes: str = ""


TOOLS: dict[str, Tool] = {
    # Rust built-ins / components
    "cargo-test": Tool(
        "cargo-test", "Rust native unit/integration/doc test runner", "builtin", ("rust",),
        ("unit", "integration"), ("cargo", "--version"), ("cargo", "test", "--help"), None,
    ),
    "rust-clippy": Tool(
        "rust-clippy", "Rust lints via rustup component", "user", ("rust",), ("static",),
        ("cargo", "clippy", "--version"), ("cargo", "clippy", "--version"), "rustup-component",
        ("clippy",), requires=("rustup",),
    ),
    "cargo-nextest": Tool(
        "cargo-nextest", "Fast, process-isolated Rust test runner", "user", ("rust",), ("runner",),
        ("cargo", "nextest", "--version"), ("cargo", "nextest", "--version"), "cargo-install-locked",
        ("cargo-nextest",), requires=("cargo",),
        notes="Official source install requires `cargo install cargo-nextest --locked`; prebuilt/binstall may be faster.",
    ),
    "cargo-llvm-cov": Tool(
        "cargo-llvm-cov", "Rust source coverage using LLVM instrumentation", "user", ("rust",), ("coverage",),
        ("cargo", "llvm-cov", "--version"), ("cargo", "llvm-cov", "--version"), "cargo-install-locked-stable",
        ("cargo-llvm-cov",), requires=("cargo",),
    ),
    "cargo-mutants": Tool(
        "cargo-mutants", "Rust mutation testing", "user", ("rust",), ("mutation", "test-the-tests"),
        ("cargo", "mutants", "--version"), ("cargo", "mutants", "--version"), "cargo-install-locked",
        ("cargo-mutants",), requires=("cargo",),
    ),
    "cargo-audit": Tool(
        "cargo-audit", "RustSec dependency vulnerability audit", "user", ("rust",), ("security",),
        ("cargo", "audit", "--version"), ("cargo", "audit", "--version"), "cargo-install",
        ("cargo-audit",), requires=("cargo",),
    ),
    "rust-nightly": Tool(
        "rust-nightly", "Nightly Rust toolchain used by fuzzing/sanitizers/Miri", "user", ("rust",),
        ("fuzz", "memory-ub", "sanitizer"), ("rustup", "run", "nightly", "rustc", "--version"),
        ("rustup", "run", "nightly", "rustc", "--version"), "rustup-toolchain",
        ("nightly",), requires=("rustup",),
    ),
    "cargo-fuzz": Tool(
        "cargo-fuzz", "Coverage-guided Rust fuzzing with libFuzzer", "user", ("rust",), ("fuzz",),
        ("cargo", "fuzz", "--version"), ("cargo", "fuzz", "--version"), "cargo-install",
        ("cargo-fuzz",), requires=("cargo", "rust-nightly"),
        notes="cargo-fuzz currently requires nightly, x86-64/aarch64 Unix-like host, and a C++11 compiler.",
    ),
    "miri": Tool(
        "miri", "Rust MIR interpreter for undefined-behavior and some concurrency checks", "user", ("rust",),
        ("cargo", "+nightly", "miri", "--version"), ("cargo", "+nightly", "miri", "--version"),
        "rustup-nightly-component", ("miri",), requires=("rustup", "rust-nightly"),
        notes="Miri is a nightly rustup component; it does not replace sanitizers or Loom.",
    ),
    # Rust project dependencies
    "proptest": Tool(
        "proptest", "Property-based testing with shrinking", "project", ("rust",), ("property", "model"),
        None, None, "cargo-add-dev", ("proptest@1",), requires=("cargo",), project_mutation=True,
    ),
    "loom": Tool(
        "loom", "Systematic exploration of concurrent Rust executions", "project", ("rust",),
        ("systematic-concurrency",), None, None, "cargo-add-dev", ("loom@0.7",), requires=("cargo",),
        project_mutation=True,
        notes="Loom requires code/test seams that route modeled synchronization through loom types.",
    ),
    "testcontainers-rust": Tool(
        "testcontainers-rust", "Container-backed real-dependency integration tests for Rust", "project", ("rust",),
        ("real-integration",), None, None, "cargo-add-dev", ("testcontainers",), requires=("cargo", "container-engine"),
        project_mutation=True,
    ),
    # Go built-ins / user tools
    "go-test": Tool(
        "go-test", "Go native unit/integration runner", "builtin", ("go",), ("unit", "integration"),
        ("go", "version"), ("go", "test", "-h"), None,
    ),
    "go-race": Tool(
        "go-race", "Go built-in data race detector", "builtin", ("go",), ("race", "concurrency"),
        ("go", "version"), ("go", "test", "-race", "-run", "^$", "./..."), None,
        notes="Availability depends on GOOS/GOARCH and cgo/toolchain support; the doctor only confirms Go is present, actual race execution is evidence-time verification.",
    ),
    "go-fuzz": Tool(
        "go-fuzz", "Go built-in coverage-guided fuzzing", "builtin", ("go",), ("fuzz",),
        ("go", "version"), ("go", "help", "testflag"), None,
    ),
    "go-cover": Tool(
        "go-cover", "Go built-in coverage profiling", "builtin", ("go",), ("coverage",),
        ("go", "version"), ("go", "tool", "cover", "-help"), None,
    ),
    "gotestsum": Tool(
        "gotestsum", "Structured Go test runner/reporting for local and CI", "user", ("go",), ("runner", "reporting"),
        ("gotestsum", "--version"), ("gotestsum", "--version"), "go-install-latest",
        ("gotest.tools/gotestsum",), requires=("go",),
    ),
    "govulncheck": Tool(
        "govulncheck", "Go call-aware known-vulnerability scanner", "user", ("go",), ("security", "static"),
        ("govulncheck", "-version"), ("govulncheck", "-version"), "go-install-latest",
        ("golang.org/x/vuln/cmd/govulncheck",), requires=("go",),
    ),
    "staticcheck": Tool(
        "staticcheck", "High-signal static analysis for Go", "user", ("go",), ("static",),
        ("staticcheck", "-version"), ("staticcheck", "-version"), "go-install-latest",
        ("honnef.co/go/tools/cmd/staticcheck",), requires=("go",),
    ),
    "testcontainers-go": Tool(
        "testcontainers-go", "Container-backed real-dependency integration tests for Go", "project", ("go",),
        ("real-integration",), None, None, "go-get", ("github.com/testcontainers/testcontainers-go",),
        requires=("go", "container-engine"), project_mutation=True,
    ),
    # Business transaction verification helpers
    "psycopg3-test-adapter": Tool(
        "psycopg3-test-adapter", "Pinned Psycopg 3 adapter in a project-local reliability venv", "project-tool", ("all",),
        ("business-transaction-postgres", "postgres-observability"), None, None, "python-venv-package",
        ("psycopg[binary]==3.3.5",),
        notes="Installed under .reliability/py-tools so the test harness does not pollute application dependencies or system Python.",
    ),
    "pgbench": Tool(
        "pgbench", "PostgreSQL official concurrent workload generator with custom scripts and retry/failure accounting",
        "prerequisite", ("all",), ("business-load-postgres",), ("pgbench", "--version"), ("pgbench", "--version"), None,
        notes="Ships with PostgreSQL client/server packages; this bootstrapper detects it but does not install system packages with sudo/Homebrew/apt.",
    ),
    # Cross-language
    "container-engine": Tool(
        "container-engine", "Docker or Podman runtime for isolated integration/fault tests", "prerequisite", ("all",),
        ("real-integration", "network-fault"),
        ("docker", "version", "--format", "{{.Server.Version}}"), None, None,
        notes="If Docker is unavailable, Podman may satisfy the project; this script never installs/starts a privileged daemon.",
    ),
    "toxiproxy": Tool(
        "toxiproxy", "Shopify Toxiproxy network fault-injection service", "container", ("all",),
        ("network-fault",), None, None, "container-pull", ("ghcr.io/shopify/toxiproxy:2.12.0",), requires=("container-engine",),
        notes="Pinned container tag; use a digest pin in hardened CI if your registry workflow supports it.",
    ),
}

# Capability resolution is intentionally small and opinionated. The skill decides whether the
# capability is justified; this script decides which concrete tools can provide it.
CAPABILITY_TOOLS = {
    "rust": {
        "runner": ("cargo-nextest",),
        "coverage": ("cargo-llvm-cov",),
        "mutation": ("cargo-mutants",),
        "fuzz": ("rust-nightly", "cargo-fuzz"),
        "property": ("proptest",),
        "model": ("proptest",),
        "systematic-concurrency": ("loom",),
        "memory-ub": ("rust-nightly", "miri"),
        "real-integration": ("container-engine", "testcontainers-rust"),
        "network-fault": ("container-engine", "toxiproxy"),
        "static": ("rust-clippy",),
        "security": ("cargo-audit",),
    },
    "go": {
        "runner": ("gotestsum",),
        "coverage": ("go-cover",),
        "fuzz": ("go-fuzz",),
        "race": ("go-race",),
        "concurrency": ("go-race",),
        "real-integration": ("container-engine", "testcontainers-go"),
        "network-fault": ("container-engine", "toxiproxy"),
        "security": ("govulncheck",),
        "static": ("staticcheck",),
    },
}


CROSS_CAPABILITY_TOOLS: dict[str, tuple[str, ...]] = {
    "business-transaction-postgres": ("psycopg3-test-adapter",),
    "postgres-observability": ("psycopg3-test-adapter",),
    "business-load-postgres": ("pgbench",),
}

SUITES: dict[str, tuple[str, ...]] = {
    "rust-foundation": ("cargo-test", "rust-clippy", "cargo-nextest", "cargo-llvm-cov", "cargo-audit"),
    "rust-adversarial-tools": ("cargo-mutants", "rust-nightly", "cargo-fuzz", "miri"),
    "go-foundation": ("go-test", "go-race", "go-fuzz", "go-cover", "gotestsum", "govulncheck", "staticcheck"),
    "integration-fault-tools": ("container-engine", "toxiproxy"),
    "postgres-business-transactions": ("psycopg3-test-adapter", "pgbench"),
}


def run(cmd: Iterable[str], cwd: Path | None = None, timeout: float = 5.0, env: dict[str, str] | None = None) -> tuple[int, str]:
    try:
        p = subprocess.run(list(cmd), cwd=cwd, env=env, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=timeout)
        return p.returncode, (p.stdout or "").strip()
    except (FileNotFoundError, subprocess.TimeoutExpired) as e:
        return 127, str(e)


def executable_exists(name: str) -> bool:
    return shutil.which(name) is not None


def detect_project(root: Path) -> set[str]:
    langs: set[str] = set()
    if (root / "Cargo.toml").exists():
        langs.add("rust")
    if (root / "go.mod").exists():
        langs.add("go")
    return langs


def reliability_venv_python(root: Path) -> Path:
    if os.name == "nt":
        return root / ".reliability" / "py-tools" / "Scripts" / "python.exe"
    return root / ".reliability" / "py-tools" / "bin" / "python"


def install_python_venv_package(root: Path, package: str, timeout: float) -> tuple[int, str]:
    venv_dir = root / ".reliability" / "py-tools"
    py = reliability_venv_python(root)
    outputs: list[str] = []
    if not py.exists():
        rc, out = run((sys.executable, "-m", "venv", str(venv_dir)), cwd=root, timeout=timeout)
        outputs.append(out)
        if rc != 0:
            return rc, "\n".join(x for x in outputs if x)
    rc, out = run((str(py), "-m", "pip", "install", package), cwd=root, timeout=timeout)
    outputs.append(out)
    return rc, "\n".join(x for x in outputs if x)


def go_bin_candidate(name: str) -> Path | None:
    if not executable_exists("go"):
        return None
    env = os.environ.copy()
    env["GOTOOLCHAIN"] = "local"
    rc, gobin = run(("go", "env", "GOBIN"), cwd=None, env=env)
    if rc != 0:
        return None
    gobin = gobin.strip()
    if not gobin:
        rc, gopath = run(("go", "env", "GOPATH"), cwd=None, env=env)
        if rc != 0 or not gopath.strip():
            return None
        gobin = str(Path(gopath.strip().split(os.pathsep)[0]) / "bin")
    return Path(gobin) / name


def tool_present(tool: Tool, root: Path) -> tuple[bool, str]:
    if tool.id == "container-engine":
        for engine in ("docker", "podman"):
            if not executable_exists(engine):
                continue
            rc, out = run((engine, "version"), cwd=root)
            if rc == 0:
                return True, f"{engine}: {out.splitlines()[0] if out else 'available'}"
        return False, "neither a working docker nor podman engine was found"

    if tool.id in {"go-test", "go-race", "go-fuzz", "go-cover"}:
        if not executable_exists("go"):
            return False, "go not on PATH"
        env = os.environ.copy()
        env["GOTOOLCHAIN"] = "local"
        rc, out = run(("go", "version"), cwd=None, env=env)
        return rc == 0, out.splitlines()[0] if out else f"exit={rc}"

    if tool.id == "psycopg3-test-adapter":
        py = reliability_venv_python(root)
        if not py.exists():
            return False, f"project-local reliability Python not found at {py}"
        rc, out = run((str(py), "-c", "import psycopg; print(psycopg.__version__)"), cwd=root)
        return rc == 0, (f"psycopg {out.strip()} via {py}" if rc == 0 else out)

    if tool.scope == "project":
        # Project dependency presence is detected textually to avoid requiring third-party TOML parsers.
        cargo = root / "Cargo.toml"
        gomod = root / "go.mod"
        if tool.id in {"proptest", "loom", "testcontainers-rust"} and cargo.exists():
            needle = "testcontainers" if tool.id == "testcontainers-rust" else tool.id
            text = cargo.read_text(encoding="utf-8", errors="replace")
            return (needle in text), (f"found {needle!r} in Cargo.toml" if needle in text else "not declared in Cargo.toml")
        if tool.id == "testcontainers-go" and gomod.exists():
            text = gomod.read_text(encoding="utf-8", errors="replace")
            needle = "github.com/testcontainers/testcontainers-go"
            return (needle in text), (f"found {needle!r} in go.mod" if needle in text else "not declared in go.mod")
        return False, "project manifest not found"

    if tool.id == "toxiproxy":
        for engine in ("docker", "podman"):
            if not executable_exists(engine):
                continue
            rc, out = run((engine, "image", "inspect", tool.install_args[0]), cwd=root)
            if rc == 0:
                return True, f"image present via {engine}"
        return False, "container image not present"

    if tool.id in {"gotestsum", "govulncheck", "staticcheck"} and tool.check is not None:
        cmd = list(tool.check)
        if not executable_exists(cmd[0]):
            candidate = go_bin_candidate(cmd[0])
            if candidate and candidate.exists():
                cmd[0] = str(candidate)
        rc, out = run(cmd, cwd=None)
        return rc == 0, out.splitlines()[0] if out else f"exit={rc}"

    if tool.check is None:
        return False, "no check defined"
    if not executable_exists(tool.check[0]) and tool.check[0] not in {"cargo", "rustup", "go"}:
        return False, f"{tool.check[0]} not on PATH"
    rc, out = run(tool.check, cwd=root)
    return rc == 0, out.splitlines()[0] if out else f"exit={rc}"



def shallow_tool_present(tool: Tool, root: Path) -> tuple[bool, str]:
    """Cheap, side-effect-free presence check used by `plan`.

    It deliberately avoids contacting Docker/Podman daemons or invoking slow toolchains.
    `doctor` and `install` perform authoritative checks.
    """
    if tool.scope == "project":
        cargo = root / "Cargo.toml"
        gomod = root / "go.mod"
        if tool.id in {"proptest", "loom", "testcontainers-rust"} and cargo.exists():
            needle = "testcontainers" if tool.id == "testcontainers-rust" else tool.id
            text = cargo.read_text(encoding="utf-8", errors="replace")
            return needle in text, (f"found {needle!r} in Cargo.toml" if needle in text else "not declared in Cargo.toml")
        if tool.id == "testcontainers-go" and gomod.exists():
            needle = "github.com/testcontainers/testcontainers-go"
            text = gomod.read_text(encoding="utf-8", errors="replace")
            return needle in text, (f"found {needle!r} in go.mod" if needle in text else "not declared in go.mod")
        return False, "project dependency not detected"
    if tool.id == "container-engine":
        for engine in ("docker", "podman"):
            if executable_exists(engine):
                return True, f"{engine} executable present (daemon not probed during plan)"
        return False, "docker/podman executable not on PATH"
    if tool.id == "toxiproxy":
        return False, "container image presence not probed during plan"
    if tool.id == "psycopg3-test-adapter":
        py = reliability_venv_python(root)
        return py.exists(), (f"project-local reliability Python present at {py}" if py.exists() else "project-local reliability Python missing")
    executable_map = {
        "cargo-nextest": "cargo-nextest", "cargo-llvm-cov": "cargo-llvm-cov", "cargo-mutants": "cargo-mutants",
        "cargo-audit": "cargo-audit", "cargo-fuzz": "cargo-fuzz", "gotestsum": "gotestsum",
        "govulncheck": "govulncheck", "staticcheck": "staticcheck", "pgbench": "pgbench",
    }
    exe = executable_map.get(tool.id)
    if exe:
        return executable_exists(exe), (f"{exe} on PATH" if executable_exists(exe) else f"{exe} not on PATH")
    if tool.languages == ("rust",):
        return executable_exists("cargo"), "cargo present; component/toolchain not deeply probed during plan"
    if tool.languages == ("go",):
        return executable_exists("go"), "go present; built-in capability not deeply probed during plan"
    return False, "not deeply probed during plan"

def requirement_present(req: str, root: Path) -> tuple[bool, str]:
    if req in TOOLS:
        return tool_present(TOOLS[req], root)
    if executable_exists(req):
        return True, f"{req} on PATH"
    return False, f"{req} not on PATH"


def install_command(tool: Tool, root: Path) -> tuple[list[str] | None, str | None]:
    k = tool.install_kind
    a = list(tool.install_args)
    if k is None:
        return None, "built-in or prerequisite; no install action"
    if k == "cargo-install-locked":
        return ["cargo", "install", a[0], "--locked"], None
    if k == "cargo-install-locked-stable":
        return ["cargo", "+stable", "install", a[0], "--locked"], None
    if k == "cargo-install":
        return ["cargo", "install", a[0]], None
    if k == "rustup-component":
        return ["rustup", "component", "add", a[0]], None
    if k == "rustup-toolchain":
        return ["rustup", "toolchain", "install", a[0], "--profile", "minimal"], None
    if k == "rustup-nightly-component":
        return ["rustup", "toolchain", "install", "nightly", "--profile", "minimal", "--component", a[0]], None
    if k == "cargo-add-dev":
        return ["cargo", "add", "--dev", a[0]], None
    if k == "go-install-latest":
        return ["go", "install", a[0] + "@latest"], None
    if k == "go-get":
        return ["go", "get", a[0] + "@latest"], None
    if k == "python-venv-package":
        py = reliability_venv_python(root)
        return [str(py), "-m", "pip", "install", a[0]], "venv is created automatically if missing"
    if k == "container-pull":
        engine = "docker" if executable_exists("docker") else "podman" if executable_exists("podman") else None
        if not engine:
            return None, f"no container engine available; required image: {a[0]}"
        return [engine, "pull", a[0]], None
    return None, f"unknown install kind {k}"


def resolve_tools(root: Path, capabilities: list[str], explicit_tools: list[str], suites: list[str]) -> list[str]:
    result: list[str] = []
    seen: set[str] = set()

    def add(tid: str):
        if tid not in TOOLS:
            raise ValueError(f"unknown tool: {tid}")
        if tid not in seen:
            seen.add(tid)
            result.append(tid)

    for tid in explicit_tools:
        add(tid)
    for suite in suites:
        if suite not in SUITES:
            raise ValueError(f"unknown suite: {suite}")
        for tid in SUITES[suite]:
            add(tid)

    langs = detect_project(root)
    for cap in capabilities:
        matched = False
        for lang in sorted(langs):
            tids = CAPABILITY_TOOLS.get(lang, {}).get(cap, ())
            if tids:
                matched = True
                for tid in tids:
                    add(tid)
        # Cross-language capabilities are resolved regardless of project language.
        cross = CROSS_CAPABILITY_TOOLS.get(cap, ())
        if cross:
            matched = True
            for tid in cross:
                add(tid)
        # Cross-language network faulting can still be planned when no manifest is detected.
        if not matched and cap == "network-fault":
            for tid in ("container-engine", "toxiproxy"):
                add(tid)
    return result


def topo_expand(tool_ids: list[str]) -> list[str]:
    out: list[str] = []
    seen: set[str] = set()

    def visit(tid: str):
        if tid in seen:
            return
        seen.add(tid)
        t = TOOLS[tid]
        for req in t.requires:
            if req in TOOLS:
                visit(req)
        out.append(tid)

    for tid in tool_ids:
        visit(tid)
    return out


def get_version(tool: Tool, root: Path) -> str | None:
    present, info = tool_present(tool, root)
    if not present:
        return None
    return info[:1000]


def write_lock(root: Path, selected: list[str], lock_path: Path) -> None:
    payload = {
        "generated_at_epoch": int(time.time()),
        "platform": {
            "system": platform.system(),
            "release": platform.release(),
            "machine": platform.machine(),
            "python": sys.version.split()[0],
        },
        "project_languages": sorted(detect_project(root)),
        "tools": [],
    }
    for tid in selected:
        t = TOOLS[tid]
        present, info = tool_present(t, root)
        payload["tools"].append({
            "id": tid,
            "scope": t.scope,
            "present": present,
            "observed": info,
        })
    lock_path.parent.mkdir(parents=True, exist_ok=True)
    lock_path.write_text(json.dumps(payload, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")


def print_doctor(root: Path, as_json: bool) -> int:
    langs = sorted(detect_project(root))
    rows = []
    for tid, tool in TOOLS.items():
        if tool.languages != ("all",) and not set(tool.languages).intersection(langs):
            continue
        present, info = tool_present(tool, root)
        rows.append({"id": tid, "scope": tool.scope, "present": present, "info": info})
    result = {
        "root": str(root),
        "platform": {"system": platform.system(), "machine": platform.machine()},
        "languages": langs,
        "tools": rows,
    }
    if as_json:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        print(f"root: {root}")
        print(f"languages: {', '.join(langs) if langs else '(none detected)'}")
        for row in rows:
            mark = "OK" if row["present"] else "MISS"
            print(f"{mark:4} {row['id']:<22} [{row['scope']}] {row['info']}")
    return 0


def plan_or_install(args: argparse.Namespace, do_install: bool) -> int:
    root = Path(args.root).resolve()
    try:
        selected = resolve_tools(root, args.capability or [], args.tool or [], args.suite or [])
    except ValueError as e:
        print(str(e), file=sys.stderr)
        return 2
    if not selected:
        print("no tools selected; pass --capability, --tool, and/or --suite", file=sys.stderr)
        return 2
    selected = topo_expand(selected)
    failures = 0
    installed_or_present: list[str] = []

    for tid in selected:
        tool = TOOLS[tid]
        present, info = (tool_present(tool, root) if do_install else shallow_tool_present(tool, root))
        if present:
            print(f"OK   {tid}: {info}")
            installed_or_present.append(tid)
            continue

        print(f"MISS {tid}: {info}")
        for req in tool.requires:
            if req in selected:
                continue
            ok, req_info = requirement_present(req, root)
            if not ok:
                print(f"     prerequisite missing: {req}: {req_info}")

        cmd, reason = install_command(tool, root)
        if cmd:
            print("     install:", " ".join(cmd))
        elif reason:
            print("     action:", reason)

        if not do_install:
            continue

        if tool.scope == "prerequisite":
            print(f"BLOCK {tid}: system prerequisite must be installed/configured outside this script")
            failures += 1
            continue
        if tool.project_mutation and not args.modify_project:
            print(f"BLOCK {tid}: would modify project manifest; re-run with --modify-project")
            failures += 1
            continue
        if cmd is None:
            print(f"BLOCK {tid}: {reason or 'no safe install method'}")
            failures += 1
            continue

        # Ensure executable prerequisites that are not catalog tools exist before execution.
        external_missing = [r for r in tool.requires if r not in TOOLS and not executable_exists(r)]
        catalog_missing = []
        for r in tool.requires:
            if r in TOOLS:
                ok, _ = tool_present(TOOLS[r], root)
                if not ok:
                    catalog_missing.append(r)
        if external_missing or catalog_missing:
            print(f"BLOCK {tid}: missing prerequisite(s): {', '.join(external_missing + catalog_missing)}")
            failures += 1
            continue

        print(f"RUN  {tid}: {' '.join(cmd)}", flush=True)
        if tool.install_kind == "python-venv-package":
            rc, out = install_python_venv_package(root, tool.install_args[0], args.timeout)
        else:
            rc, out = run(cmd, cwd=root, timeout=args.timeout)
        if out:
            print(out[-6000:])
        if rc != 0:
            print(f"FAIL {tid}: installer exit={rc}")
            failures += 1
            continue

        present2, info2 = tool_present(tool, root)
        if present2:
            print(f"PASS {tid}: {info2}")
            installed_or_present.append(tid)
        else:
            print(f"FAIL {tid}: install command succeeded but verification failed: {info2}")
            failures += 1

    if do_install and args.lock_file:
        lock_path = Path(args.lock_file)
        if not lock_path.is_absolute():
            lock_path = root / lock_path
        write_lock(root, selected, lock_path)
        print(f"LOCK {lock_path}")

    return 1 if failures else 0


def build_parser() -> argparse.ArgumentParser:
    ap = argparse.ArgumentParser(description=__doc__)
    sub = ap.add_subparsers(dest="cmd", required=True)

    d = sub.add_parser("doctor", help="detect project languages and current verification tool availability")
    d.add_argument("--root", default=".")
    d.add_argument("--json", action="store_true")

    for name in ("plan", "install"):
        p = sub.add_parser(name, help=("show required tool actions" if name == "plan" else "install selected tools safely"))
        p.add_argument("--root", default=".")
        p.add_argument("--capability", action="append", choices=sorted({c for m in CAPABILITY_TOOLS.values() for c in m} | set(CROSS_CAPABILITY_TOOLS)))
        p.add_argument("--tool", action="append", choices=sorted(TOOLS))
        p.add_argument("--suite", action="append", choices=sorted(SUITES), help="install/plan a curated foundation bundle; use only when justified")
        p.add_argument("--modify-project", action="store_true", help="allow Cargo.toml/go.mod changes for selected test libraries")
        p.add_argument("--timeout", type=float, default=900.0)
        p.add_argument("--lock-file", default=".reliability/toolchain.lock.json")
        if name == "install":
            p.add_argument("--apply", action="store_true", help="execute installs; without this flag install behaves like a plan")

    l = sub.add_parser("list", help="list tool catalog")
    l.add_argument("--json", action="store_true")
    ls = sub.add_parser("list-suites", help="list curated tool suites")
    ls.add_argument("--json", action="store_true")
    return ap


def main() -> int:
    ap = build_parser()
    args = ap.parse_args()
    if args.cmd == "doctor":
        return print_doctor(Path(args.root).resolve(), args.json)
    if args.cmd == "list":
        rows = [asdict(TOOLS[k]) for k in sorted(TOOLS)]
        if args.json:
            print(json.dumps(rows, indent=2, ensure_ascii=False))
        else:
            for row in rows:
                print(f"{row['id']:<22} {row['scope']:<12} {row['description']}")
        return 0
    if args.cmd == "list-suites":
        if args.json:
            print(json.dumps(SUITES, indent=2, ensure_ascii=False))
        else:
            for name, tids in SUITES.items():
                print(f"{name}: {', '.join(tids)}")
        return 0
    if args.cmd == "plan":
        return plan_or_install(args, False)
    if args.cmd == "install":
        return plan_or_install(args, bool(args.apply))
    return 2


if __name__ == "__main__":
    raise SystemExit(main())
