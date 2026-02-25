# 审查评分模型

## 评分维度

- `copyright_readiness`：0-100
- `patent_readiness`：0-100
- `ownership_risk`：LOW/MEDIUM/HIGH
- `rejection_risk`：LOW/MEDIUM/HIGH

## 评分建议阈值

- `>=80`：建议立即推进
- `60-79`：补充后推进
- `<60`：先修复问题再申请

## 推荐结论映射

- 软著高 + 专利高：并行推进
- 软著高 + 专利中低：软著先行，专利补强
- 软著中低 + 专利高：先修正权属与文档，再推进专利
- 双低：先做技术/资产梳理，不建议直接提交
