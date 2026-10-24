"""Exercise the installed entry points of the adapted Waza content helpers."""

import os
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import subprocess
import sys
from threading import Thread

import pytest


ROOT = Path(__file__).resolve().parents[1]
READ = ROOT / "Waza/research-write/scripts"
WRITE = ROOT / "Waza/research-write/scripts/check_punctuation.py"
ARTICLE = """<html><title>Local fixture</title><article>
<p>First source paragraph.</p><p>Second source paragraph.</p>
<p>Third source paragraph.</p><p>Fourth source paragraph.</p>
<p>Fifth source paragraph.</p><p>Sixth source paragraph.</p>
</article></html>""".encode()


def punctuation(text: str, *args: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [sys.executable, "-I", str(WRITE), *args],
        input=text,
        text=True,
        capture_output=True,
        check=False,
        timeout=10,
    )


@pytest.mark.parametrize("style", ["gollum", "obsidian"])
def test_wikilink_checks_label_and_preserves_target(style: str) -> None:
    target = "docs/Python教程"
    label = "中文API"
    link = f"[[{label}|{target}]]" if style == "gollum" else f"[[{target}|{label}]]"
    result = punctuation(link, "--lang", "zh", "--wikilink-style", style)
    assert result.returncode == 1
    assert result.stdout.count("zh-missing-space") == 1
    assert "文A" in result.stdout

    fixed = punctuation(link + "\r\n", "--lang", "zh", "--wikilink-style", style, "--fix")
    assert fixed.returncode == 0
    assert target in fixed.stdout
    assert "中文 API" in fixed.stdout
    assert punctuation(fixed.stdout, "--lang", "zh", "--wikilink-style", style).returncode == 0


@pytest.mark.parametrize(
    ("style", "link"),
    [("gollum", "[[说明:内容|かな-target]]"), ("obsidian", "[[かな-target|说明:内容]]")],
)
def test_wikilink_language_detection_uses_label(style: str, link: str) -> None:
    result = punctuation(link, "--wikilink-style", style)
    assert result.returncode == 1
    assert "zh-halfwidth-punct" in result.stdout
    assert "ja-ascii-punct" not in result.stdout


def test_wikilink_default_remains_gollum() -> None:
    text = "[[Python 教程|docs/Python教程]]"
    default = punctuation(text, "--lang", "zh")
    explicit = punctuation(text, "--lang", "zh", "--wikilink-style", "gollum")
    assert default.returncode == explicit.returncode == 0
    assert default.stdout == explicit.stdout


@pytest.mark.parametrize("style", ["gollum", "obsidian"])
def test_non_prose_targets_and_code_remain_unchanged(style: str) -> None:
    text = "[[docs/Python教程]] [中文 API](docs/Python教程) `中文API`\n```text\n中文API\n```\n"
    result = punctuation(text, "--lang", "zh", "--wikilink-style", style, "--fix")
    assert result.returncode == 0
    assert result.stdout == text


@contextmanager
def source_server(status: int = 200):
    requests = []

    class Handler(BaseHTTPRequestHandler):
        def do_GET(self) -> None:
            requests.append(self.path)
            self.send_response(status)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.end_headers()
            self.wfile.write(ARTICLE)

        def log_message(self, *args) -> None:
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield f"http://127.0.0.1:{server.server_port}", requests
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)


def clean_proxy_environment() -> dict[str, str]:
    return {
        key: value
        for key, value in os.environ.items()
        if key.lower() not in {"http_proxy", "https_proxy", "all_proxy", "no_proxy", "request_method"}
    }


def test_default_fetch_ignores_implicit_network_proxy() -> None:
    with source_server() as (source, source_requests), source_server() as (proxy, proxy_requests):
        env = clean_proxy_environment()
        env.update(http_proxy=proxy, no_proxy="")
        result = subprocess.run(
            [sys.executable, "-I", str(READ / "fetch_local.py"), source + "/article", "--prefer", "stdlib"],
            env=env, text=True, capture_output=True, check=False, timeout=10,
        )
    assert result.returncode == 0, result.stderr
    assert source_requests == ["/article"]
    assert proxy_requests == []
    assert "First source paragraph." in result.stdout


@pytest.mark.parametrize("entry", ["python", "shell"])
def test_explicit_network_proxy_reaches_local_fetch_tier(entry: str) -> None:
    target = "http://waza-proxy-check.invalid/article"
    with source_server() as (proxy, requests):
        command = (
            [sys.executable, "-I", str(READ / "fetch_local.py"), target, "--network-proxy", proxy]
            if entry == "python"
            else ["bash", str(READ / "fetch.sh"), target, proxy]
        )
        result = subprocess.run(
            command, env=clean_proxy_environment(), text=True, capture_output=True, check=False, timeout=10,
        )
    assert result.returncode == 0, result.stderr
    assert requests == [target]
    assert "First source paragraph." in result.stdout
    assert "tier=local status=ok" in result.stderr
    assert "tier=defuddle" not in result.stderr
    assert "tier=jina" not in result.stderr


def test_failed_source_fetch_never_enables_third_party_extraction(tmp_path: Path) -> None:
    marker = tmp_path / "curl-called"
    curl = tmp_path / "curl"
    curl.write_text('#!/bin/sh\n: > "$WAZA_CURL_MARKER"\nexit 90\n')
    curl.chmod(0o755)
    env = clean_proxy_environment()
    env.update(PATH=str(tmp_path) + os.pathsep + env["PATH"], WAZA_CURL_MARKER=str(marker))
    with source_server(status=403) as (source, requests):
        result = subprocess.run(
            ["bash", str(READ / "fetch.sh"), source + "/denied"],
            env=env, text=True, capture_output=True, check=False, timeout=10,
        )
    assert result.returncode == 1
    assert requests == ["/denied"]
    assert not marker.exists()
    assert "tier=local status=fail" in result.stderr
