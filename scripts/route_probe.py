#!/usr/bin/env python3
"""Cross-family routing regression: ask TypeSafe Jev which installed skill each request should load.

The probe simulates description-only selection (the step hosts run before loading a skill). It is evidence about
description quality, not about a host's real router. Requires `jev` (TypeSafe) and an API key; without them the
run fails instead of inventing probabilities.
"""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys

import yaml

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))

from sync_skills import list_skill_dirs, library_owners, published_name, read_metadata  # noqa: E402

CORPUS = ROOT / "schemas" / "route_corpus.yaml"
DEFAULT_EXTERNAL = Path.home() / ".agents" / "skills"
BATCH = 12  # questions per request; one roster copy per request keeps payloads small
INSTRUCTIONS = (
    "A coding agent must load exactly one skill before handling the user request in `requests.{key}`. "
    "Each option is a skill whose full description is in `roster` under the same name. Choose the skill "
    "whose description best covers the request; when the request has two phases, choose the skill for the "
    "first phase."
)


def compact(text: str) -> str:
    return " ".join(str(text).split())


def load_roster(external_root: Path, external_names: list[str]) -> dict[str, str]:
    """Discoverable entrypoints: repository routers and skills plus the named external installs."""
    bundled = set(library_owners(ROOT))
    roster: dict[str, str] = {}
    for directory in list_skill_dirs(ROOT):
        metadata = read_metadata(directory / "SKILL.md")
        if metadata["name"] in bundled:
            continue
        roster[published_name(metadata["name"], "dash")] = compact(metadata.get("description", ""))
    for name in external_names:
        path = external_root / name / "SKILL.md"
        if not path.is_file():
            raise SystemExit(f"external skill missing: {path}")
        text = path.read_text(encoding="utf-8")
        match = re.match(r"^---\n(.*?)\n---\n", text, re.S)
        metadata = yaml.safe_load(match.group(1)) if match else {}
        roster[name] = compact(metadata.get("description", ""))
    return roster


def ask(payload: dict, jev: str) -> dict:
    result = subprocess.run([jev, "ask"], input=json.dumps(payload, ensure_ascii=False), text=True,
                            capture_output=True, check=False)
    if result.returncode != 0:
        raise SystemExit(f"jev ask failed ({result.returncode}): {result.stderr.strip() or result.stdout.strip()}")
    return json.loads(result.stdout)


def probe(cases: list[dict], roster: dict[str, str], jev: str) -> list[dict]:
    rows: list[dict] = []
    for start in range(0, len(cases), BATCH):
        batch = cases[start:start + BATCH]
        keys = [f"q{start + index}" for index in range(len(batch))]
        payload = {
            "state": {"roster": roster, "requests": {key: case["utterance"] for key, case in zip(keys, batch)}},
            "questions": {
                key: {
                    "type": "choice",
                    "instructions": INSTRUCTIONS.format(key=key),
                    "criteria": {name: f"The skill `{name}` described in `roster.{name}`" for name in roster},
                }
                for key in keys
            },
        }
        answers = ask(payload, jev).get("answers", {})
        for key, case in zip(keys, batch):
            answer = answers.get(key)
            if not answer:
                raise SystemExit(f"no answer for {key}: {case['utterance']}")
            ranked = sorted(answer["probabilities"].items(), key=lambda item: -item[1])
            expected = case["expect"] if isinstance(case["expect"], list) else [case["expect"]]
            accepted = sum(answer["probabilities"].get(name, 0.0) for name in expected)
            passed = answer["choice"] in expected and accepted >= case.get("min_confidence", 0.5)
            rows.append({
                "utterance": case["utterance"], "expect": "|".join(expected), "choice": answer["choice"],
                "confidence": round(answer["confidence"], 2),
                "top3": [(name, round(probability, 2)) for name, probability in ranked[:3]],
                "status": "PASS" if passed else "FAIL",
            })
    return rows


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--corpus", type=Path, default=CORPUS)
    parser.add_argument("--external-root", type=Path, default=DEFAULT_EXTERNAL)
    parser.add_argument("--jev", default=os.environ.get("JEV_BIN", "jev"))
    parser.add_argument("--out", type=Path, help="write the full JSON report here")
    args = parser.parse_args(argv)
    corpus = yaml.safe_load(args.corpus.read_text(encoding="utf-8"))
    roster = load_roster(args.external_root.expanduser(), corpus.get("external", []))
    expected_names = {name for case in corpus["cases"]
                      for name in (case["expect"] if isinstance(case["expect"], list) else [case["expect"]])}
    unknown = sorted(expected_names - set(roster))
    if unknown:
        raise SystemExit(f"corpus expects skills missing from the roster: {', '.join(unknown)}")
    rows = probe(corpus["cases"], roster, args.jev)
    failures = [row for row in rows if row["status"] == "FAIL"]
    for row in rows:
        top = ", ".join(f"{name} {probability}" for name, probability in row["top3"])
        print(f"{row['status']} {row['expect']:<16} conf={row['confidence']:<4} {row['utterance']}  [{top}]")
    print(f"\n{len(rows) - len(failures)}/{len(rows)} passed; roster {len(roster)} skills")
    if args.out:
        args.out.parent.mkdir(parents=True, exist_ok=True)
        args.out.write_text(json.dumps({"roster": sorted(roster), "rows": rows}, ensure_ascii=False, indent=2),
                            encoding="utf-8")
    return 1 if failures else 0


if __name__ == "__main__":
    raise SystemExit(main())
