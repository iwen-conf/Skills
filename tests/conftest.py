"""Keep local runtime-install tests out of the developer's real installation."""

import sys

import pytest


@pytest.fixture(autouse=True)
def isolated_arc_runtime(tmp_path, monkeypatch):
    home = str(tmp_path / "arc runtime")
    monkeypatch.setenv("ARC_POLICY_HOME", home)
    # Existing subprocess fixtures snapshot ENV during module collection. Keep their
    # explicit child environments isolated as well as this pytest process.
    helper = sys.modules.get("test_arc_policy")
    if helper is not None:
        monkeypatch.setitem(helper.ENV, "ARC_POLICY_HOME", home)
