"""Keep local runtime-install tests out of the developer's real installation."""

import sys

import pytest


@pytest.fixture(autouse=True)
def isolated_arc_runtime(tmp_path, monkeypatch):
    home = str(tmp_path / "arc runtime")
    monkeypatch.setenv("ARC_POLICY_HOME", home)
    # The installed launcher exports ARC_POLICY_ROOT; under `arc-policy task verify`
    # it would point fixture entries at the installed runtime.
    monkeypatch.delenv("ARC_POLICY_ROOT", raising=False)
    # Existing subprocess fixtures snapshot ENV during module collection. Keep their
    # explicit child environments isolated as well as this pytest process.
    helper = sys.modules.get("test_arc_policy")
    if helper is not None:
        monkeypatch.setitem(helper.ENV, "ARC_POLICY_HOME", home)
        monkeypatch.delitem(helper.ENV, "ARC_POLICY_ROOT", raising=False)
