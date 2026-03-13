# Visual Regression Plan

## Scope
- Critical pages: `<list>`
- Viewports: 1920x1080, 1366x768, 390x844
- Themes/locales: `<list>`

## Baseline Strategy
- When to capture baseline: after a stable release passes E2E gate
- Storage: `baselines/{{RUN_ID}}/` (committed to Git)
- Review process: compare output reviewed by Executor, accepted by Reviewer

### Initialize Baselines
```bash
python3 Arc/arc:e2e/scripts/baseline_manager.py init \
  --run-dir {{REPORT_OUTPUT_DIR}}/{{RUN_ID}} \
  --baseline-dir baselines/<project>
```

### Run Comparison
```bash
python3 Arc/arc:e2e/scripts/baseline_manager.py compare \
  --run-dir {{REPORT_OUTPUT_DIR}}/{{RUN_ID}} \
  --baseline-dir baselines/<project> \
  --threshold 0.95 \
  --output-dir {{REPORT_OUTPUT_DIR}}/{{RUN_ID}}/visual-diffs \
  --fail-on-diff
```

### Single-file Diff (ad hoc)
```bash
python3 Arc/arc:e2e/scripts/visual_diff.py \
  --baseline baselines/<project>/s0001_login.png \
  --current {{REPORT_OUTPUT_DIR}}/{{RUN_ID}}/screenshots/s0001_login.png \
  --output visual-diffs/s0001_login.diff.png \
  --threshold 0.95 \
  --json --fail-on-diff
```

### Accept Changes as New Baselines
```bash
python3 Arc/arc:e2e/scripts/baseline_manager.py update \
  --run-dir {{REPORT_OUTPUT_DIR}}/{{RUN_ID}} \
  --baseline-dir baselines/<project> \
  --files s0001_login.png,s0003_dashboard.png \
  --reason "New sidebar layout"
```

## Diff Triage
- Ignore: dynamic timestamps, avatars, randomized content
  - Use `--mask-regions "x,y,w,h;..."` to exclude these areas
- Investigate: layout shifts, missing elements, clipped text, contrast changes

## Evidence
- Baseline screenshots: `baselines/<project>/`
- Baseline manifest: `baselines/<project>/baseline-manifest.json`
- Diff reports: `visual-diffs/visual-diff-summary.json`
- Diff images: `visual-diffs/*.diff.png`

## Quality Gate Integration
```bash
python3 Arc/arc:e2e/scripts/check_artifacts.py \
  --run-dir {{REPORT_OUTPUT_DIR}}/{{RUN_ID}} \
  --check-baselines --strict
```
