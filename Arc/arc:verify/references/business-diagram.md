# Verify business diagrams with arch-diagram

Use the [arch-diagram plugin](https://github.com/bassnova42/arch-diagram-skill) for graph/flow review
visuals. The Arc plugin bundles a pinned, licensed upstream copy. Resolve its actual location with
`arc-policy skills --name arch-diagram`; read that `SKILL.md` and `references/spec.md` only for
this drawing substep. Carry the selected snapshot, stable IDs, package evidence and scope; release
unrelated execution references. Other installations use the host inventory, never a guessed cache path.
Repository discovery still follows [code-intel](../../_shared/code-intel.md), using `arc-idx`.

## Language and business package names

- Set `lang: "zh-CN"`. Titles, business descriptions, conditions, states, edge labels, legends and
  status explanations are Chinese; keep code identifiers, paths and canonical verdict codes intact.
- Business node titles use `中文业务名【实际英文业务包名】`. Take the package identifier from the
  target's actual directory/package declaration or manifest, with traced `file:line` evidence.
  Do not translate Chinese into an invented English module name or substitute an API/table name.
- For a capability spanning packages, list the actual mapped packages; keep their declared spelling
  and namespace. An expected capability with no located implementation uses `中文业务名【业务包待核实】`
  and retains its candidate/gap status. Package mapping never defines the expected business tree.

## Source and evidence boundaries

- For DB graphs, render the model read back after import, recording graph ID/version and source in
  the spec/HTML. Retain the target-local DB and the CLI's HTML+JSON snapshot; keep diagram files in
  a separate artifact directory. Refresh from the current model when it changes.
- For local flows, render the existing flow catalog and retain its Mermaid blocks and branch/
  terminal tables for `flow_ledger.py check`. The plugin's Mermaid export can drop raw-point edges
  and does not replace this ledger. Keep all canonical graph/FLOW/RULE/node/branch IDs and meanings;
  if renderer anchor IDs differ, record their source-ID mapping in the spec.
- Keep the view business-oriented: expected capabilities → actions → conditions/states/recovery.
  Split large graphs into linked bounded views with the same source IDs; report omitted scope.
  Do not reduce the business graph to a directory/package architecture chart.
- The HTML is a self-contained read-only view, with no server or execute/approve/retry controls.
  Omit synthetic live trace logs; animated wires illustrate relationships, never real execution.
  Candidates remain unverified. Display existing verdicts only with their run/source identity;
  a clean diagram check never supplies RAW, an oracle, approval or a business PASS.

## Build and deliver

For an existing v2 DB snapshot, use the plugin's file-only bridge:

```bash
arc-policy --cwd "$TARGET_REPO" diagram --snapshot "$VERIFY_SNAPSHOT" \
  --presentation "$VERIFY_PRESENTATION" --view flows --out "$VERIFY_DIAGRAM_DIR"
```

The input is the original `snapshot.json`, never a freshly invented model. `--view capabilities`
draws the capability tree. The optional presentation JSON supplies Chinese names and package
evidence, not rules or verdicts. It is bound to the snapshot's `graph` and `graph_sha256`.
Resolve names from the user's business vocabulary. A package requires a readable source `file`,
one-based `line` and its actual identifier on that line; otherwise show `业务包待核实`. Select
`node_ids` for graphs over 16 nodes and report omitted nodes/edges. No hidden truncation.

Presentation shape (replace IDs/hash with the actual snapshot; `name_zh` excludes package brackets):

```json
{
  "graph": "GRAPH-ID",
  "graph_sha256": "<meta.graph_sha256>",
  "nodes": {
    "ACTION-ID": {"name_zh": "提交报名", "packages": []}
  }
}
```

Use `packages: [{"name": "actual.package", "file": "relative/manifest", "line": 1}]` only with
actual directory/package/manifest evidence. Empty packages keep the explicit pending label.

The bridge checks upstream hashes, builds the spec and HTML, then runs the official layout check.
Its build/check outcomes are separate from business verdicts recorded at snapshot export time;
offline rendering cannot revalidate today's DB, source or environment. Keep failed-check output
for diagnosis. `--no-check` is for drafting and records layout `NOT_RUN`; never call it visual QA.

Keep `<slug>.spec.json` beside `<slug>.html` in the project's existing graph/flow artifact location
(default `.arc/verify/diagrams/<graph-id>/` for DB graphs). Use the resolved installed directory:

```bash
node "$ARCH_DIAGRAM_SKILL/scripts/build.mjs" "$VERIFY_DIAGRAM_SPEC" "$VERIFY_DIAGRAM_HTML"
node "$ARCH_DIAGRAM_SKILL/scripts/check.mjs" "$VERIFY_DIAGRAM_HTML" "$VERIFY_DIAGRAM_SCREENSHOT"
```

Inspect the screenshot after the check passes; fix clipped Chinese/package names, overlaps and
crossing labels. Return spec/HTML links and the canonical model/ledger paths. PNG/video/GIF exports
are optional when requested. Missing plugin/browser or a failed render/check makes visualization
`BLOCKED` with its exact cause; continue independent model/test work without claiming a substitute
picture is the requested diagram or a successful DB export. Node builds HTML without a browser;
layout/PNG need Chrome, Chromium or Edge (`CHROME=/actual/path`). No browser download occurs in hooks.
