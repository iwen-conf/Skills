"""Select the provisioned runtime before importing third-party engine dependencies."""

from __future__ import annotations

import importlib.util
import os
import shlex
import sys
from pathlib import Path


def ensure_runtime() -> None:
    plugin = Path(__file__).resolve().parents[2]
    owners = [plugin]
    source = plugin.parents[1]
    if plugin.parent.name == "plugins" and (source / "pyproject.toml").is_file():
        owners.append(source)
    for owner in owners:
        venv = owner / ".venv"
        python = venv / "bin" / "python"
        if python.is_file():
            if Path(sys.prefix).resolve() != venv.resolve():
                # Re-exec before reading hook stdin; every host keeps the same payload.
                os.execv(str(python), [str(python), *sys.argv])
            break
    if importlib.util.find_spec("yaml") is None:
        venv = plugin / ".venv"
        setup = (
            f"`uv venv --python 3.12 {shlex.quote(str(venv))}`, then "
            if not venv.is_dir()
            else ""
        )
        raise RuntimeError(
            f"PyYAML is unavailable in {sys.executable}. Prepare the plugin runtime with "
            f"{setup}`uv pip install --python {shlex.quote(str(venv / 'bin' / 'python'))} "
            "'PyYAML>=6.0.2'`."
        )
