## API Resources

```bash
lark-cli schema im.<resource>.<method>   # 调用 API 前必须先查看参数结构
lark-cli im <resource> <method> [flags] # 调用 API
```

> **重要**：使用原生 API 时，必须先运行 `schema` 查看 `--data` / `--params` 参数结构，不要猜测字段格式。

### chats

  - `create` — 创建群。Identity: `bot` only (`tenant_access_token`).
  - `get` — 获取群信息。Identity: supports `user` and `bot`; the caller must be in the target chat to get full details, and must belong to the same tenant for internal chats.
  - `link` — 获取群分享链接。Identity: supports `user` and `bot`; the caller must be in the target chat, must be an owner or admin when chat sharing is restricted to owners/admins, and must belong to the same tenant for internal chats.
  - `update` — 更新群信息。Identity: supports `user` and `bot`.

### chat.members

  - `create` — 将用户或机器人拉入群聊。Identity: supports `user` and `bot`; the caller must be in the target chat; for `bot` calls, added users must be within the app's availability; for internal chats the operator must belong to the same tenant; if only owners/admins can add members, the caller must be an owner/admin, or a chat-creator bot with `im:chat:operate_as_owner`.
  - `delete` — 将用户或机器人移出群聊。Identity: supports `user` and `bot`; only group owner, admin, or creator bot can remove others; max 50 users or 5 bots per request.

### chat.user_setting

  - `batch_query` — 批量查询当前用户在群内的个人偏好设置 (e.g. `is_muted` mutes normal messages, `is_mute_at_all` mutes @all messages); up to 10 chats per request. Identity: `user` only (`user_access_token`); the caller must be in each target chat.
  - `batch_update` — 批量更新当前用户在群内的个人偏好设置 (e.g. `is_muted` mutes normal messages, `is_mute_at_all` mutes @all messages); up to 10 chats per request. Identity: `user` only (`user_access_token`); the caller must be in each target chat.

### chat.nickname

  - `get` — 获取自己的群昵称。Get your own nickname in the chat (self-only). Identity: `user` only (`user_access_token`); returns an empty string when no nickname is set.
  - `update` — 设置自己的群昵称。Set or update your own nickname in the chat (self-only). Identity: `user` only (`user_access_token`); `nickname` must be a non-empty string (max 300 bytes). Use DELETE to clear it.
  - `delete` — 清空自己的群昵称。Clear your own nickname in the chat (self-only). Identity: `user` only (`user_access_token`).

### chat.managers

  - `add_managers` — 指定群管理员。Identity: supports `user` and `bot`; only the group owner can add managers; max 10 managers per chat (20 for super-large chats), and at most 5 bots per request.
  - `delete_managers` — 删除群管理员。Identity: supports `user` and `bot`; only the group owner can remove managers; max 50 users or 5 bots per request.

### chat.moderation

  - `get` — 获取群成员发言权限。Identity: supports `user` and `bot`; the caller must be in the target chat and belong to the same tenant.
  - `update` — 更新群发言权限。Identity: supports `user` and `bot`; only the group owner (or creator bot with `im:chat:operate_as_owner`) can update; the caller must be in the chat.

### messages

  - `delete` — 撤回消息。Identity: supports `user` and `bot`; for `bot` calls, the bot must be in the chat to revoke group messages; to revoke another user's group message, the bot must be the owner, an admin, or the creator; for user P2P recalls, the target user must be within the bot's availability.
  - `forward` — 转发消息。Identity: supports `user` and `bot`.
  - `merge_forward` — 合并转发消息。Identity: `bot` only (`tenant_access_token`).
  - `read_users` — 查询消息已读信息。Identity: `bot` only (`tenant_access_token`); the bot must be in the chat, and can only query read status for messages it sent within the last 7 days.
  - `urgent_app` — 发送应用内加急。Identity: `bot` only (`tenant_access_token`); the bot must be the message sender and must be in the conversation that contains the message.
  - `urgent_phone` — 发送电话加急。Identity: `bot` only (`tenant_access_token`); the bot must be the message sender and must be in the conversation that contains the message.
  - `urgent_sms` — 发送短信加急。Identity: `bot` only (`tenant_access_token`); the bot must be the message sender and must be in the conversation that contains the message.

### reactions

  - `batch_query` — 批量获取消息表情。Identity: supports `user` and `bot`.[Must-read](lark-im-reactions.md)
  - `create` — 添加消息表情回复。Identity: supports `user` and `bot`; the caller must be in the conversation that contains the message.[Must-read](lark-im-reactions.md)
  - `delete` — 删除消息表情回复。Identity: supports `user` and `bot`; the caller must be in the conversation that contains the message, and can only delete reactions added by itself.[Must-read](lark-im-reactions.md)
  - `list` — 获取消息表情回复。Identity: supports `user` and `bot`; the caller must be in the conversation that contains the message.[Must-read](lark-im-reactions.md)

### threads

  - `forward` — 转发话题。Identity: supports `user` and `bot`.

### images

  - `create` — 上传图片。Identity: `bot` only (`tenant_access_token`).

### pins

  - `create` — Pin 消息。Identity: supports `user` and `bot`.
  - `delete` — 移除 Pin 消息。Identity: supports `user` and `bot`.
  - `list` — 获取群内 Pin 消息。Identity: supports `user` and `bot`.

### feed.groups

  - `batch_add_item` — Batch add feed cards to a feed group. Identity: `user` only (`user_access_token`).[Must-read](lark-im-feed-groups.md)
  - `batch_query` — Batch query feed groups. Identity: `user` only (`user_access_token`).[Must-read](lark-im-feed-groups.md)
  - `batch_remove_item` — Batch remove feed cards from a feed group. Identity: `user` only (`user_access_token`).[Must-read](lark-im-feed-groups.md)
  - `create` — Create a feed group. Identity: `user` only (`user_access_token`).[Must-read](lark-im-feed-groups.md)
  - `delete` — Delete a feed group. Identity: `user` only (`user_access_token`).[Must-read](lark-im-feed-groups.md)
  - `update` — Update a feed group. Identity: `user` only (`user_access_token`).[Must-read](lark-im-feed-groups.md)

## 权限表

| 方法 | 所需 scope |
|------|-----------|
| `chats.create` | `im:chat:create` |
| `chats.get` | `im:chat:read` |
| `chats.link` | `im:chat:read` |
| `chats.update` | `im:chat:update` |
| `chat.members.create` | `im:chat.members:write_only` |
| `chat.members.delete` | `im:chat.members:write_only` |
| `chat.members.get` | `im:chat.members:read` |
| `+chat-members-list` | `im:chat.members:read` |
| `chat.user_setting.batch_query` | `im:chat.user_setting:read` |
| `chat.user_setting.batch_update` | `im:chat.user_setting:write` |
| `chat.managers.add_managers` | `im:chat.managers:write_only` |
| `chat.managers.delete_managers` | `im:chat.managers:write_only` |
| `chat.moderation.get` | `im:chat.moderation:read` |
| `chat.moderation.update` | `im:chat:moderation:write_only` |
| `messages.delete` | `im:message:recall` |
| `messages.forward` | `im:message` |
| `messages.merge_forward` | `im:message` |
| `messages.read_users` | `im:message:readonly` |
| `messages.urgent_app` | `im:message.urgent` |
| `messages.urgent_phone` | `im:message.urgent:phone` |
| `messages.urgent_sms` | `im:message.urgent:sms` |
| `reactions.batch_query` | `im:message.reactions:read` |
| `reactions.create` | `im:message.reactions:write_only` |
| `reactions.delete` | `im:message.reactions:write_only` |
| `reactions.list` | `im:message.reactions:read` |
| `threads.forward` | `im:message` |
| `images.create` | `im:resource` |
| `pins.create` | `im:message.pins:write_only` |
| `pins.delete` | `im:message.pins:write_only` |
| `pins.list` | `im:message.pins:read` |
| `feed.groups.batch_add_item` | `im:feed_group_v1:write` |
| `feed.groups.batch_query` | `im:feed_group_v1:read` |
| `feed.groups.batch_remove_item` | `im:feed_group_v1:write` |
| `feed.groups.create` | `im:feed_group_v1:write` |
| `feed.groups.delete` | `im:feed_group_v1:write` |
| `feed.groups.update` | `im:feed_group_v1:write` |
