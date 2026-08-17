from __future__ import annotations

from pathlib import Path
from typing import Any

import yaml  # type: ignore[import-untyped]

TriggerEntry = dict[str, Any]
TriggerCorpus = dict[str, TriggerEntry]


def trigger_corpus_path(root: Path) -> Path:
    return root / "schemas" / "trigger_corpus.yaml"


def load_trigger_corpus(root: Path) -> TriggerCorpus:
    path = trigger_corpus_path(root)
    if not path.exists():
        return {}
    data = yaml.safe_load(path.read_text(encoding="utf-8")) or {}
    skills = data.get("skills", data)
    if not isinstance(skills, dict):
        return {}
    return {str(name): value for name, value in skills.items() if isinstance(value, dict)}


def missing_trigger_terms(description: str, entry: TriggerEntry) -> list[str]:
    haystack = description.lower()
    missing: list[str] = []
    for term in entry.get("must_contain", []):
        token = str(term).strip()
        if token and token.lower() not in haystack:
            missing.append(token)
    return missing


def score_utterance(utterance: str, entry: TriggerEntry) -> int:
    haystack = utterance.lower()
    score = 0
    for term in entry.get("must_contain", []):
        token = str(term).strip().lower()
        if token and token in haystack:
            score += 1
    return score


def rank_skills(utterance: str, corpus: TriggerCorpus) -> list[tuple[str, int]]:
    ranked = [(name, score_utterance(utterance, entry)) for name, entry in corpus.items()]
    ranked.sort(key=lambda item: (-item[1], item[0]))
    return ranked


def winner(utterance: str, corpus: TriggerCorpus) -> str | None:
    ranked = rank_skills(utterance, corpus)
    if not ranked or ranked[0][1] <= 0:
        return None
    top_name, top_score = ranked[0]
    ties = [name for name, score in ranked if score == top_score]
    if len(ties) != 1:
        return None
    return top_name
