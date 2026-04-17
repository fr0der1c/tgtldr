# TGTLDR v1 产品流程与实施计划

本文档补充 [ARCHITECTURE.md](/Users/frederic/Documents/Projects/tgtldr/docs/ARCHITECTURE.md)，用于定义 v1 的向导流程、后台页面和实施顺序。

## 1. 首次启动向导

首次启动建议做成顺序式向导。

### 第一步：Telegram 应用配置

用户填写：

- `api_id`
- `api_hash`

页面说明需要先在 Telegram 官方页面申请应用。

### 第二步：Telegram 用户登录

用户填写手机号，系统进入认证流程。

前端只负责展示表单和状态，真正的 MTProto 登录逻辑由后端执行。

需要支持的交互状态：

- 输入手机号
- 输入验证码
- 输入 2FA 密码
- 登录成功
- 登录失败并重试

说明：

- 不应把“短信验证码”写死为唯一方式
- Telegram 可能使用 App 内通知、邮箱验证码或其他认证方式

### 第三步：选择监听群组

登录成功后：

- 后端拉取用户已加入的群组/超级群组
- 前端提供搜索、勾选、启停操作

用户配置：

- 哪些群启用监听

### 第四步：AI 配置

用户填写：

- OpenAI Base URL
- API Key
- Model
- Temperature
- Max Output Tokens

建议提供“测试连接”按钮。

### 第五步：摘要策略

针对每个启用的群组设置：

- 每日摘要 Prompt
- 每日摘要时间
- 摘要时区

默认值可以来自系统级配置，再允许群组级覆盖。

### 第六步：交付方式

用户选择：

- 仅平台内查看摘要
- 同时通过 Telegram Bot 推送

如果选择 Bot 推送，再显示：

- `Bot Token`
- 推送测试
- 接收目标绑定说明

这一步必须是可选的，且不应阻塞主流程。

## 2. 后台管理界面

v1 后台建议包含四个主页面。

### 2.1 概览页

展示：

- Telegram 连接状态
- 已监听群数量
- 最近一次摘要生成时间
- 最近失败任务

### 2.2 群组管理

支持：

- 查看群组列表
- 启用/停用监听
- 修改单群 Prompt
- 修改单群摘要时间与时区

### 2.3 AI 与系统配置

支持：

- 配置 OpenAI 兼容接口
- 配置默认模型参数
- 可选启用 Bot 发送

### 2.4 摘要管理

支持：

- 查看摘要列表
- 按群、日期筛选
- 查看单条摘要详情
- 手动触发重新生成

## 3. 目录结构建议

建议初始化为如下结构：

```text
tgtldr/
  docs/
    ARCHITECTURE.md
    PRODUCT_FLOW.md
  app/
    cmd/
      server/
    internal/
      bot/
      config/
      openai/
      scheduler/
      store/
      summary/
      telegram/
  web/
    app/
    components/
    lib/
  deploy/
    docker/
  scripts/
```

说明：

- `app` 与 `web` 分离，便于独立构建
- `docs` 存架构与实施说明
- `deploy` 存 Docker 相关内容

## 4. 迭代计划

### 阶段 1：项目初始化

- 建立 Go 服务骨架
- 建立 Next.js 前端骨架
- 建立 PostgreSQL migration
- 建立 Docker Compose

### 阶段 2：登录与群组接入

- Telegram 应用配置
- 用户登录向导
- 群组列表拉取
- 启用监听配置

### 阶段 3：消息监听与落库

- updates 常驻监听
- 消息持久化
- 断线恢复

### 阶段 4：摘要能力

- AI 配置
- 每群独立摘要策略
- 自动摘要
- 手动触发摘要
- 摘要详情页

### 阶段 5：可选 Bot 推送

- Bot Token 配置
- 推送测试
- 摘要投递

## 5. 当前结论

TGTLDR v1 应坚持以下原则：

- 先做单用户自部署
- 先跑通登录、监听、摘要、展示的主链路
- Bot 推送是可选能力，不阻塞主流程
- 数据模型控制在 5 张主表，避免过度设计
- 为图片理解预留结构，但不在 v1 实现

后续所有实现工作，均以本方案作为边界基准。
