## 业务闭环铁律（arc:plan model）

- 业务生命周期模型登记在 `docs/01-系统设计/业务闭环/03-closure-spec.yaml`；需求、API 合同和执行进度沿用项目现有来源，不新建重复账本。
- 只有影响该模型的业务改动才先更新 spec；注释、纯格式等无行为变化不强制生成模型。
- 编码前定位当前安装的 closure 检查器，核对项目副本 SHA256 与它一致；不同则使用当前安装脚本，
  或显式更新项目副本并重新 check。不能用旧副本与旧状态自洽来绕过检查器升级。
  副本一致后运行 `python3 docs/01-系统设计/业务闭环/tools/check_closure.py gate docs/01-系统设计/业务闭环/03-closure-spec.yaml`。
  模型闸门须零未豁免 ERROR/WARN、完整未截断检查，且 spec/检查器指纹一致；变更后重新 check。
- gate 通过后按本会话已有明确实现授权和范围交 `arc:code`，不重复索取授权；仅设计/只读任务不自动进入实现。
- 未决业务假设、豁免及范围决策必须先解决；既有确认可复用，spec 变更若改变决策才需重新确认。
- spec PASS 仅为模型证据；实际完成需要项目门禁及目标面行为，由 `arc:verify` 验证，发现回传 `arc:audit` / `arc:code`。
- 只读闭环审查仅写 `.arc/artifacts/<task-slug>/closure/`；禁止初始化设计文档、复制工具或追加常驻规则。
