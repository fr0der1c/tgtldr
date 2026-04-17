# TGTLDR v1 架构方案

## 1. 项目目标

TGTLDR 是一个单用户自部署系统，用于：

- 以 Telegram User Bot 方式登录用户账号
- 实时监听用户选定的群组消息并落库
- 按群组配置，在每日固定时间对前一天消息进行 AI 总结
- 在后台展示摘要结果
- 可选地通过 Telegram Bot 将摘要发送给用户

本阶段只定义 v1 方案，目标是优先跑通完整主链路，而不是一次性覆盖所有扩展能力。

## 2. 范围边界

### 2.1 v1 必做

- 单用户自部署
- Docker 启动
- Web 首次启动向导
- Telegram 用户登录
- 群组列表展示与监听开关
- 文本消息、链接、回复关系、caption 落库
- 每个群独立配置摘要 Prompt、摘要时间、时区
- OpenAI 兼容接口配置
- 每日自动摘要
- 后台查看历史摘要
- 手动触发某个群某天的摘要生成
- 可选 Bot 推送

### 2.2 v1 明确不做

- 多用户/多租户
- 语音识别与转写
- 图片理解
- 视频理解
- 复杂权限系统
- 多 AI 提供商并存管理
- 完整任务编排后台

### 2.3 v1 预留但不实现

- 图片理解能力
- 媒体文件分析流水线
- 更复杂的摘要审计和 chunk 级回放界面

## 3. 关键产品决策

### 3.1 单用户模型

系统只服务一个部署者，不做账号体系，不做组织、多空间、成员协作。

这意味着：

- 大部分全局配置可以直接放在系统级配置中
- Telegram 登录态只维护一个主账号
- 不需要额外设计租户隔离和权限模型

### 3.2 Bot 推送是可选能力

默认交付方式是“后台查看摘要”。

用户在首次配置的最后一步选择：

- 仅在平台内查看
- 同时通过 Telegram Bot 推送

如果用户不启用 Bot，系统仍然是完整可用的。

### 3.3 图片能力先预留

v1 不做图片理解，但消息存储和处理链路需要保留扩展空间：

- 记录消息类型
- 记录媒体类型
- 保留 caption
- 保留 Telegram 原始载荷或可恢复的媒体引用

这样后续可以在不破坏现有表结构的前提下补上 OCR、图像理解等能力。

## 4. 技术选型

## 4.1 后端

- Go
- Telegram 客户端库：`github.com/gotd/td`
- HTTP API：标准库 `net/http` 或轻量路由
- 数据库：PostgreSQL

选择 `gotd/td` 的原因：

- 支持 Telegram 用户认证流程
- 支持会话持久化
- 支持更新流处理与断线恢复
- 更适合长期在线监听消息

### 4.2 前端

- Next.js
- Tailwind CSS
- `shadcn/ui`

选择理由：

- 适合管理后台和向导流程
- 组件可定制，不会被模板锁死
- 自部署和 Docker 化成熟

### 4.3 部署

- `docker compose`

建议容器：

- `app`：Go 服务
- `web`：Next.js 管理界面
- `postgres`：业务数据库

在 v1 阶段不强制拆分更多基础设施。

## 5. 系统总体架构

```text
+-------------+        +-------------+
|   Browser   | <----> |  Web (UI)   |
+-------------+        +-------------+
                               |
                               v
                        +-------------+
                        |   Go API    |
                        +-------------+
                          |    |    |
                          |    |    +--------------------+
                          |    |                         |
                          v    v                         v
                    +---------+---------+        +-------------+
                    | Telegram Runtime  |        |  Scheduler  |
                    | gotd/td listener  |        |  Summaries  |
                    +---------+---------+        +-------------+
                              |                         |
                              +-----------+-------------+
                                          |
                                          v
                                     +---------+
                                     |Postgres |
                                     +---------+
                                          |
                                          v
                                     +---------+
                                     | Bot API |
                                     +---------+
```

说明：

- `web` 负责向导和后台页面
- `Go API` 负责配置读写、登录流程接口、手动触发摘要等
- `Telegram Runtime` 负责用户会话、更新流监听、消息入库
- `Scheduler` 负责每日摘要生成和可选 Bot 投递

## 6. 核心模块拆分

建议 Go 服务内部按模块拆分：

- `internal/config`
  - 全局配置加载
  - 密钥处理
- `internal/telegram`
  - 用户登录
  - session 持久化
  - 群组列表拉取
  - updates 监听
- `internal/store`
  - PostgreSQL 访问
- `internal/summary`
  - 摘要构建
  - 分块逻辑
  - 聚合逻辑
- `internal/openai`
  - OpenAI 兼容接口封装
- `internal/bot`
  - Bot 发送能力
- `internal/scheduler`
  - 定时扫描与任务执行

## 7. 补充文档

以下内容已拆分到补充文档，便于后续维护：

- 首次启动向导
- 后台管理界面
- 目录结构建议
- 实施阶段计划

参见 [PRODUCT_FLOW.md](/Users/frederic/Documents/Projects/tgtldr/docs/PRODUCT_FLOW.md)。

## 8. 精简后的数据模型

v1 先收敛到 5 张主表。

## 8.1 `app_settings`

用途：保存系统级配置。

建议字段：

- `id`
- `telegram_api_id`
- `telegram_api_hash`
- `openai_base_url`
- `openai_api_key`
- `openai_model`
- `openai_temperature`
- `openai_max_output_tokens`
- `bot_enabled`
- `bot_token`
- `created_at`
- `updated_at`

说明：

- 单用户系统下，不需要拆独立“AI 提供商表”
- 敏感字段需要加密存储

## 8.2 `telegram_auth`

用途：保存 Telegram 用户登录态。

建议字段：

- `id`
- `phone_number`
- `telegram_user_id`
- `telegram_username`
- `session_data`
- `status`
- `last_connected_at`
- `created_at`
- `updated_at`

说明：

- 单用户模式通常只保留一条记录
- `session_data` 需要加密存储

## 8.3 `chats`

用途：保存群组信息和群级配置。

建议字段：

- `id`
- `telegram_chat_id`
- `telegram_access_hash`
- `title`
- `username`
- `chat_type`
- `enabled`
- `summary_prompt`
- `summary_time_local`
- `summary_timezone`
- `delivery_mode`
- `created_at`
- `updated_at`

说明：

- `delivery_mode` 可取 `dashboard` 或 `bot`
- 群级配置直接并入此表，避免单独拆策略表

## 8.4 `messages`

用途：保存监听到的消息。

建议字段：

- `id`
- `chat_id`
- `telegram_message_id`
- `telegram_sender_id`
- `sender_name`
- `text_content`
- `caption`
- `message_type`
- `media_kind`
- `reply_to_message_id`
- `message_time`
- `raw_json`
- `created_at`

说明：

- `raw_json` 用于保留扩展空间
- `message_time` 统一用 UTC 存储
- 唯一键建议为 `chat_id + telegram_message_id`

### 8.5 `summaries`

用途：保存群组每日摘要。

建议字段：

- `id`
- `chat_id`
- `summary_date`
- `status`
- `content`
- `model`
- `source_message_count`
- `chunk_count`
- `generated_at`
- `error_message`
- `created_at`
- `updated_at`

说明：

- 一条记录表示“某个群某一天”的最终摘要
- 唯一键建议为 `chat_id + summary_date`

## 9. 消息监听设计

### 10.1 监听原则

- 只监听用户启用的群
- 只处理与摘要相关的消息内容
- 运行期需要具备断线恢复能力

### 10.2 入库内容

v1 入库重点：

- 文本消息
- 链接
- 回复关系
- caption
- 基本发送者信息
- 消息时间

### 10.3 暂不处理

- 语音转写
- 图片 OCR
- 视频分析

但要保留未来接入这些能力所需的原始数据。

## 10. 摘要生成策略

## 10.1 调度规则

每个群独立配置：

- 摘要时间
- 时区
- Prompt

系统按群时区计算“前一天”的边界，而不是按服务器本地日期硬切。

例如：

- 群配置时区为 `Asia/Shanghai`
- 每天 `09:00` 生成摘要
- 则汇总区间应为该时区下前一天的 `00:00:00` 到 `24:00:00`

## 10.2 摘要输入预处理

将消息整理为适合模型阅读的 transcript：

- 去掉明显无意义的系统噪音
- 保留发送者标识
- 保留时间顺序
- 尽量保留 reply 关系
- 合理拼接连续短消息

## 10.3 超长上下文拆分

如果某日某群消息过多，超过模型上下文预算，使用 map-reduce 风格摘要：

1. 将原始消息按时间和规模切分成多个 chunk
2. 对每个 chunk 单独调用模型，生成结构化阶段摘要
3. 将多个阶段摘要再汇总成最终摘要
4. 如果阶段摘要总量仍然过大，则继续递归聚合

chunk 切分原则：

- 优先按时间连续性切
- 尽量避免打断回复链
- 不只按字数硬切

## 10.4 输出结构

建议最终摘要包含：

- 今日核心话题
- 关键结论
- 待办事项
- 值得关注的链接
- 未解决问题

这样既适合后台查看，也适合后续 Bot 推送。

## 11. Bot 推送设计

Bot 推送在 v1 中是增强项，不应成为前置依赖。

推荐规则：

- 用户默认只在平台内查看摘要
- 只有开启 Bot 推送后，系统才校验 Bot 配置
- 如果 Bot 配置失败，不影响摘要生成本身

接收侧建议最终使用稳定标识，而不是只依赖用户名。

原因：

- Telegram Bot 实际发送更适合使用已建立私聊的目标标识
- 用户名可能变化，也不能保证总是可用

v1 可以先保守设计：

- 配置 `Bot Token`
- 提供接收绑定说明
- 发送测试消息

具体绑定细节可在实现阶段进一步细化。

## 12. 安全设计

- Telegram session 必须加密存储
- OpenAI API Key 必须加密存储
- 前端不直接接触 Telegram MTProto 凭据
- 所有敏感操作通过后端完成
- 日志中禁止输出完整密钥、验证码、session 数据

## 13. Docker 部署方案

推荐使用 `docker compose`。

基础服务：

- `postgres`
- `app`
- `web`

建议：

- 数据库使用独立卷
- 应用和前端分容器构建
- 通过环境变量注入主密钥和数据库连接配置
