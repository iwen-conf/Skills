import os

base_dir = "/Users/iluwen/Documents/Code/Skills/.arc/audit/Skills"

recommendations_content = """# 改进路线：Skills

## P0 风险与修复计划

### 1. 建立 CI/CD 流水线
建议动作：补齐仓库级 CI 配置以实现流水线。问题事实：没有自动化流水线。收益：自动验证 `validate_skills.py` 从而避免破窗。成本：极低。风险：基本无风险，仅增加 CI 运行时间。证据：`README.md:1`。

## P1 优化建议

### 1. 补齐自动化测试矩阵
建议动作：针对核心工具补齐 `pytest`。问题事实：测试文件过少。收益：确保编排脚本不会回归。成本：中等，需要开发人员投入时间。风险：可能短期内延缓业务特性开发进度。证据：`arc:cartography/scripts/test_cartographer.py:1`。
"""

with open(os.path.join(base_dir, "recommendations.md"), "w") as f: f.write(recommendations_content)
print("Recommendations fixed.")
