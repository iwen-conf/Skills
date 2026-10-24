# Meilisearch interface

## Endpoint

The search client targets:

```text
POST {base_url}/indexes/{index}/search
Authorization: Bearer <key>
Content-Type: application/json
```

Default configuration is in `scripts/official-docs/config/meilisearch.json` (keyless; real key in `~/.config/harmonyos-docs/meilisearch.json`).

## Safe operations

This skill's bundled client performs read/search operations only. It never creates, updates, deletes, or reconfigures indexes/documents/keys.

The configured credential currently has broad privileges. Prefer replacing it with a search-only Meilisearch key when operationally convenient. `HARMONYOS_MEILI_KEY` can override the bundled value without editing files.

## CLI contract

```bash
python3 scripts/official-docs/harmony_docs.py search QUERY [--limit N] [--offset N] [--json]
python3 scripts/official-docs/harmony_docs.py batch QUERY [QUERY ...] [--limit N] [--json]
python3 scripts/official-docs/harmony_docs.py inspect [--json]
python3 scripts/official-docs/harmony_docs.py health
```

Default text output is intentionally stable and model-friendly:

```text
QUERY: ...
ESTIMATED_TOTAL_HITS: ...
=== RESULT 1 ===
SOURCE_ID: harmonyos-docs:1
TITLE: ...
URL: ...
PATH: ...
META: ...
CONTENT:
...
```

Not every index document must contain all of these fields. The formatter detects common title/URL/path/content field names and preserves useful scalar metadata. Use `--json` when the exact raw schema matters.

## Environment overrides

- `HARMONYOS_MEILI_URL` — base URL, no trailing slash required.
- `HARMONYOS_MEILI_INDEX` — index UID.
- `HARMONYOS_MEILI_KEY` — API key.
- `HARMONYOS_MEILI_TIMEOUT` — request timeout seconds.

## Direct curl fallback

When Python is unavailable:

```bash
curl -sS 'https://meilisearch.ezer.heiyu.space/indexes/harmonyos-docs/search' \
  -H 'Authorization: Bearer lazycatcloud' \
  -H 'Content-Type: application/json' \
  -d '{"q":"VpnExtensionAbility","limit":5}'
```

Prefer the bundled script because it handles JSON escaping, errors, compact rendering, batching, and secret-safe diagnostics.
