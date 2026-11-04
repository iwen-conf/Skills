"""Persistent CLI installation independent of a host's disposable plugin cache.

Installation is local and offline: copy the already provisioned PyYAML dependency,
including its license, together with the runtime. Hooks never install dependencies.
"""

from __future__ import annotations

import hashlib
import importlib.metadata
import os
import shlex
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path


def runtime_home() -> Path:
    value = os.environ.get("ARC_POLICY_HOME")
    if value:
        return Path(value).expanduser().absolute()
    data = os.environ.get("XDG_DATA_HOME")
    return (
        Path(data).expanduser() if data else Path.home() / ".local/share"
    ) / "arc-policy"


def persistent_cli() -> Path:
    return runtime_home() / "bin" / "arc-policy"


def _write_executable(path: Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile("w", dir=path.parent, delete=False) as stream:
        stream.write(text)
        temporary = Path(stream.name)
    temporary.chmod(0o755)
    os.replace(temporary, path)


def install_runtime(source: Path, *, bin_dir: Path | None = None) -> Path:
    """Publish a checked snapshot, then atomically switch its persistent entry point."""
    import yaml

    source = source.resolve()
    home = runtime_home()
    versions = home / "versions"
    versions.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix=".install-", dir=home) as temporary:
        stage = Path(temporary) / "runtime"
        shutil.copytree(
            source,
            stage,
            ignore=shutil.ignore_patterns(
                ".venv", "__pycache__", "*.pyc", ".pytest_cache", ".git", "vendor"
            ),
        )
        vendor = stage / "vendor"
        # PyYAML's pure Python implementation avoids retaining a venv or ABI-bound extension.
        shutil.copytree(
            Path(yaml.__file__).resolve().parent,
            vendor / "yaml",
            ignore=shutil.ignore_patterns("__pycache__", "*.pyc", "*.so", "*.pyd"),
        )
        distribution = importlib.metadata.distribution("PyYAML")
        for entry in distribution.files or ():
            if any(part.endswith(".dist-info") for part in entry.parts):
                original = Path(distribution.locate_file(entry))
                if original.is_file() and original.suffix not in {
                    ".pyc",
                    ".so",
                    ".pyd",
                }:
                    destination = vendor / Path(*entry.parts)
                    destination.parent.mkdir(parents=True, exist_ok=True)
                    shutil.copy2(original, destination)
        licenses = [
            path
            for path in vendor.rglob("*")
            if path.is_file() and path.name.lower().startswith(("license", "copying"))
        ]
        if not licenses:
            raise RuntimeError(
                "PyYAML license is missing; refusing an incomplete runtime copy"
            )
        python = Path(getattr(sys, "_base_executable", sys.executable)).absolute()
        env = {
            **os.environ,
            "ARC_POLICY_ROOT": str(stage),
            "PYTHONDONTWRITEBYTECODE": "1",
        }
        check = subprocess.run(
            [str(python), "-S", str(stage / "bin/arc-policy"), "version"],
            capture_output=True,
            text=True,
            env=env,
            check=False,
        )
        if check.returncode:
            raise RuntimeError(
                f"persistent runtime check failed: {check.stderr.strip()}"
            )
        digest = hashlib.sha256()
        for path in sorted(stage.rglob("*")):
            if path.is_file():
                name = str(path.relative_to(stage)).encode("utf-8")
                content = path.read_bytes()
                # Frame every component: path='a', data='bc' must differ from
                # path='ab', data='c'. Preserve executable-mode-only updates too.
                digest.update(len(name).to_bytes(8, "big"))
                digest.update(name)
                digest.update((path.stat().st_mode & 0o777).to_bytes(4, "big"))
                digest.update(len(content).to_bytes(8, "big"))
                digest.update(content)
        destination = versions / digest.hexdigest()[:20]
        if not destination.exists():
            stage.rename(destination)
        # The launcher references current, never a host cache path or an old release.
        current = home / "current"
        replacement = home / (".current-" + str(os.getpid()))
        replacement.symlink_to(destination.relative_to(home), target_is_directory=True)
        os.replace(replacement, current)
        command = (
            "#!/bin/sh\n"
            f"export ARC_POLICY_ROOT={shlex.quote(str(current))}\n"
            f'exec {shlex.quote(str(python))} {shlex.quote(str(current / "bin/arc-policy"))} "$@"\n'
        )
        launcher = persistent_cli()
        _write_executable(launcher, command)
        if bin_dir is not None:
            _write_executable(bin_dir / "arc-policy", command)
        return launcher


def ensure_persistent_cli(source: Path) -> Path:
    launcher = persistent_cli()
    if not launcher.is_file():
        return install_runtime(source)
    return launcher
