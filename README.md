# TGTLDR

TGTLDR 是一个单用户自部署的 Telegram 群消息监听与每日 AI 摘要系统。

## 当前实现

- Go 后端
  - 配置管理
  - Telegram 登录流程 API
  - 群组同步
  - 消息落库
  - 每日摘要调度
  - 可选 Bot 推送
- Next.js 前端
  - 首次配置向导
  - 概览页
  - 群组管理
  - 摘要查看与手动触发
  - 系统设置
- PostgreSQL
  - 5 张核心表
  - 启动时自动 migration

## 本地启动

### 使用 Docker

```bash
cp .env.example .env
docker compose up --build
```

启动后：

- 前端：http://localhost:3000
- 后端 API：http://localhost:8080
- PostgreSQL（宿主机映射）：`127.0.0.1:15432`

说明：

- 所有端口默认只绑定到 `127.0.0.1`
- 宿主机 `5432` 已预留给本机已有数据库，compose 使用 `15432`

### 手动启动后端

```bash
cd app
export TGTLDR_DATABASE_URL='postgres://postgres:postgres@localhost:5432/tgtldr?sslmode=disable'
export TGTLDR_MASTER_KEY='MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY='
go run ./cmd/server
```

### 手动启动前端

```bash
cd web
npm install
NEXT_PUBLIC_API_BASE_URL=http://localhost:8080 npm run dev
```

## 首次配置顺序

1. 在 `/setup` 页面填写 Telegram `api_id` / `api_hash`
2. 填写 OpenAI 兼容接口参数
3. 保存基础配置
4. 输入 Telegram 手机号并完成验证码/2FA 登录
5. 同步群组列表
6. 在后台为群组设置监听、摘要时间、Prompt 和交付方式
7. 如果需要 Bot 推送，再配置 `Bot Token` 和目标 `chat_id`

## 文档

- [架构方案](docs/ARCHITECTURE.md)
- [产品流程与实施计划](docs/PRODUCT_FLOW.md)
