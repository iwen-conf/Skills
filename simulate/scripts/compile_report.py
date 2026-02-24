#!/usr/bin/env python3
"""
Compile ui-ux-simulation artifacts from events.jsonl into Markdown deliverables.

Outputs (in <run_dir>/):
  - action-log.compiled.md
  - screenshot-manifest.compiled.md
  - report.generated.md  (or in-place update of report.md auto blocks)

Always run with --help first.
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from collections import defaultdict
from datetime import datetime
from pathlib import Path
from typing import Any


_PLACEHOLDER_RE = re.compile(r"\{\{([A-Z0-9_]+)\}\}")


def _render_placeholders(text: str, mapping: dict[str, str]) -> str:
    def replace(match: re.Match[str]) -> str:
        key = match.group(1)
        return mapping.get(key, match.group(0))

    return _PLACEHOLDER_RE.sub(replace, text)


def _read_events(path: Path) -> tuple[list[dict[str, Any]], list[str]]:
    errors: list[str] = []
    events: list[dict[str, Any]] = []
    if not path.exists():
        return events, [f"events file not found: {path}"]

    for i, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        if not line.strip():
            continue
        try:
            obj = json.loads(line)
            if isinstance(obj, dict):
                events.append(obj)
            else:
                errors.append(f"{path}:{i}: JSON value must be an object")
        except json.JSONDecodeError as e:
            errors.append(f"{path}:{i}: invalid JSON ({e})")
    return events, errors


def _step_str(step: Any) -> str:
    try:
        return f"{int(step):04d}"
    except Exception:
        return "????"


def _truncate(s: str, limit: int = 160) -> str:
    s = " ".join(str(s).split())
    return s if len(s) <= limit else s[: limit - 1] + "…"


def _parse_widths_spec(spec: str) -> dict[str, int]:
    """
    Parse a comma-separated widths spec:
      "Action=120,Expected=80,Actual=80"
    """
    spec = (spec or "").strip()
    if not spec:
        return {}
    out: dict[str, int] = {}
    for part in spec.split(","):
        part = part.strip()
        if not part:
            continue
        if "=" not in part:
            raise ValueError(f"Invalid widths spec item (expected k=v): {part}")
        k, v = part.split("=", 1)
        k = k.strip()
        v = v.strip()
        if not k:
            raise ValueError(f"Invalid widths spec item (empty key): {part}")
        out[k] = int(v)
    return out


def _cell(value: Any, *, missing: str = "<MISSING>") -> str:
    if value is None:
        return missing
    s = str(value).strip()
    return s if s else missing


def _apply_col_widths(headers: list[str], rows: list[list[str]], widths: dict[str, int]) -> list[list[str]]:
    if not widths:
        return rows
    out: list[list[str]] = []
    for row in rows:
        formatted: list[str] = []
        for h, v in zip(headers, row):
            limit = widths.get(h)
            formatted.append(_truncate(v, limit) if limit else v)
        out.append(formatted)
    return out


def _render_pipe_table(headers: list[str], rows: list[list[str]], *, align: list[str] | None = None) -> str:
    align = align or ["left"] * len(headers)

    def align_marker(a: str) -> str:
        a = a.lower()
        if a == "right":
            return "---:"
        if a == "center":
            return ":---:"
        return ":---"

    lines = [
        "| " + " | ".join(headers) + " |",
        "| " + " | ".join(align_marker(a) for a in align) + " |",
    ]
    lines.extend("| " + " | ".join(r) + " |" for r in rows)
    return "\n".join(lines)


def _render_tabulate(headers: list[str], rows: list[list[str]], *, align: list[str] | None = None) -> str:
    try:
        from tabulate import tabulate  # type: ignore
    except Exception:
        raise RuntimeError("tabulate is not installed. Install (prefer venv) with: python3 -m pip install tabulate")

    colalign = tuple((a.lower() if a else "left") for a in (align or ["left"] * len(headers)))
    return tabulate(rows, headers=headers, tablefmt="github", colalign=colalign)


def _render_pandas(headers: list[str], rows: list[list[str]]) -> str:
    try:
        import pandas as pd  # type: ignore
    except Exception:
        raise RuntimeError("pandas is not installed. Install (prefer venv) with: python3 -m pip install pandas")

    df = pd.DataFrame(rows, columns=headers)
    try:
        return df.to_markdown(index=False)  # requires tabulate
    except Exception as e:
        raise RuntimeError(f"pandas to_markdown failed (is tabulate installed?): {e}")


def _render_py_markdown_table(
    headers: list[str],
    rows: list[list[str]],
    *,
    widths: dict[str, int] | None,
    row_sep: str,
    padding_width: int,
    padding_weight: str,
) -> str:
    data = [{h: v for h, v in zip(headers, row)} for row in rows]

    # Prefer the modern package: py_markdown_table.markdown_table.markdown_table
    try:
        from py_markdown_table.markdown_table import markdown_table  # type: ignore

        params: dict[str, Any] = {
            "row_sep": row_sep,
            "padding_width": padding_width,
            "padding_weight": padding_weight,
            "quote": True,
        }
        if widths:
            params["multiline"] = {h: widths[h] for h in headers if h in widths}
        return markdown_table(data).set_params(**params).get_markdown()
    except Exception:
        pass

    # Fallback: older package variants may expose `markdownTable`.
    try:
        from markdownTable import markdownTable  # type: ignore

        table = markdownTable(data)
        # Best-effort: not all variants support all params.
        try:
            table.setParams(row_sep=row_sep, padding_width=padding_width, padding_weight=padding_weight)  # type: ignore
        except Exception:
            pass
        try:
            if widths:
                table.setParams(multiline={h: widths[h] for h in headers if h in widths})  # type: ignore
        except Exception:
            pass
        return table.getMarkdown()  # type: ignore
    except Exception as e:
        raise RuntimeError(
            "py-markdown-table is not installed (or incompatible). Install with:\n"
            "  python3 -m pip install py-markdown-table  (prefer venv)\n"
            f"Underlying error: {e}"
        )


def _render_table(
    headers: list[str],
    rows: list[list[str]],
    *,
    backend: str,
    widths: dict[str, int] | None = None,
    align: list[str] | None = None,
    py_row_sep: str = "always",
    py_padding_width: int = 1,
    py_padding_weight: str = "left",
) -> str:
    backend_key = backend.lower().replace("-", "_").strip()
    if backend_key == "auto":
        for candidate in ["tabulate", "native"]:
            try:
                return _render_table(
                    headers,
                    rows,
                    backend=candidate,
                    widths=widths,
                    align=align,
                    py_row_sep=py_row_sep,
                    py_padding_width=py_padding_width,
                    py_padding_weight=py_padding_weight,
                )
            except Exception:
                continue
        return _render_pipe_table(headers, rows, align=align)

    if backend_key in {"native", "pipe"}:
        rows2 = _apply_col_widths(headers, rows, widths or {})
        return _render_pipe_table(headers, rows2, align=align)

    if backend_key == "tabulate":
        rows2 = _apply_col_widths(headers, rows, widths or {})
        return _render_tabulate(headers, rows2, align=align)

    if backend_key == "pandas":
        rows2 = _apply_col_widths(headers, rows, widths or {})
        return _render_pandas(headers, rows2)

    if backend_key == "py_markdown_table":
        return _render_py_markdown_table(
            headers,
            rows,
            widths=widths,
            row_sep=py_row_sep,
            padding_width=py_padding_width,
            padding_weight=py_padding_weight,
        )

    raise ValueError(f"Unknown table backend: {backend}")


def _mdformat_files(files: list[Path]) -> None:
    try:
        import mdformat as _  # noqa: F401
    except Exception:
        raise SystemExit(
            "mdformat is not installed. Install (prefer venv) with:\n"
            "  python3 -m pip install mdformat\n"
        )

    failures: list[str] = []
    for p in files:
        result = subprocess.run([sys.executable, "-m", "mdformat", str(p)], capture_output=True, text=True)
        if result.returncode != 0:
            failures.append(f"{p}: {result.stderr.strip() or result.stdout.strip()}")
    if failures:
        print("\n".join(f"[FAIL] {f}" for f in failures))
        raise SystemExit(1)


def _format_event_line(ev: dict[str, Any]) -> str:
    step = _step_str(ev.get("step"))
    role = str(ev.get("role", "-"))
    kind = str(ev.get("kind", "note")).upper()
    ts = str(ev.get("ts", ""))
    msg = (
        ev.get("msg")
        or ev.get("message")
        or ev.get("text")
        or ev.get("cmd")
        or ev.get("path")
        or json.dumps(ev, ensure_ascii=False)
    )
    ts_part = f"[{ts}]" if ts else ""
    return f"[STEP {step}][{role}][{kind}]{ts_part} {msg}"


def _is_fail(ev: dict[str, Any]) -> bool:
    result = str(ev.get("result", "")).upper()
    if result == "FAIL":
        return True
    if ev.get("kind") in {"error", "exception"}:
        return True
    return False


def _is_pass(ev: dict[str, Any]) -> bool:
    return str(ev.get("result", "")).upper() == "PASS"


def _collect_by_step(events: list[dict[str, Any]]) -> dict[int, list[dict[str, Any]]]:
    grouped: dict[int, list[dict[str, Any]]] = defaultdict(list)
    for ev in events:
        step_raw = ev.get("step")
        try:
            step = int(step_raw)
        except Exception:
            continue
        grouped[step].append(ev)
    return dict(sorted(grouped.items(), key=lambda kv: kv[0]))


def _generate_action_log_md(*, events: list[dict[str, Any]], source: str) -> str:
    lines = [
        "# Action Log (Compiled)",
        "",
        f"- Generated at: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}",
        f"- Source: `{source}`",
        "",
        "```text",
    ]
    lines.extend(_format_event_line(ev) for ev in events)
    lines.append("```")
    lines.append("")
    return "\n".join(lines)


def _generate_screenshot_manifest_md(
    *,
    events: list[dict[str, Any]],
    source: str,
    table_backend: str,
    widths: dict[str, int],
    py_row_sep: str,
    py_padding_width: int,
    py_padding_weight: str,
) -> str:
    rows: list[list[str]] = []
    for ev in events:
        kind = str(ev.get("kind", "")).lower()
        path = ev.get("path")
        if kind != "screenshot" and not path:
            continue
        if not path:
            continue
        rows.append(
            [
                _step_str(ev.get("step")),
                f"`{_cell(path, missing='<MISSING_PATH>')}`",
                _cell(ev.get("ts"), missing="<MISSING_TS>"),
                _cell(ev.get("url"), missing="<MISSING_URL>"),
                _cell(ev.get("description"), missing="<MISSING_DESCRIPTION>"),
                _cell(ev.get("expectation") or ev.get("expected"), missing="<MISSING_EXPECTATION>"),
                _cell(ev.get("result"), missing="<MISSING_RESULT>"),
            ]
        )

    header = [
        "# Screenshot Manifest (Compiled)",
        "",
        f"- Generated at: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}",
        f"- Source: `{source}`",
        "",
    ]
    if not rows:
        rows = [["0000", "-", "-", "-", "-", "-", "-"]]

    headers = ["Step", "Path", "Captured At", "URL", "Description", "Expectation", "Result"]
    body = _render_table(
        headers,
        rows,
        backend=table_backend,
        widths=widths,
        align=["center", "left", "left", "left", "left", "left", "center"],
        py_row_sep=py_row_sep,
        py_padding_width=py_padding_width,
        py_padding_weight=py_padding_weight,
    )
    return "\n".join(header + [body, ""])


def _generate_steps_table_md(
    events: list[dict[str, Any]],
    *,
    table_backend: str,
    widths: dict[str, int],
    py_row_sep: str,
    py_padding_width: int,
    py_padding_weight: str,
) -> str:
    by_step = _collect_by_step(events)
    headers = ["Step", "Role", "Action", "Expected", "Actual", "Evidence", "Result"]
    rows: list[list[str]] = []

    for step, step_events in by_step.items():
        role = _cell(next((e.get("role") for e in step_events if e.get("role")), None), missing="<MISSING_ROLE>")

        exec_cmds = [
            str(e.get("cmd"))
            for e in step_events
            if str(e.get("kind", "")).lower() == "exec" and e.get("cmd")
        ]
        action_raw = (
            "; ".join(exec_cmds)
            if exec_cmds
            else next((e.get("action") for e in step_events if e.get("action")), None)
        )
        action = _cell(action_raw, missing="<MISSING_ACTION>")

        verify = next((e for e in step_events if str(e.get("kind", "")).lower() == "verify"), None)
        expected = (
            _cell(verify.get("expected", verify.get("expectation")), missing="<MISSING_EXPECTED>") if verify else "<MISSING_EXPECTED>"
        )
        actual = _cell(verify.get("actual"), missing="<MISSING_ACTUAL>") if verify else "<MISSING_ACTUAL>"

        screenshot = next((e for e in step_events if (str(e.get("kind", "")).lower() == "screenshot" and e.get("path"))), None)
        evidence = f"`{screenshot.get('path')}`" if screenshot else "<MISSING_EVIDENCE>"

        result = "FAIL" if any(_is_fail(e) for e in step_events) else ("PASS" if any(_is_pass(e) for e in step_events) else "<UNKNOWN>")

        rows.append([f"{step:04d}", role, action, expected, actual, evidence, result])

    if not rows:
        rows = [["0000", "-", "-", "-", "-", "-", "-"]]

    return _render_table(
        headers,
        rows,
        backend=table_backend,
        widths=widths,
        align=["center", "left", "left", "left", "left", "left", "center"],
        py_row_sep=py_row_sep,
        py_padding_width=py_padding_width,
        py_padding_weight=py_padding_weight,
    )


def _replace_auto_block(text: str, *, start_marker: str, end_marker: str, new_block: str) -> str:
    if start_marker not in text or end_marker not in text:
        raise ValueError(f"Missing markers: {start_marker} ... {end_marker}")

    before, rest = text.split(start_marker, 1)
    _, after = rest.split(end_marker, 1)
    return f"{before}{start_marker}\n{new_block}\n{end_marker}{after}"


def _compute_result(events: list[dict[str, Any]]) -> str:
    if any(_is_fail(e) for e in events):
        return "FAIL"
    if any(_is_pass(e) for e in events):
        return "PASS"
    return "<PASS|FAIL>"


def _parse_ts(ts: Any) -> datetime | None:
    if not ts:
        return None
    if isinstance(ts, (int, float)):
        try:
            return datetime.fromtimestamp(float(ts))
        except Exception:
            return None
    s = str(ts).strip()
    if not s:
        return None
    try:
        # Accept "Z" suffix
        return datetime.fromisoformat(s.replace("Z", "+00:00"))
    except Exception:
        return None


def _compute_time_range(events: list[dict[str, Any]]) -> tuple[str, str]:
    parsed = [t for t in (_parse_ts(e.get("ts")) for e in events) if t is not None]
    if not parsed:
        return "<start_time>", "<end_time>"
    start = min(parsed).astimezone().strftime("%Y-%m-%d %H:%M:%S")
    end = max(parsed).astimezone().strftime("%Y-%m-%d %H:%M:%S")
    return start, end


def _generate_failure_summary_md(*, run_dir: Path, events: list[dict[str, Any]]) -> str:
    failures_dir = run_dir / "failures"
    failure_files = sorted(failures_dir.glob("failure-*.md")) if failures_dir.exists() else []
    if failure_files:
        return "\n".join(f"- `{p.relative_to(run_dir)}`" for p in failure_files)

    by_step = _collect_by_step(events)
    failing_steps = [step for step, evs in by_step.items() if any(_is_fail(e) for e in evs)]
    if failing_steps:
        return "\n".join(
            f"- Step `{step:04d}` -> FAIL (see `screenshot-manifest.compiled.md` / `action-log.compiled.md`)"
            for step in failing_steps
        )
    return "- _No failures recorded._"


def main() -> None:
    parser = argparse.ArgumentParser(description="Compile ui-ux-simulation reports from events.jsonl")
    parser.add_argument("--run-dir", required=True, help="Run directory (e.g. reports/<run_id>/)")
    parser.add_argument("--events", default="events.jsonl", help="Events JSONL filename (default: events.jsonl)")
    parser.add_argument("--strict", action="store_true", help="Fail on any JSON parse errors")
    parser.add_argument(
        "--table-backend",
        default="auto",
        help='Table backend: auto|native|tabulate|pandas|py-markdown-table (default: auto)',
    )
    parser.add_argument(
        "--steps-widths",
        default="Action=120,Expected=80,Actual=80,Evidence=80",
        help='Steps table column widths (k=v, comma-separated). Example: "Action=120,Expected=80,Actual=80"',
    )
    parser.add_argument(
        "--manifest-widths",
        default="Path=80,URL=80,Description=80,Expectation=80",
        help='Manifest table column widths (k=v, comma-separated). Example: "Path=80,Description=80,Expectation=80"',
    )
    parser.add_argument("--py-row-sep", default="always", help="py-markdown-table row_sep (default: always)")
    parser.add_argument("--py-padding-width", type=int, default=1, help="py-markdown-table padding_width (default: 1)")
    parser.add_argument("--py-padding-weight", default="left", help="py-markdown-table padding_weight (default: left)")
    parser.add_argument(
        "--in-place",
        action="store_true",
        help="Update report.md auto blocks in-place (requires markers in report.md)",
    )
    parser.add_argument("--report", default="report.md", help="Report template filename (default: report.md)")
    parser.add_argument("--out-report", default="report.generated.md", help="Generated report filename (default: report.generated.md)")
    parser.add_argument("--beautify-md", action="store_true", help="Run mdformat on generated markdown files")
    args = parser.parse_args()

    run_dir = Path(args.run_dir)
    events_path = run_dir / args.events

    events, errors = _read_events(events_path)
    if errors:
        for e in errors:
            print(f"[WARN] {e}")
        if args.strict:
            raise SystemExit(1)

    try:
        steps_widths = _parse_widths_spec(args.steps_widths)
        manifest_widths = _parse_widths_spec(args.manifest_widths)
    except Exception as e:
        raise SystemExit(f"Invalid widths spec: {e}")

    action_log_md = _generate_action_log_md(events=events, source=str(events_path.name))
    manifest_md = _generate_screenshot_manifest_md(
        events=events,
        source=str(events_path.name),
        table_backend=args.table_backend,
        widths=manifest_widths,
        py_row_sep=args.py_row_sep,
        py_padding_width=args.py_padding_width,
        py_padding_weight=args.py_padding_weight,
    )
    steps_table_md = _generate_steps_table_md(
        events,
        table_backend=args.table_backend,
        widths=steps_widths,
        py_row_sep=args.py_row_sep,
        py_padding_width=args.py_padding_width,
        py_padding_weight=args.py_padding_weight,
    )

    (run_dir / "action-log.compiled.md").write_text(action_log_md, encoding="utf-8")
    (run_dir / "screenshot-manifest.compiled.md").write_text(manifest_md, encoding="utf-8")

    report_path = run_dir / args.report
    template_text = report_path.read_text(encoding="utf-8") if report_path.exists() else ""

    start_time, end_time = _compute_time_range(events)
    mapping = {
        "RESULT": _compute_result(events),
        "START_TIME": start_time,
        "END_TIME": end_time,
        "GENERATED_AT": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
    }

    if template_text:
        report_text = _render_placeholders(template_text, mapping)
        if "<!-- AUTO_STEPS_TABLE_START -->" in report_text:
            report_text = _replace_auto_block(
                report_text,
                start_marker="<!-- AUTO_STEPS_TABLE_START -->",
                end_marker="<!-- AUTO_STEPS_TABLE_END -->",
                new_block=steps_table_md,
            )
        if "<!-- AUTO_FAILURE_SUMMARY_START -->" in report_text:
            report_text = _replace_auto_block(
                report_text,
                start_marker="<!-- AUTO_FAILURE_SUMMARY_START -->",
                end_marker="<!-- AUTO_FAILURE_SUMMARY_END -->",
                new_block=_generate_failure_summary_md(run_dir=run_dir, events=events),
            )
        if args.in_place:
            report_path.write_text(report_text, encoding="utf-8")
        else:
            (run_dir / args.out_report).write_text(report_text, encoding="utf-8")
    else:
        # Fallback: generate a minimal report if report.md does not exist.
        minimal = [
            "# E2E UI/UX Simulation Report (Generated)",
            "",
            f"- Run dir: `{run_dir}`",
            f"- Generated at: {mapping['GENERATED_AT']}",
            f"- Result: {mapping['RESULT']}",
            "",
            "## Step-by-step Execution",
            "",
            steps_table_md,
            "",
            "## Artifacts",
            f"- `action-log.compiled.md`",
            f"- `screenshot-manifest.compiled.md`",
            "",
        ]
        (run_dir / args.out_report).write_text("\n".join(minimal), encoding="utf-8")

    if args.beautify_md:
        report_out = report_path if args.in_place else (run_dir / args.out_report)
        _mdformat_files(
            [p for p in [report_out, run_dir / "action-log.compiled.md", run_dir / "screenshot-manifest.compiled.md"] if p.exists()]
        )

    print(str(run_dir))


if __name__ == "__main__":
    main()
