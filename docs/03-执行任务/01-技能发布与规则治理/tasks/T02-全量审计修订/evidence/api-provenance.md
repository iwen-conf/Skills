# 外部资料核对

2026-10-05 读取 [Android verified-email 官方实现页](https://developer.android.com/identity/digital-credentials/email-verification-implementation)：文字仍写外层 digital.requests，Android 示例却使用顶层 requests。此处仅证实上游文档内部冲突，未验证任何项目解析版本/provider 的真实请求合同。技能保留示例并显式要求实际版本验证，状态为 NEEDS_TARGET_VALIDATION；服务端认证、非验证数据类型和日志泄露问题已在入口及直接参考中修订。

ArkTS 未随包提供的参考链接依据 [OpenHarmony/docs master 树](https://api.github.com/repos/openharmony/docs/git/trees/master?recursive=1) 的完整响应（truncated=false）解析到官方仓库真实路径；不是固定 API 版本证明。仅将缺失本地链接转成确认存在的上游路径，原知识内容保留。

原生 Android/HarmonyOS 构建、GMS、真实飞书/CNB API 与 Agent 行为未执行。PowerShell/pwsh 不可用，run.ps1 未运行；run.sh 已用独立临时项目/mock 命令验证 cwd、参数和失败传播。
