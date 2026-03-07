import importlib.util
from pathlib import Path

from arc_core.privacy import has_private, redact_private, strip_private

SCRIPT_PATH = Path(__file__).resolve().parents[1] / "scripts" / "arc_privacy.py"
SCRIPT_SPEC = importlib.util.spec_from_file_location("arc_privacy_script", SCRIPT_PATH)
if SCRIPT_SPEC is None or SCRIPT_SPEC.loader is None:
    raise RuntimeError(f"Unable to load script wrapper from {SCRIPT_PATH}")

script_module = importlib.util.module_from_spec(SCRIPT_SPEC)
SCRIPT_SPEC.loader.exec_module(script_module)
script_redact_private = script_module.redact_private


def test_has_private_detects_private_tag() -> None:
    assert has_private("hello <private>secret</private> world")


def test_redact_private_masks_reason() -> None:
    text = 'hello <private reason="token">secret</private> world'
    assert redact_private(text) == "hello [PRIVATE: token] world"


def test_strip_private_removes_tagged_content() -> None:
    text = "hello <private>secret</private> world"
    assert strip_private(text) == "hello  world"


def test_script_wrapper_reexports_core_behavior() -> None:
    text = 'hello <private reason="token">secret</private> world'
    assert script_redact_private(text) == redact_private(text)
