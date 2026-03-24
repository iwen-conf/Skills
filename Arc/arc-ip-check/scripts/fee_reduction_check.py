#!/usr/bin/env python3
"""Fee reduction checker for CN patent fees (85%/70%).

Usage:
  python fee_reduction_check.py --applicant-type individual --annual-income 50000 \
    --co-applicants 1 --output analysis/fee-reduction-assessment.md

This script does not call external services. It encodes the 2025 thresholds:
- individual: annual income < 60000 RMB → eligible
- enterprise: taxable income < 1000000 RMB → eligible
- institution (non-profit/education/research): always eligible
- single eligible applicant → 85% reduction; multiple eligible co-applicants → 70%
"""

import argparse
import json
from pathlib import Path
from typing import Dict, List


def evaluate(applicant_type: str, annual_income: float, co_applicants: int) -> Dict:
    """Return eligibility decision and required proofs.

    Core logic is intentionally simple; extend here if policy changes (e.g., new caps).
    """

    eligible = False
    basis = ""
    reduction = 0
    required: List[str] = []

    if applicant_type == "individual":
        if annual_income < 60000:
            eligible = True
            basis = "annual_income < 60000"
            required = ["income proof (stamped) or tax certificate"]
    elif applicant_type == "enterprise":
        if annual_income < 1_000_000:
            eligible = True
            basis = "taxable_income < 1000000"
            required = ["last fiscal year CIT filing (stamped)"]
    elif applicant_type == "institution":
        eligible = True
        basis = "non-profit/education/research institution"
        required = ["registration certificate or proof of institution nature"]

    if eligible:
        reduction = 85 if co_applicants <= 1 else 70
    return {
        "eligible": eligible,
        "basis": basis,
        "reduction_percent": reduction,
        "required_proofs": required,
        "co_applicants": co_applicants,
    }


def render_markdown(result: Dict, applicant_type: str) -> str:
    header = "# Fee Reduction Assessment\n"
    lines = [header]
    lines.append(f"- applicant_type: {applicant_type}")
    lines.append(f"- co_applicants: {result['co_applicants']}")
    lines.append(f"- eligible: {result['eligible']}")
    lines.append(f"- reduction_percent: {result['reduction_percent']}")
    lines.append(f"- basis: {result['basis'] or 'n/a'}")
    if result["required_proofs"]:
        proofs = "\n  - ".join(["", *result["required_proofs"]])
        lines.append(f"- required_proofs:{proofs}")
    else:
        lines.append("- required_proofs: []")
    lines.append("\n## JSON\n")
    lines.append("```json")
    lines.append(json.dumps(result, indent=2))
    lines.append("```")
    return "\n".join(lines) + "\n"


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Check CN patent fee reduction eligibility"
    )
    parser.add_argument(
        "--applicant-type",
        choices=["individual", "enterprise", "institution"],
        required=True,
    )
    parser.add_argument(
        "--annual-income",
        type=float,
        required=True,
        help="Annual income or taxable income of last year",
    )
    parser.add_argument(
        "--co-applicants", type=int, default=1, help="Number of applicants (>=1)"
    )
    parser.add_argument(
        "--output", type=Path, help="Output markdown path; stdout if omitted"
    )
    args = parser.parse_args()

    co_apps = max(1, args.co_applicants)
    result = evaluate(args.applicant_type, args.annual_income, co_apps)
    content = render_markdown(result, args.applicant_type)

    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(content, encoding="utf-8")
    else:
        print(content)


if __name__ == "__main__":
    main()
