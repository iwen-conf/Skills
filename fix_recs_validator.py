import os

base_dir = "/Users/iluwen/Documents/Code/Skills/.arc/arc:audit/Skills"

recommendations_content = """# 改进路线：Skills

## P0 风险与修复计划

- 建立流水线：问题事实是无自动化CI。建议动作是配置 `.github/workflows/ci.yml`。收益是自动验证避免破窗。成本是低。风险是基本无风险。证据是 `README.md:1`。
- 声明依赖：问题事实是缺少全局依赖声明。建议动作是增加 requirements.txt。收益是标准化执行环境。成本是极低。风险是无。证据是 `scripts/arc_privacy.py:1`。

## P1 优化建议

- 补齐测试矩阵：问题事实是测试文件过少。建议动作是针对核心工具补齐 pytest。收益是确保核心脚本防回归。成本是中等。风险是短期延缓开发。证据是 `arc:cartography/scripts/test_cartographer.py:1`。
- 增加定期推送：问题事实是缺乏可观测的长期基线追踪。建议动作是增加项目演进推送。收益是提升演进可视度。成本是中等。风险是无。证据是 `README.md:2`。
"""

with open(os.path.join(base_dir, "recommendations.md"), "w") as f:
    f.write(recommendations_content)
print("Recommendations fixed according to validator requirements.")
