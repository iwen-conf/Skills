# Local Code Index

This directory contains local-only helpers for code search in this repository.

```bash
.ai-code-index/reindex.sh
.ai-code-index/search.sh "query"
.ai-code-index/struct-search.sh python 'def $NAME($$$): $$$'
.ai-code-index/symbols.sh
```

`reindex.sh` writes Zoekt shards under `${AI_CODE_INDEX_DIR:-~/.cache/ai-code-index/zoekt}/Skills`.
No remote indexing, memory service, or paid storage is used.
