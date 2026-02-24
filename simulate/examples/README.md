# Examples

> 这些示例用于展示 `simulate/scripts/` 的典型用法与产物结构。

## 1) 生成 Run 目录（含“测试公司级”全流程模版）

```bash
python scripts/scaffold_run.py \
  --pack full-process \
  --objective "验证从普通用户提交采购申请到经理审批通过的完整流程" \
  --target-url "http://localhost:5173" \
  --personas examples/personas.sample.json \
  --formats markdown,jsonl
```

脚本会输出一个目录路径（例如 `reports/2026-02-01_14-30-00_ab12cd/`）。

## 2) 从 events.jsonl 编译报告

将 `events.jsonl` 放入 run 目录后：

```bash
python scripts/compile_report.py --run-dir reports/<run_id> --in-place
```

输出：
- `action-log.compiled.md`
- `screenshot-manifest.compiled.md`
- 更新 `report.md` 的步骤表（需要模版内 marker）

可选（报告美化/列宽控制）：

```bash
python scripts/compile_report.py \
  --run-dir reports/<run_id> \
  --table-backend tabulate \
  --steps-widths "Action=120,Expected=80,Actual=80,Evidence=80" \
  --manifest-widths "Path=80,URL=80,Description=80,Expectation=80" \
  --beautify-md
```

或对现有 Markdown 一键格式化：

```bash
python scripts/beautify_md.py --run-dir reports/<run_id>
```

## 3) 生成缺陷/失败单

```bash
python scripts/new_defect.py \
  --run-dir reports/<run_id> \
  --step 0007 \
  --title "提交后出现权限不足弹窗" \
  --role buyer \
  --url "http://localhost:5173/requests" \
  --user buyer_01 \
  --password secret123 \
  --screenshot screenshots/s0007_after-submit.png
```

## 4) 质量门禁检查

```bash
python scripts/check_artifacts.py --run-dir reports/<run_id>
```
