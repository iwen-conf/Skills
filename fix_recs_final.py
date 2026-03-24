import os

base_dir = "/Users/iluwen/Documents/Code/Skills/.arc/audit/Skills"

recommendations_content = """# 改进路线：Skills

## 建议清单

1. 建立流水线：问题事实是无 CI。建议动作是配置 CI。收益是自动验证。成本是低。风险是无。证据是 `README.md:1`。
2. 声明依赖：问题事实是无全局依赖。建议动作是增加 requirements.txt。收益是标准化。成本是低。风险是无。证据是 `scripts/arc_privacy.py:1`。
3. 补齐测试：问题事实是无测试。建议动作是补齐 pytest。收益是防回归。成本是中。风险是低。证据是 `arc-cartography/scripts/test_cartographer.py:1`。
4. 增加看板：问题事实是无通知。建议动作是建立定期推送。收益是可视度高。成本是中。风险是无。证据是 `README.md:2`。
5. 统一日志：问题事实是输出随意。建议动作是统一格式。收益是好排查。成本是低。风险是无。证据是 `scripts/arc_privacy.py:2`。
"""

with open(os.path.join(base_dir, "recommendations.md"), "w") as f: f.write(recommendations_content)
