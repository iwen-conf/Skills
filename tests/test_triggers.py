from pathlib import Path

from arc_core.domain.skill import LIFECYCLE_PHASES
from arc_core.domain.triggers import load_trigger_corpus, winner
from arc_core.skill_validation import collect_skill_files, parse_frontmatter

ROOT = Path(__file__).resolve().parents[1]


def test_trigger_corpus_covers_every_skill() -> None:
    corpus = load_trigger_corpus(ROOT)
    names = set()
    for path in collect_skill_files(ROOT):
        frontmatter, error = parse_frontmatter(path.read_text(encoding="utf-8"))
        assert error is None
        names.add(frontmatter["name"])
    assert set(corpus) == names


def test_positive_utterances_select_the_expected_skill() -> None:
    """Static keyword gate: listed positives uniquely win. Not runtime routing."""
    corpus = load_trigger_corpus(ROOT)
    for name, entry in corpus.items():
        for utterance in entry["positive"]:
            assert winner(utterance, corpus) == name, f"{utterance!r} -> expected {name}"


def test_negative_utterances_do_not_select_the_skill() -> None:
    """Static keyword gate: listed negatives do not select that skill."""
    corpus = load_trigger_corpus(ROOT)
    for name, entry in corpus.items():
        for utterance in entry["negative"]:
            assert winner(utterance, corpus) != name, f"{utterance!r} incorrectly selected {name}"


def test_lifecycle_phases_cover_the_full_engineering_cycle() -> None:
    assert set(LIFECYCLE_PHASES.values()) == {"understand", "select", "plan", "change", "prove", "evaluate"}
    corpus = load_trigger_corpus(ROOT)
    assert set(LIFECYCLE_PHASES) == set(corpus)


def test_retired_skill_phrases_still_route_to_their_new_owner() -> None:
    corpus = load_trigger_corpus(ROOT)
    owners = {
        "帮我写PRD": "arc:plan",
        "需求不清，先澄清一下": "arc:plan",
        "先拆任务": "arc:plan",
        "按方案写代码": "arc:code",
        "线上报错了": "arc:code",
        "做前端页面": "arc:code",
        "给控制器加注释": "arc:code",
        "给 Gin SSR 加计时": "arc:code",
        "按 DIP 分层": "arc:code",
        "用 arc-idx 搜代码": "arc:explore",
        "补测试，写测试": "arc:verify",
        "给支付流程做可靠性验证": "arc:verify",
        "给项目做个体检": "arc:audit",
        "检查订单的业务闭环": "arc:plan",
        "修改状态机后重新闭环检查": "arc:plan",
        "跑一次安全扫描": "arc:audit",
        "引入依赖前先做开源选型": "arc:vet",
        "这个库停更三年了还能用吗": "arc:vet",
    }
    for utterance, owner in owners.items():
        assert winner(utterance, corpus) == owner, utterance
    for utterance in ("用 prewalk 省token", "创建飞书项目空间"):
        assert winner(utterance, corpus) is None, utterance
