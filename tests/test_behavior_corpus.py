from pathlib import Path

import yaml

from arc_core.domain.triggers import load_trigger_corpus, winner

ROOT = Path(__file__).resolve().parents[1]
REQUIRED_CATEGORIES = {"negation", "multi-intent", "scope", "authorization"}


def _scenarios() -> list[dict]:
    data = yaml.safe_load((ROOT / "schemas" / "behavior_corpus.yaml").read_text(encoding="utf-8"))
    scenarios = data.get("scenarios", [])
    assert isinstance(scenarios, list) and scenarios
    return scenarios


def test_behavior_corpus_covers_required_runtime_categories() -> None:
    scenarios = _scenarios()
    assert REQUIRED_CATEGORIES <= {item["category"] for item in scenarios}
    for item in scenarios:
        assert item["utterance"].strip()
        assert item["expected_runtime"].strip()
        assert item["static_keyword_is_authoritative"] is False


def test_static_keyword_winner_is_not_runtime_evidence() -> None:
    corpus = load_trigger_corpus(ROOT)
    for item in _scenarios():
        static = winner(item["utterance"], corpus)
        expected_static = item.get("static_keyword_winner")
        if expected_static:
            assert static == expected_static, f"{item['id']}: static winner {static!r}"
        # 静态命中不能当成真实路由、范围或授权结论。
        assert item["expected_runtime"] != static
        assert item["static_keyword_is_authoritative"] is False
