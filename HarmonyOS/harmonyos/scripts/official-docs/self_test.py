#!/usr/bin/env python3
"""Self-test for the harmonyos official-docs lookup tool.

Default mode is fully offline and includes a mock HTTP server that validates the
Meilisearch request shape and Authorization header. Use --live to also test the
configured real endpoint/index/key.
"""

from __future__ import annotations

import argparse
import importlib.util
import json
import os
import py_compile
import subprocess
import sys
import tempfile
import threading
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any, Iterator

ROOT = Path(__file__).resolve().parent
CLIENT_PATH = ROOT / "harmony_docs.py"
FIXTURE_PATH = ROOT / "tests" / "fixtures" / "search_response.json"


def load_client_module():
    spec = importlib.util.spec_from_file_location("harmony_docs", CLIENT_PATH)
    if spec is None or spec.loader is None:
        raise RuntimeError("cannot load harmony_docs.py")
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


def check(condition: bool, message: str) -> None:
    if not condition:
        raise AssertionError(message)


def check_structure() -> None:
    for path in (ROOT / "config" / "meilisearch.json", CLIENT_PATH, ROOT / "self_test.py", FIXTURE_PATH):
        check(path.exists(), f"missing required file: {path.relative_to(ROOT)}")
    references = ROOT.parents[1] / "references" / "official-docs"
    for name in ("search-playbook.md", "development-workflow.md", "grounding-contract.md",
                 "meilisearch-interface.md", "troubleshooting.md"):
        check((references / name).exists(), f"missing reference: official-docs/{name}")


def check_config() -> None:
    # The packaged config never carries the key; it comes from HARMONYOS_MEILI_KEY or the user config.
    config = json.loads((ROOT / "config" / "meilisearch.json").read_text(encoding="utf-8"))
    check(config.get("base_url") == "https://meilisearch.ezer.heiyu.space", "unexpected base_url")
    check(config.get("index") == "harmonyos-docs", "unexpected index")
    check(config.get("api_key", "") == "", "packaged config must not contain an api_key")


def check_python_compiles() -> None:
    with tempfile.TemporaryDirectory() as tmp:
        py_compile.compile(str(CLIENT_PATH), cfile=str(Path(tmp) / "harmony_docs.pyc"), doraise=True)
        py_compile.compile(str(ROOT / "self_test.py"), cfile=str(Path(tmp) / "self_test.pyc"), doraise=True)


class MockHandler(BaseHTTPRequestHandler):
    fixture: dict[str, Any] = {}
    expected_key = "mock-key-for-tests"

    def log_message(self, format: str, *args: Any) -> None:
        return

    def _json(self, status: int, value: dict[str, Any]) -> None:
        data = json.dumps(value).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self) -> None:  # noqa: N802
        if self.path == "/health":
            self._json(200, {"status": "available"})
            return
        self._json(404, {"message": "not found"})

    def do_POST(self) -> None:  # noqa: N802
        if self.path != "/indexes/harmonyos-docs/search":
            self._json(404, {"message": "not found"})
            return
        if self.headers.get("Authorization") != f"Bearer {self.expected_key}":
            self._json(401, {"message": "bad auth"})
            return
        try:
            length = int(self.headers.get("Content-Length", "0"))
            payload = json.loads(self.rfile.read(length).decode("utf-8"))
        except Exception:
            self._json(400, {"message": "bad json"})
            return
        if not isinstance(payload.get("q"), str) or not isinstance(payload.get("limit"), int):
            self._json(400, {"message": "bad search payload"})
            return
        response = dict(self.fixture)
        response["query"] = payload["q"]
        response["limit"] = payload["limit"]
        response["offset"] = payload.get("offset", 0)
        self._json(200, response)


@contextmanager
def mock_server() -> Iterator[str]:
    MockHandler.fixture = json.loads(FIXTURE_PATH.read_text(encoding="utf-8"))
    server = ThreadingHTTPServer(("127.0.0.1", 0), MockHandler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        host, port = server.server_address
        yield f"http://{host}:{port}"
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=2)


def check_mock_protocol(module: Any) -> None:
    with mock_server() as base_url:
        config = module.Config(base_url=base_url, index="harmonyos-docs", api_key="mock-key-for-tests", timeout_seconds=3)
        client = module.HarmonyDocsClient(config)
        health = client.server_health()
        check(health.get("status") == "available", "mock /health failed")
        result = client.search("VpnExtensionAbility", limit=2)
        check(result.get("query") == "VpnExtensionAbility", "mock query not preserved")
        hits = result.get("hits")
        check(isinstance(hits, list) and len(hits) == 2, "mock search hits invalid")
        rendered = module.format_result("VpnExtensionAbility", result, max_chars=500)
        check("SOURCE_ID: harmonyos-docs:1" in rendered, "stable source id missing")
        check("TITLE: VpnExtensionAbility" in rendered, "title normalization failed")
        check("CONTENT:" in rendered, "content normalization failed")
        check("mock-key-for-tests" not in rendered, "secret leaked into formatted output")


def check_cli_end_to_end() -> None:
    with mock_server() as base_url, tempfile.TemporaryDirectory() as tmp:
        config_path = Path(tmp) / "meili.json"
        config_path.write_text(
            json.dumps(
                {
                    "base_url": base_url,
                    "index": "harmonyos-docs",
                    "api_key": "mock-key-for-tests",
                    "timeout_seconds": 3,
                }
            ),
            encoding="utf-8",
        )
        proc = subprocess.run(
            [
                sys.executable,
                str(CLIENT_PATH),
                "--config",
                str(config_path),
                "search",
                "VpnExtensionAbility",
                "--limit",
                "2",
                "--max-chars",
                "500",
            ],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            timeout=10,
            check=False,
        )
        check(proc.returncode == 0, f"CLI returned {proc.returncode}: {proc.stderr}")
        check("SOURCE_ID: harmonyos-docs:1" in proc.stdout, "CLI source id missing")
        check("TITLE: VpnExtensionAbility" in proc.stdout, "CLI title missing")
        check("mock-key-for-tests" not in proc.stdout + proc.stderr, "CLI leaked API key")

        bad = json.loads(config_path.read_text(encoding="utf-8"))
        bad["api_key"] = "wrong-secret-value"
        config_path.write_text(json.dumps(bad), encoding="utf-8")
        proc_bad = subprocess.run(
            [sys.executable, str(CLIENT_PATH), "--config", str(config_path), "search", "VpnExtensionAbility"],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            timeout=10,
            check=False,
        )
        check(proc_bad.returncode != 0, "CLI should fail with invalid credentials")
        check("wrong-secret-value" not in proc_bad.stdout + proc_bad.stderr, "error path leaked API key")


def check_live(module: Any, query: str) -> None:
    config = module.Config.load(ROOT / "config" / "meilisearch.json")
    client = module.HarmonyDocsClient(config)
    health = client.server_health()
    check(isinstance(health, dict), "live health response is not an object")
    result = client.search(query, limit=3)
    hits = result.get("hits")
    check(isinstance(hits, list), "live search missing hits array")
    check(len(hits) > 0, f"live search returned no hits for {query!r}")
    rendered = module.format_result(query, result, max_chars=800)
    check("SOURCE_ID: harmonyos-docs:1" in rendered, "live result formatting failed")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--live", action="store_true", help="also validate the configured real Meilisearch endpoint")
    parser.add_argument("--query", default="VpnExtensionAbility", help="live probe query")
    args = parser.parse_args()

    checks = [
        ("package structure", check_structure),
        ("configuration", check_config),
        ("python compilation", check_python_compiles),
    ]

    try:
        for name, fn in checks:
            fn()
            print(f"PASS: {name}")

        module = load_client_module()
        check_mock_protocol(module)
        print("PASS: mocked Meilisearch protocol + formatter")
        check_cli_end_to_end()
        print("PASS: CLI end-to-end + secret-safe error path")

        if args.live:
            check_live(module, args.query)
            print(f"PASS: live endpoint/index/search ({args.query})")
        else:
            print("SKIP: live endpoint test (run with --live)")

        print("SELF_TEST: PASS")
        return 0
    except Exception as exc:
        print(f"SELF_TEST: FAIL: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
