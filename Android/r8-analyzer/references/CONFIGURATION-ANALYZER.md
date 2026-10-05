# R8 quantitative analysis

Requirements: R8 >= 9.3.7-dev, Python and protobuf >= 6.33.4 compatible with the bundled binding. A missing tool/schema is BLOCKED, not a reason to output a quantitative score.

1. Create a unique temporary output directory outside the project. Keep prior runs unchanged; do not glob-delete historical artifacts.
2. In the project cwd, execute the actual release variant with `-Dcom.android.tools.r8.dumpkeepradiustodirectory="$R8_RUN_DIR"`. Example: `./gradlew assembleRelease` plus that property. Select the exact newly produced `.pb` for the intended variant; multiple outputs require explicit selection, never a lexicographically “latest” guess.
3. `SKILL_ROOT` is the loaded r8-analyzer directory. Run the [analyzer](../scripts/analyze.py):

```bash
python3 "$SKILL_ROOT/scripts/analyze.py" --proto "$R8_PROTO" --out "$R8_RUN_DIR/analysis_result.txt"
```

For already converted JSON from the same build, use `--json "$R8_JSON"` instead of `--proto`. The [binding](../scripts/keep_radius_pb2.py) is version-specific; unknown protobuf fields or unrecognized constraint enums fail closed; JSON mode validates the metric fields it consumes and ignores auxiliary metadata. No stale result reuse after a failed command.

The script validates required tables, positive live counts, constraint/rule references and metric bounds, then writes scores and rule-impact details to **analysis_result.txt**. No missing metric defaults to zero. Retain the input, output and build/variant identity for reproducibility. Compare historical results only for comparable variants/configurations; no automatic deletion on failure.

Scores measure constraints on reported live items, not APK byte reduction or runtime safety. Reflection/serialization/native entry-point validation remains mandatory before any rule removal. Heuristic analysis remains separately labelled and has no fabricated quantitative scores.
