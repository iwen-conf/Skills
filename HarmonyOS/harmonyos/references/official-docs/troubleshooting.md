# Troubleshooting

## Empty hits

Do not conclude that the API does not exist after one query.

1. Search the exact identifier only.
2. Remove namespace/module prefixes.
3. Search the containing type and member separately.
4. Try the canonical Chinese/English term pair.
5. Search a related permission, config key, or error code.

If still empty, report the searched terms and mark the platform fact unverified.

## HTTP 401 / 403

- Confirm the API key is available through `scripts/official-docs/config/meilisearch.json` (keyless; real key in `~/.config/harmonyos-docs/meilisearch.json`) or `HARMONYOS_MEILI_KEY`.
- Do not print the key while debugging.
- Run `python3 scripts/official-docs/harmony_docs.py health`.

## HTTP 404

Usually indicates a wrong base URL/index UID or a changed Meilisearch route. Check:

```bash
python3 scripts/official-docs/harmony_docs.py health
```

and verify `HARMONYOS_MEILI_URL` / `HARMONYOS_MEILI_INDEX`.

## Timeout / DNS / TLS failure

This is an environment/network problem, not evidence that documentation is missing. Preserve the distinction in the final answer. Retry once if transient; otherwise state the blocker.

## Unexpected result schema

Use raw output:

```bash
python3 scripts/official-docs/harmony_docs.py inspect --json
python3 scripts/official-docs/harmony_docs.py search 'known symbol' --json
```

The default formatter is schema-tolerant, but raw JSON is authoritative.

## Too much text

Reduce result count and per-hit text:

```bash
python3 scripts/official-docs/harmony_docs.py search 'symbol' --limit 3 --max-chars 1800
```

Prefer another targeted query over increasing context volume.
