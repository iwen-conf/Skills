#!/usr/bin/env python3
"""Read-only HarmonyOS documentation search client for Meilisearch.

Uses only Python's standard library. Secrets are never printed in diagnostics.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Iterable

ROOT = Path(__file__).resolve().parent
PACKAGED_CONFIG_PATH = ROOT / "config" / "meilisearch.json"
USER_CONFIG_PATH = Path.home() / ".config" / "harmonyos-docs" / "meilisearch.json"


def _default_config_path() -> Path:
    """Explicit override, then the user's private config (holds the key), then the keyless packaged default."""
    override = os.getenv("HARMONYOS_MEILI_CONFIG", "").strip()
    if override:
        return Path(override).expanduser()
    return USER_CONFIG_PATH if USER_CONFIG_PATH.is_file() else PACKAGED_CONFIG_PATH


DEFAULT_CONFIG_PATH = _default_config_path()

TITLE_KEYS = ("title", "name", "heading", "pageTitle", "page_title", "documentTitle")
URL_KEYS = ("url", "href", "sourceUrl", "source_url", "canonicalUrl", "canonical_url")
PATH_KEYS = ("path", "sourcePath", "source_path", "file", "filename", "docPath", "doc_path")
CONTENT_KEYS = (
    "content",
    "text",
    "markdown",
    "body",
    "description",
    "summary",
    "plainText",
    "plain_text",
)
META_PRIORITY_KEYS = (
    "apiLevel",
    "api_level",
    "since",
    "version",
    "systemCapability",
    "system_capability",
    "category",
    "type",
    "module",
    "namespace",
    "breadcrumbs",
)


@dataclass(frozen=True)
class Config:
    base_url: str
    index: str
    api_key: str
    timeout_seconds: float = 20.0

    @classmethod
    def load(cls, path: Path = DEFAULT_CONFIG_PATH) -> "Config":
        try:
            raw = json.loads(path.read_text(encoding="utf-8"))
        except FileNotFoundError as exc:
            raise RuntimeError(f"Config file not found: {path}") from exc
        except json.JSONDecodeError as exc:
            raise RuntimeError(f"Invalid JSON config: {path}: {exc}") from exc

        base_url = os.getenv("HARMONYOS_MEILI_URL", str(raw.get("base_url", ""))).strip().rstrip("/")
        index = os.getenv("HARMONYOS_MEILI_INDEX", str(raw.get("index", ""))).strip()
        api_key = os.getenv("HARMONYOS_MEILI_KEY", str(raw.get("api_key", ""))).strip()
        timeout_raw = os.getenv("HARMONYOS_MEILI_TIMEOUT", str(raw.get("timeout_seconds", 20)))

        try:
            timeout_seconds = float(timeout_raw)
        except ValueError as exc:
            raise RuntimeError("HARMONYOS_MEILI_TIMEOUT must be numeric") from exc

        if not base_url.startswith(("http://", "https://")):
            raise RuntimeError("Meilisearch base_url must start with http:// or https://")
        if not index:
            raise RuntimeError("Meilisearch index is empty")
        if not api_key:
            raise RuntimeError("Meilisearch API key is empty")
        if timeout_seconds <= 0:
            raise RuntimeError("Meilisearch timeout must be > 0")

        return cls(base_url=base_url, index=index, api_key=api_key, timeout_seconds=timeout_seconds)


class MeiliError(RuntimeError):
    pass


class HarmonyDocsClient:
    def __init__(self, config: Config):
        self.config = config

    @property
    def search_url(self) -> str:
        index = urllib.parse.quote(self.config.index, safe="")
        return f"{self.config.base_url}/indexes/{index}/search"

    @property
    def health_url(self) -> str:
        return f"{self.config.base_url}/health"

    def _request_json(
        self,
        url: str,
        *,
        method: str,
        payload: dict[str, Any] | None = None,
        authenticated: bool = True,
    ) -> dict[str, Any]:
        body = None
        headers = {"Accept": "application/json"}
        if authenticated:
            headers["Authorization"] = f"Bearer {self.config.api_key}"
        if payload is not None:
            body = json.dumps(payload, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
            headers["Content-Type"] = "application/json"

        request = urllib.request.Request(url, data=body, headers=headers, method=method)
        try:
            with urllib.request.urlopen(request, timeout=self.config.timeout_seconds) as response:
                data = response.read()
        except urllib.error.HTTPError as exc:
            detail = ""
            try:
                detail = exc.read().decode("utf-8", errors="replace")[:800]
            except Exception:
                pass
            suffix = f": {detail}" if detail else ""
            raise MeiliError(f"HTTP {exc.code} from Meilisearch{suffix}") from exc
        except urllib.error.URLError as exc:
            raise MeiliError(f"Cannot reach Meilisearch: {exc.reason}") from exc
        except TimeoutError as exc:
            raise MeiliError("Meilisearch request timed out") from exc

        try:
            decoded = json.loads(data.decode("utf-8"))
        except (UnicodeDecodeError, json.JSONDecodeError) as exc:
            raise MeiliError("Meilisearch returned a non-JSON response") from exc
        if not isinstance(decoded, dict):
            raise MeiliError("Meilisearch returned an unexpected JSON shape")
        return decoded

    def search(self, query: str, *, limit: int = 5, offset: int = 0) -> dict[str, Any]:
        if limit < 1 or limit > 100:
            raise ValueError("limit must be between 1 and 100")
        if offset < 0:
            raise ValueError("offset must be >= 0")
        return self._request_json(
            self.search_url,
            method="POST",
            payload={"q": query, "limit": limit, "offset": offset},
            authenticated=True,
        )

    def server_health(self) -> dict[str, Any]:
        return self._request_json(self.health_url, method="GET", authenticated=False)


def first_text(hit: dict[str, Any], keys: Iterable[str]) -> str | None:
    for key in keys:
        value = hit.get(key)
        if isinstance(value, str) and value.strip():
            return value.strip()
    return None


def flatten_scalar(value: Any, *, max_len: int = 240) -> str | None:
    if value is None or isinstance(value, (dict, list)):
        return None
    text = str(value).strip()
    if not text:
        return None
    return text if len(text) <= max_len else text[: max_len - 1] + "…"


def collect_meta(hit: dict[str, Any]) -> list[tuple[str, str]]:
    excluded = set(TITLE_KEYS) | set(URL_KEYS) | set(PATH_KEYS) | set(CONTENT_KEYS)
    pairs: list[tuple[str, str]] = []
    seen: set[str] = set()

    for key in META_PRIORITY_KEYS:
        if key in hit:
            value = flatten_scalar(hit[key])
            if value is not None:
                pairs.append((key, value))
                seen.add(key)

    for key, raw in hit.items():
        if key in excluded or key in seen or key.startswith("_"):
            continue
        value = flatten_scalar(raw)
        if value is not None:
            pairs.append((key, value))
        if len(pairs) >= 10:
            break
    return pairs


def best_content(hit: dict[str, Any]) -> str | None:
    direct = first_text(hit, CONTENT_KEYS)
    if direct:
        return direct

    # Schema-tolerant fallback: choose the longest human-readable string that is
    # not already used as title/url/path and is long enough to be meaningful.
    ignored_keys = set(TITLE_KEYS) | set(URL_KEYS) | set(PATH_KEYS)
    candidates: list[str] = []
    for key, value in hit.items():
        if key in ignored_keys or key.startswith("_"):
            continue
        if isinstance(value, str) and len(value.strip()) >= 80:
            candidates.append(value.strip())
    return max(candidates, key=len) if candidates else None


def truncate(text: str, max_chars: int) -> str:
    if max_chars <= 0 or len(text) <= max_chars:
        return text
    return text[: max_chars - 1].rstrip() + "…"


def format_result(query: str, response: dict[str, Any], *, max_chars: int = 3200) -> str:
    hits = response.get("hits", [])
    if not isinstance(hits, list):
        hits = []

    total = response.get("estimatedTotalHits")
    if total is None:
        total = response.get("totalHits")
    if total is None:
        total = len(hits)

    lines = [f"QUERY: {query}", f"ESTIMATED_TOTAL_HITS: {total}", f"RETURNED_HITS: {len(hits)}"]
    if not hits:
        lines.append("NO_RESULTS: true")
        return "\n".join(lines)

    for i, raw_hit in enumerate(hits, start=1):
        hit = raw_hit if isinstance(raw_hit, dict) else {"value": raw_hit}
        title = first_text(hit, TITLE_KEYS) or "(untitled)"
        url = first_text(hit, URL_KEYS)
        path = first_text(hit, PATH_KEYS)
        content = best_content(hit)
        meta = collect_meta(hit)

        lines.extend(["", f"=== RESULT {i} ===", f"SOURCE_ID: harmonyos-docs:{i}", f"TITLE: {title}"])
        if url:
            lines.append(f"URL: {url}")
        if path:
            lines.append(f"PATH: {path}")
        if meta:
            lines.append("META: " + "; ".join(f"{k}={v}" for k, v in meta))
        if content:
            lines.append("CONTENT:")
            lines.append(truncate(content, max_chars))
        else:
            compact = json.dumps(hit, ensure_ascii=False, separators=(",", ":"))
            lines.append("RAW_HIT:")
            lines.append(truncate(compact, max_chars))

    return "\n".join(lines)


def print_json(value: Any) -> None:
    json.dump(value, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\n")


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Search indexed official HarmonyOS documentation.")
    parser.add_argument(
        "--config",
        type=Path,
        default=DEFAULT_CONFIG_PATH,
        help=f"config JSON (default: {DEFAULT_CONFIG_PATH})",
    )
    sub = parser.add_subparsers(dest="command", required=True)

    search = sub.add_parser("search", help="search one query")
    search.add_argument("query")
    search.add_argument("--limit", type=int, default=5)
    search.add_argument("--offset", type=int, default=0)
    search.add_argument("--max-chars", type=int, default=3200, help="max content chars per hit; <=0 means unlimited")
    search.add_argument("--json", action="store_true", help="emit raw JSON")

    batch = sub.add_parser("batch", help="search multiple queries independently")
    batch.add_argument("queries", nargs="+")
    batch.add_argument("--limit", type=int, default=5)
    batch.add_argument("--max-chars", type=int, default=2400)
    batch.add_argument("--json", action="store_true")

    inspect = sub.add_parser("inspect", help="inspect one document and field schema")
    inspect.add_argument("--query", default="VpnExtensionAbility")
    inspect.add_argument("--json", action="store_true")

    health = sub.add_parser("health", help="check server, authentication, index, and search")
    health.add_argument("--query", default="VpnExtensionAbility")

    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        config = Config.load(args.config)
        client = HarmonyDocsClient(config)

        if args.command == "search":
            response = client.search(args.query, limit=args.limit, offset=args.offset)
            if args.json:
                print_json(response)
            else:
                print(format_result(args.query, response, max_chars=args.max_chars))
            return 0

        if args.command == "batch":
            responses: dict[str, Any] = {}
            rendered: list[str] = []
            for query in args.queries:
                response = client.search(query, limit=args.limit)
                if args.json:
                    responses[query] = response
                else:
                    rendered.append(format_result(query, response, max_chars=args.max_chars))
            if args.json:
                print_json(responses)
            else:
                print("\n\n##### NEXT QUERY #####\n\n".join(rendered))
            return 0

        if args.command == "inspect":
            response = client.search(args.query, limit=1)
            if args.json:
                print_json(response)
            else:
                hits = response.get("hits")
                first = hits[0] if isinstance(hits, list) and hits else None
                print(f"QUERY: {args.query}")
                if isinstance(first, dict):
                    print("FIELDS: " + ", ".join(sorted(first.keys())))
                    print(format_result(args.query, response, max_chars=2200))
                else:
                    print("NO_RESULTS: true")
            return 0

        if args.command == "health":
            health = client.server_health()
            probe = client.search(args.query, limit=1)
            hits = probe.get("hits")
            hit_count = len(hits) if isinstance(hits, list) else 0
            print("SERVER_HEALTH: " + json.dumps(health, ensure_ascii=False, separators=(",", ":")))
            print(f"AUTH_INDEX_SEARCH: ok")
            print(f"PROBE_QUERY: {args.query}")
            print(f"PROBE_HITS: {hit_count}")
            return 0

        raise RuntimeError(f"Unknown command: {args.command}")
    except (RuntimeError, ValueError, MeiliError) as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 2
    except KeyboardInterrupt:
        print("ERROR: interrupted", file=sys.stderr)
        return 130


if __name__ == "__main__":
    raise SystemExit(main())
