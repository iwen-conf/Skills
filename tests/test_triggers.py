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
    corpus = load_trigger_corpus(ROOT)
    for name, entry in corpus.items():
        for utterance in entry["positive"]:
            assert winner(utterance, corpus) == name, f"{utterance!r} -> expected {name}"


def test_negative_utterances_do_not_select_the_skill() -> None:
    corpus = load_trigger_corpus(ROOT)
    for name, entry in corpus.items():
        for utterance in entry["negative"]:
            assert winner(utterance, corpus) != name, f"{utterance!r} incorrectly selected {name}"


def test_lifecycle_phases_cover_the_full_engineering_cycle() -> None:
    required_phases = {
        "define",
        "clarify",
        "design",
        "plan",
        "implement",
        "search",
        "verify",
        "secure",
        "repair",
        "operate",
        "document",
        "route",
    }
    assert required_phases <= set(LIFECYCLE_PHASES.values())
    corpus = load_trigger_corpus(ROOT)
    assert set(LIFECYCLE_PHASES) == set(corpus)
