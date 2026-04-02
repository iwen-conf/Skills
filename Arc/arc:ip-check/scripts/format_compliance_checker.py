#!/usr/bin/env python3
"""Generate a format-compliance checklist for software copyright filing."""

import argparse
from pathlib import Path


TEMPLATE = """# Format Compliance Checklist
- project_path: {project_path}
- software_name: {software_name}
- version: {version}

## Code Pages
- target pages: first 30 + last 30 (or full if <60)
- requirement: >=50 lines per page, A4, black/white, single-side
- samples (file:start-end lines):
  - FILL_IN: add entries
- code_pages_ok: FILL_IN
- sensitive sections to redact: FILL_IN

## Documentation Pages
- requirement: >=35 lines per page, page header with software name/version/page no., footer with owner
- screenshots naming == software name in header/title
- doc_lines_ok: FILL_IN
- name_consistency: FILL_IN
- signature_page_ready: FILL_IN (electronic sign page prepared)
- non-duty declaration: FILL_IN (for individuals)

## Naming Consistency
- full name: {software_name}
- short name: {short_name}
- must match in: headers, footers, screenshots, application form
- name_consistency: FILL_IN

## App E-Copyright (if targeting app stores)
- recommended: FILL_IN
- required materials: license, ID, code package hash, statements

## Fee Reduction
- eligible: FILL_IN
- basis: FILL_IN
- required proofs: FILL_IN

## Actions
- [ ] Update headers/footers to exact name + version
- [ ] Ensure code page density >=50 lines/page
- [ ] Ensure doc page density >=35 lines/page
- [ ] Prepare signature page / seals
- [ ] Prepare non-duty declaration (individual)
- [ ] Align screenshots names with software full name
- [ ] Attach open-source license statements if required
"""


def main() -> None:
    parser = argparse.ArgumentParser(description="Prepare format compliance checklist")
    parser.add_argument("--project-path", required=True)
    parser.add_argument("--software-name", default="<software-name>")
    parser.add_argument("--short-name", default="<short-name>")
    parser.add_argument("--version", default="V1.0")
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()

    content = TEMPLATE.format(
        project_path=args.project_path,
        software_name=args.software_name,
        short_name=args.short_name,
        version=args.version,
    )

    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(content, encoding="utf-8")


if __name__ == "__main__":
    main()
