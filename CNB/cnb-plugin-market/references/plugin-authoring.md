## 插件 README 编写建议

可控插件建议在 README.md 中包含以下内容，以便 `cnb-plugin-market` 精确提取配置信息：

### 参数表（核心）

```markdown
## 参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| webhook | string | 是 | - | 企业微信群机器人 Webhook 地址 |
| msg_type | string | 否 | text | 消息类型：text / markdown |
| content | string | 是 | - | 消息内容 |
```

### 配置示例（必须有）

```yaml
# 密钥仓库文件 secret-env.yml（存放敏感信息，声明变量名）
WECOM_WEBHOOK: https://qyapi.weixin.qq.com/cgi-bin/webhook?key=xxx

# .cnb.yml 中使用
main:
  push:
    - stages:
        - name: 通知企业微信群
          imports: https://cnb.cool/<org>/<secret-repo>/-/blob/main/secret-env.yml
          image: tencentcom/wecom-message    # 默认 latest；可加 :<版本> 固定版本
          settings:
            webhook: $WECOM_WEBHOOK           # WECOM_WEBHOOK 是 secret-env.yml 中声明的变量名
            msg_type: markdown
            content: "## 构建完成\n状态：**成功**"
```

### 要点

- **参数表五项齐全**：便于本技能直接解析并生成对应 `settings` 配置
- **配置示例可直接使用**：环境变量引用用 `$VAR` 占位（`VAR` 为密钥仓库文件中声明的真实变量名），敏感信息标注放密钥仓库
- 镜像遵循项目版本策略，长期 CI 固定经验证版本
