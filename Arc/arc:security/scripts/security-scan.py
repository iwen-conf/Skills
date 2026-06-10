#!/usr/bin/env python3
from __future__ import annotations

import argparse
import html
import json
import os
import shutil
import subprocess
import sys
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from urllib.parse import urlparse, urlunparse


SEVERITIES = ["critical", "high", "medium", "low", "info", "unknown"]


@dataclass
class Check:
    name: str
    command: list[str]
    cwd: Path
    raw_file: str | None = None
    stdout_file: str | None = None
    stderr_file: str | None = None
    timeout: int = 900
    env: dict[str, str] | None = None


def now_stamp() -> str:
    return datetime.now().strftime("%Y%m%d-%H%M%S")


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


def which(binary: str) -> str | None:
    return shutil.which(binary)


def rel(path: Path, base: Path) -> str:
    try:
        return str(path.relative_to(base))
    except ValueError:
        return str(path)


def normalize_severity(value: object) -> str:
    if not isinstance(value, str) or not value:
        return "unknown"
    value = value.strip().lower()
    aliases = {
        "error": "high",
        "warning": "medium",
        "warn": "medium",
        "note": "info",
        "negligible": "info",
        "moderate": "medium",
    }
    return value if value in SEVERITIES else aliases.get(value, "unknown")


def add_severity(counts: dict[str, int], severity: object, amount: int = 1) -> None:
    key = normalize_severity(severity)
    counts[key] = counts.get(key, 0) + amount


def load_json(path: Path) -> object | None:
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except Exception:
        return None


def load_jsonl(path: Path) -> list[object]:
    rows: list[object] = []
    try:
        for line in path.read_text(encoding="utf-8", errors="replace").splitlines():
            line = line.strip()
            if not line:
                continue
            try:
                rows.append(json.loads(line))
            except json.JSONDecodeError:
                continue
    except Exception:
        return []
    return rows


def file_exists(root: Path, names: list[str]) -> bool:
    return any((root / name).exists() for name in names)


def discover_openapi(root: Path) -> Path | None:
    candidates = [
        "openapi.yaml",
        "openapi.yml",
        "openapi.json",
        "swagger.yaml",
        "swagger.yml",
        "swagger.json",
        "docs/openapi.yaml",
        "docs/openapi.yml",
        "docs/openapi.json",
        "api/openapi.yaml",
        "api/openapi.yml",
        "api/openapi.json",
    ]
    for item in candidates:
        path = root / item
        if path.exists():
            return path
    return None


def resolve_openapi_arg(value: str | None, root: Path) -> str | None:
    if value:
        if "://" in value:
            return value
        return str(Path(value).expanduser().resolve())
    discovered = discover_openapi(root)
    return str(discovered) if discovered else None


def local_url_for_docker(target_url: str) -> str:
    parsed = urlparse(target_url)
    if parsed.hostname not in {"localhost", "127.0.0.1", "::1"}:
        return target_url
    netloc = "host.docker.internal"
    if parsed.port:
        netloc = f"{netloc}:{parsed.port}"
    return urlunparse(parsed._replace(netloc=netloc))


def docker_ready() -> bool:
    if not which("docker"):
        return False
    try:
        subprocess.run(
            ["docker", "info"],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            check=False,
            timeout=10,
        )
    except Exception:
        return False
    return True


def run_check(check: Check, output_dir: Path) -> dict[str, object]:
    logs_dir = output_dir / "logs"
    raw_dir = output_dir / "raw"
    logs_dir.mkdir(parents=True, exist_ok=True)
    raw_dir.mkdir(parents=True, exist_ok=True)

    stdout_path = logs_dir / f"{check.name}.stdout.txt"
    stderr_path = logs_dir / f"{check.name}.stderr.txt"
    if check.stdout_file:
        stdout_path = raw_dir / check.stdout_file
    if check.stderr_file:
        stderr_path = raw_dir / check.stderr_file

    env = os.environ.copy()
    if check.env:
        env.update(check.env)

    started = utc_now()
    try:
        proc = subprocess.run(
            check.command,
            cwd=str(check.cwd),
            env=env,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=check.timeout,
            check=False,
        )
        stdout_path.write_text(proc.stdout or "", encoding="utf-8")
        stderr_path.write_text(proc.stderr or "", encoding="utf-8")
        status = "ok" if proc.returncode == 0 else "findings_or_error"
        return {
            "name": check.name,
            "status": status,
            "exit_code": proc.returncode,
            "started_at": started,
            "finished_at": utc_now(),
            "command": check.command,
            "stdout": rel(stdout_path, output_dir),
            "stderr": rel(stderr_path, output_dir),
            "raw": f"raw/{check.raw_file}" if check.raw_file else None,
        }
    except subprocess.TimeoutExpired as exc:
        stdout_path.write_text(exc.stdout or "", encoding="utf-8")
        stderr_path.write_text(exc.stderr or "", encoding="utf-8")
        return {
            "name": check.name,
            "status": "timeout",
            "exit_code": None,
            "started_at": started,
            "finished_at": utc_now(),
            "command": check.command,
            "stdout": rel(stdout_path, output_dir),
            "stderr": rel(stderr_path, output_dir),
            "raw": f"raw/{check.raw_file}" if check.raw_file else None,
        }
    except FileNotFoundError:
        return {
            "name": check.name,
            "status": "missing",
            "exit_code": None,
            "started_at": started,
            "finished_at": utc_now(),
            "command": check.command,
            "stdout": None,
            "stderr": None,
            "raw": None,
        }


def summarize_gitleaks(path: Path) -> dict[str, object]:
    data = load_json(path)
    count = len(data) if isinstance(data, list) else 0
    counts = {"unknown": count} if count else {}
    return {"finding_count": count, "severity_counts": counts}


def summarize_trivy(path: Path) -> dict[str, object]:
    data = load_json(path)
    counts: dict[str, int] = {}
    total = 0
    if isinstance(data, dict):
        for result in data.get("Results", []) or []:
            if not isinstance(result, dict):
                continue
            for vuln in result.get("Vulnerabilities", []) or []:
                total += 1
                add_severity(counts, vuln.get("Severity") if isinstance(vuln, dict) else None)
            for misconfig in result.get("Misconfigurations", []) or []:
                total += 1
                add_severity(counts, misconfig.get("Severity") if isinstance(misconfig, dict) else None)
            for secret in result.get("Secrets", []) or []:
                total += 1
                add_severity(counts, secret.get("Severity") if isinstance(secret, dict) else "high")
    return {"finding_count": total, "severity_counts": counts}


def summarize_semgrep(path: Path) -> dict[str, object]:
    data = load_json(path)
    counts: dict[str, int] = {}
    total = 0
    if isinstance(data, dict):
        for result in data.get("results", []) or []:
            if not isinstance(result, dict):
                continue
            total += 1
            extra = result.get("extra") if isinstance(result.get("extra"), dict) else {}
            add_severity(counts, extra.get("severity"))
    return {"finding_count": total, "severity_counts": counts}


def summarize_gosec(path: Path) -> dict[str, object]:
    data = load_json(path)
    counts: dict[str, int] = {}
    total = 0
    if isinstance(data, dict):
        for issue in data.get("Issues", []) or []:
            if not isinstance(issue, dict):
                continue
            total += 1
            add_severity(counts, issue.get("severity"))
    return {"finding_count": total, "severity_counts": counts}


def summarize_npm_audit(path: Path) -> dict[str, object]:
    data = load_json(path)
    counts: dict[str, int] = {}
    total = 0
    if isinstance(data, dict):
        metadata = data.get("metadata")
        if isinstance(metadata, dict) and isinstance(metadata.get("vulnerabilities"), dict):
            for severity, amount in metadata["vulnerabilities"].items():
                if severity == "total":
                    continue
                if isinstance(amount, int):
                    add_severity(counts, severity, amount)
                    total += amount
        elif isinstance(data.get("vulnerabilities"), dict):
            for item in data["vulnerabilities"].values():
                if isinstance(item, dict):
                    total += 1
                    add_severity(counts, item.get("severity"))
    return {"finding_count": total, "severity_counts": counts}


def summarize_govulncheck(path: Path) -> dict[str, object]:
    rows = load_jsonl(path)
    total = 0
    for row in rows:
        if isinstance(row, dict) and ("finding" in row or "osv" in row):
            total += 1
    return {"finding_count": total, "severity_counts": {"unknown": total} if total else {}}


def summarize_nuclei(path: Path) -> dict[str, object]:
    rows = load_jsonl(path)
    counts: dict[str, int] = {}
    total = 0
    for row in rows:
        if not isinstance(row, dict):
            continue
        total += 1
        info = row.get("info") if isinstance(row.get("info"), dict) else {}
        add_severity(counts, info.get("severity"))
    return {"finding_count": total, "severity_counts": counts}


def summarize_zap(path: Path) -> dict[str, object]:
    data = load_json(path)
    counts: dict[str, int] = {}
    total = 0
    if isinstance(data, dict):
        sites = data.get("site") or data.get("sites") or []
        if isinstance(sites, dict):
            sites = [sites]
        for site in sites if isinstance(sites, list) else []:
            alerts = site.get("alerts", []) if isinstance(site, dict) else []
            for alert in alerts:
                if not isinstance(alert, dict):
                    continue
                total += 1
                risk = alert.get("riskdesc") or alert.get("risk")
                add_severity(counts, str(risk).split()[0] if risk else None)
    return {"finding_count": total, "severity_counts": counts}


def summarize_plain_status(path: Path) -> dict[str, object]:
    text = path.read_text(encoding="utf-8", errors="replace") if path.exists() else ""
    failed_markers = ["FAILED", "ERROR", "Falsifying example", "Traceback"]
    count = sum(text.count(marker) for marker in failed_markers)
    return {"finding_count": count, "severity_counts": {"unknown": count} if count else {}}


SUMMARY_BY_TOOL = {
    "gitleaks": summarize_gitleaks,
    "trivy": summarize_trivy,
    "semgrep": summarize_semgrep,
    "gosec": summarize_gosec,
    "govulncheck": summarize_govulncheck,
    "npm-audit": summarize_npm_audit,
    "pnpm-audit": summarize_npm_audit,
    "nuclei": summarize_nuclei,
    "zap": summarize_zap,
    "schemathesis": summarize_plain_status,
}


def summarize_result(result: dict[str, object], output_dir: Path) -> dict[str, object]:
    raw = result.get("raw")
    if not isinstance(raw, str):
        result["finding_count"] = 0
        result["severity_counts"] = {}
        return result
    path = output_dir / raw
    if not path.exists():
        result["finding_count"] = 0
        result["severity_counts"] = {}
        return result
    summarizer = SUMMARY_BY_TOOL.get(str(result["name"]))
    if summarizer is None:
        result["finding_count"] = 0
        result["severity_counts"] = {}
        return result
    summary = summarizer(path)
    result.update(summary)
    return result


def detect_checks(args: argparse.Namespace, root: Path, output_dir: Path) -> tuple[list[Check], list[dict[str, str]]]:
    raw_dir = output_dir / "raw"
    checks: list[Check] = []
    skipped: list[dict[str, str]] = []
    skip = set(args.skip or [])
    env_common = {"SEMGREP_SEND_METRICS": "off", "TRIVY_NO_PROGRESS": "true"}

    def missing(tool: str, reason: str) -> None:
        skipped.append({"tool": tool, "reason": reason})

    if "gitleaks" not in skip:
        if which("gitleaks"):
            checks.append(Check("gitleaks", ["gitleaks", "detect", "--source", str(root), "--report-format", "json", "--report-path", str(raw_dir / "gitleaks.json"), "--redact", "--no-banner", "--exit-code", "0"], root, raw_file="gitleaks.json", env=env_common))
        else:
            missing("gitleaks", "CLI not installed")

    if "trivy" not in skip:
        if which("trivy"):
            checks.append(Check("trivy", ["trivy", "fs", "--format", "json", "--output", str(raw_dir / "trivy.json"), "--scanners", "vuln,secret,misconfig", str(root)], root, raw_file="trivy.json", env=env_common))
        else:
            missing("trivy", "CLI not installed")

    if "semgrep" not in skip:
        if which("semgrep"):
            checks.append(Check("semgrep", ["semgrep", "scan", "--config", "auto", "--json", "--output", str(raw_dir / "semgrep.json"), "--metrics", "off", str(root)], root, raw_file="semgrep.json", env=env_common))
        else:
            missing("semgrep", "CLI not installed")

    has_go = file_exists(root, ["go.mod"])
    if has_go and "govulncheck" not in skip:
        if which("govulncheck"):
            checks.append(Check("govulncheck", ["govulncheck", "-json", "./..."], root, raw_file="govulncheck.jsonl", stdout_file="govulncheck.jsonl", env=env_common))
        else:
            missing("govulncheck", "CLI not installed")
    elif not has_go:
        missing("govulncheck", "go.mod not found")

    if has_go and "gosec" not in skip:
        if which("gosec"):
            checks.append(Check("gosec", ["gosec", "-fmt=json", "-out", str(raw_dir / "gosec.json"), "./..."], root, raw_file="gosec.json", env=env_common))
        else:
            missing("gosec", "CLI not installed")
    elif not has_go:
        missing("gosec", "go.mod not found")

    has_package_json = file_exists(root, ["package.json"])
    if has_package_json:
        if "pnpm-audit" not in skip and file_exists(root, ["pnpm-lock.yaml"]) and which("pnpm"):
            checks.append(Check("pnpm-audit", ["pnpm", "audit", "--json"], root, raw_file="pnpm-audit.json", stdout_file="pnpm-audit.json", env=env_common))
        elif "npm-audit" not in skip and which("npm"):
            checks.append(Check("npm-audit", ["npm", "audit", "--json"], root, raw_file="npm-audit.json", stdout_file="npm-audit.json", env=env_common))
        else:
            missing("npm-audit", "no supported package manager CLI found")
    else:
        missing("npm-audit", "package.json not found")

    openapi = resolve_openapi_arg(args.openapi, root)
    if args.mode == "full" and openapi and "schemathesis" not in skip:
        if which("schemathesis"):
            command = ["schemathesis", "run", openapi, "--max-examples", str(args.max_examples), "--checks", "all"]
            if args.target_url:
                command.extend(["--base-url", args.target_url])
            checks.append(Check("schemathesis", command, root, raw_file="schemathesis.txt", stdout_file="schemathesis.txt", timeout=args.timeout, env=env_common))
        else:
            missing("schemathesis", "CLI not installed")
    elif args.mode == "full" and not openapi:
        missing("schemathesis", "OpenAPI spec not provided or discovered")

    if args.mode == "full" and args.target_url and "nuclei" not in skip:
        if which("nuclei"):
            checks.append(Check("nuclei", ["nuclei", "-u", args.target_url, "-jsonl", "-o", str(raw_dir / "nuclei.jsonl"), "-silent"], root, raw_file="nuclei.jsonl", timeout=args.timeout, env=env_common))
        else:
            missing("nuclei", "CLI not installed")
    elif args.mode == "full" and not args.target_url:
        missing("nuclei", "target URL not provided")

    if args.mode == "full" and args.target_url and "zap" not in skip:
        if docker_ready():
            zap_target = local_url_for_docker(args.target_url)
            checks.append(
                Check(
                    "zap",
                    [
                        "docker",
                        "run",
                        "--rm",
                        "-v",
                        f"{output_dir}:/zap/wrk:rw",
                        "ghcr.io/zaproxy/zaproxy:stable",
                        "zap-baseline.py",
                        "-t",
                        zap_target,
                        "-J",
                        "zap-report.json",
                        "-w",
                        "zap-report.md",
                        "-r",
                        "zap-report.html",
                    ],
                    root,
                    raw_file="zap-report.json",
                    timeout=args.timeout,
                    env=env_common,
                )
            )
        else:
            missing("zap", "docker CLI or daemon not available")
    elif args.mode == "full" and not args.target_url:
        missing("zap", "target URL not provided")

    for name in sorted(skip):
        skipped.append({"tool": name, "reason": "explicitly skipped"})

    return checks, skipped


def install_missing(script_dir: Path, mode: str, include_zap: bool) -> int:
    script = script_dir / "install-security-tools.sh"
    command = [str(script), "--core"]
    if mode == "full" and include_zap:
        command.append("--with-zap")
    proc = subprocess.run(command, check=False)
    return proc.returncode


def aggregate(results: list[dict[str, object]]) -> dict[str, object]:
    total = 0
    severity_counts = {key: 0 for key in SEVERITIES}
    for result in results:
        count = result.get("finding_count")
        if isinstance(count, int):
            total += count
        counts = result.get("severity_counts")
        if isinstance(counts, dict):
            for key, amount in counts.items():
                if isinstance(amount, int):
                    severity_counts[normalize_severity(key)] += amount
    return {
        "finding_count": total,
        "severity_counts": {key: value for key, value in severity_counts.items() if value},
    }


def markdown_table(headers: list[str], rows: list[list[object]]) -> str:
    lines = ["| " + " | ".join(headers) + " |", "| " + " | ".join(["---"] * len(headers)) + " |"]
    for row in rows:
        lines.append("| " + " | ".join(str(item) for item in row) + " |")
    return "\n".join(lines)


def render_markdown(summary: dict[str, object], output_dir: Path) -> str:
    results = summary["results"]
    skipped = summary["skipped"]
    aggregate_data = summary["aggregate"]
    rows = []
    for item in results:
        raw = item.get("raw") or ""
        raw_link = f"[{raw}]({raw})" if raw else ""
        rows.append([
            item["name"],
            item["status"],
            item.get("exit_code", ""),
            item.get("finding_count", 0),
            json.dumps(item.get("severity_counts", {}), ensure_ascii=False),
            raw_link,
        ])
    skipped_rows = [[item["tool"], item["reason"]] for item in skipped]

    report = [
        "# Security Report",
        "",
        f"- Project: `{summary['project_path']}`",
        f"- Generated: `{summary['generated_at']}`",
        f"- Mode: `{summary['mode']}`",
        f"- Target URL: `{summary.get('target_url') or 'not provided'}`",
        f"- OpenAPI: `{summary.get('openapi') or 'not provided'}`",
        f"- Total scanner findings/signals: `{aggregate_data['finding_count']}`",
        f"- Severity counts: `{json.dumps(aggregate_data['severity_counts'], ensure_ascii=False)}`",
        "",
        "## Tool Results",
        "",
        markdown_table(["Tool", "Status", "Exit", "Signals", "Severity Counts", "Raw"], rows),
        "",
        "## Skipped Checks",
        "",
        markdown_table(["Tool", "Reason"], skipped_rows) if skipped_rows else "None.",
        "",
        "## Manual Security Gaps",
        "",
        "- Cross-user object access and tenant isolation.",
        "- Normal user invoking privileged/admin actions.",
        "- Approval workflow bypass and state transition abuse.",
        "- Payment amount, coupon, balance, or credit tampering.",
        "- Upload content validation, path traversal, SSRF, and malware handling.",
        "- Rate limits, idempotency, replay protection, and audit logging.",
        "",
        "## Recommended Arc Handoff",
        "",
        "- Use `arc:fix` for confirmed vulnerabilities with concrete exploit paths.",
        "- Use `arc:build` to add missing security controls or project-native checks.",
        "- Use `arc:audit` for manual AuthZ/business-logic review.",
        "- Use `arc:docs` only when `.lark.json` is active or the user explicitly requests Lark publication.",
        "",
        "## Raw Artifacts",
        "",
        "- `security-summary.json`",
        "- `logs/`",
        "- `raw/`",
    ]
    if (output_dir / "zap-report.html").exists():
        report.append("- `zap-report.html`")
    if (output_dir / "zap-report.md").exists():
        report.append("- `zap-report.md`")
    return "\n".join(report) + "\n"


def render_html(summary: dict[str, object]) -> str:
    results = summary["results"]
    skipped = summary["skipped"]
    aggregate_data = summary["aggregate"]
    severity = aggregate_data["severity_counts"]
    cards = "".join(
        f"<div class='card'><span>{html.escape(key.title())}</span><strong>{value}</strong></div>"
        for key, value in severity.items()
    ) or "<div class='card'><span>Findings</span><strong>0</strong></div>"
    result_rows = "".join(
        "<tr>"
        f"<td>{html.escape(str(item['name']))}</td>"
        f"<td><span class='status'>{html.escape(str(item['status']))}</span></td>"
        f"<td>{html.escape(str(item.get('exit_code', '')))}</td>"
        f"<td>{html.escape(str(item.get('finding_count', 0)))}</td>"
        f"<td><code>{html.escape(json.dumps(item.get('severity_counts', {}), ensure_ascii=False))}</code></td>"
        f"<td>{('<a href=' + html.escape(str(item.get('raw'))) + '>raw</a>') if item.get('raw') else ''}</td>"
        "</tr>"
        for item in results
    )
    skipped_rows = "".join(
        f"<tr><td>{html.escape(item['tool'])}</td><td>{html.escape(item['reason'])}</td></tr>"
        for item in skipped
    ) or "<tr><td colspan='2'>None</td></tr>"
    return f"""<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Security Report</title>
<style>
body {{ font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 0; color: #172026; background: #f6f7f9; }}
header {{ background: #172026; color: white; padding: 28px 32px; }}
main {{ max-width: 1180px; margin: 0 auto; padding: 28px 20px 48px; }}
h1, h2 {{ margin: 0 0 14px; }}
.meta {{ color: #c9d3dc; line-height: 1.7; }}
.grid {{ display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 12px; margin: 20px 0; }}
.card {{ background: white; border: 1px solid #dde3ea; border-radius: 8px; padding: 16px; }}
.card span {{ display: block; color: #607080; font-size: 13px; }}
.card strong {{ font-size: 28px; }}
section {{ background: white; border: 1px solid #dde3ea; border-radius: 8px; padding: 20px; margin-top: 16px; }}
table {{ width: 100%; border-collapse: collapse; font-size: 14px; }}
th, td {{ border-bottom: 1px solid #e7ebef; padding: 10px; text-align: left; vertical-align: top; }}
th {{ color: #40515f; background: #f8fafc; }}
code {{ white-space: pre-wrap; }}
.status {{ display: inline-block; padding: 2px 8px; border-radius: 999px; background: #eef3f8; }}
ul {{ line-height: 1.7; }}
a {{ color: #0b66c3; }}
</style>
</head>
<body>
<header>
<h1>Security Report</h1>
<div class="meta">
Project: {html.escape(str(summary['project_path']))}<br>
Generated: {html.escape(str(summary['generated_at']))}<br>
Mode: {html.escape(str(summary['mode']))}<br>
Target: {html.escape(str(summary.get('target_url') or 'not provided'))}<br>
OpenAPI: {html.escape(str(summary.get('openapi') or 'not provided'))}
</div>
</header>
<main>
<div class="grid">{cards}</div>
<section>
<h2>Tool Results</h2>
<table>
<thead><tr><th>Tool</th><th>Status</th><th>Exit</th><th>Signals</th><th>Severity Counts</th><th>Raw</th></tr></thead>
<tbody>{result_rows}</tbody>
</table>
</section>
<section>
<h2>Skipped Checks</h2>
<table><thead><tr><th>Tool</th><th>Reason</th></tr></thead><tbody>{skipped_rows}</tbody></table>
</section>
<section>
<h2>Manual Security Gaps</h2>
<ul>
<li>Cross-user object access and tenant isolation.</li>
<li>Normal user invoking privileged/admin actions.</li>
<li>Approval workflow bypass and state transition abuse.</li>
<li>Payment amount, coupon, balance, or credit tampering.</li>
<li>Upload content validation, path traversal, SSRF, and malware handling.</li>
<li>Rate limits, idempotency, replay protection, and audit logging.</li>
</ul>
</section>
</main>
</body>
</html>
"""


def write_reports(summary: dict[str, object], output_dir: Path) -> None:
    summary_path = output_dir / "security-summary.json"
    summary_path.write_text(json.dumps(summary, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    md = render_markdown(summary, output_dir)
    (output_dir / "security-report.md").write_text(md, encoding="utf-8")
    html_report = render_html(summary)
    (output_dir / "security-report.html").write_text(html_report, encoding="utf-8")


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run local-first security scans and generate readable reports.")
    parser.add_argument("project_path", nargs="?", default=".", help="Target project root")
    parser.add_argument("--output-dir", help="Report output directory")
    parser.add_argument("--target-url", help="Authorized running app URL for DAST")
    parser.add_argument("--openapi", help="OpenAPI file or URL for Schemathesis")
    parser.add_argument("--mode", choices=["quick", "full"], default="quick", help="Scan depth")
    parser.add_argument("--install-missing", action="store_true", help="Install missing local CLI tools before scanning")
    parser.add_argument("--pull-zap", action="store_true", help="Pull ZAP Docker image when installing missing tools")
    parser.add_argument("--skip", action="append", default=[], help="Tool name to skip; repeatable")
    parser.add_argument("--timeout", type=int, default=900, help="Per-tool timeout in seconds")
    parser.add_argument("--max-examples", type=int, default=30, help="Schemathesis max examples")
    return parser.parse_args(argv)


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    root = Path(args.project_path).expanduser().resolve()
    if not root.exists():
        print(f"Project path does not exist: {root}", file=sys.stderr)
        return 2
    if not root.is_dir():
        print(f"Project path is not a directory: {root}", file=sys.stderr)
        return 2

    output_dir = Path(args.output_dir).expanduser().resolve() if args.output_dir else root / ".arc" / "security" / now_stamp()
    output_dir.mkdir(parents=True, exist_ok=True)
    (output_dir / "raw").mkdir(exist_ok=True)
    (output_dir / "logs").mkdir(exist_ok=True)

    script_dir = Path(__file__).resolve().parent
    if args.install_missing:
        code = install_missing(script_dir, args.mode, args.pull_zap)
        if code != 0:
            print(f"Tool installation exited with {code}; continuing with available tools.", file=sys.stderr)

    checks, skipped = detect_checks(args, root, output_dir)
    results: list[dict[str, object]] = []
    for check in checks:
        print(f"==> Running {check.name}")
        result = run_check(check, output_dir)
        results.append(summarize_result(result, output_dir))

    openapi = resolve_openapi_arg(args.openapi, root)
    summary: dict[str, object] = {
        "schema_version": "1.0.0",
        "generated_at": utc_now(),
        "project_path": str(root),
        "output_dir": str(output_dir),
        "mode": args.mode,
        "target_url": args.target_url,
        "openapi": openapi,
        "results": results,
        "skipped": skipped,
        "aggregate": aggregate(results),
    }
    write_reports(summary, output_dir)
    print()
    print(f"Security report: {output_dir / 'security-report.md'}")
    print(f"HTML report: {output_dir / 'security-report.html'}")
    print(f"Summary JSON: {output_dir / 'security-summary.json'}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
