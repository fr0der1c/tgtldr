# TGTLDR

[English version](README.en.md) | [更新日志](CHANGELOG.md) | [Changelog](CHANGELOG.en.md)

TGTLDR （Telegram Too Long, Don't Read）是一个单用户自部署的 Telegram 群消息监听与每日 AI 摘要系统。

这个项目被构建出来的原因是：许多 Telegram 群聊都是超级大群，每天会产生数千条消息。有时我们只想了解一些最新的情报，而并不希望花大量的时间在水群上。使用这个工具，就能为你在每天的固定时间推送前一天的最新群聊结论。

![TGTLDR 首页截图](docs/images/home-zh.png)

## 功能特性

- 监听已加入的 Telegram 群组消息，并保存到本地数据库
- 按群组配置每日摘要时间、Prompt、过滤规则和摘要模型
- 使用 OpenAI 兼容接口生成群聊摘要
- 支持在网页端查看摘要，也可以选择通过 Telegram Bot 推送
- 支持把参与推送的多个群组合并成一篇“每日总览”，并保留来源摘要、遗漏群组和无消息群组记录
- 支持按日期范围和群组手动生成“快速回顾”
- 支持手动触发摘要、查看历史摘要和重新投递失败的 Bot 推送
- 提供首次配置向导，启动后可在网页端完成 Telegram、OpenAI 和群组设置

## 使用前准备

- Docker 和 Docker Compose（推荐启动方式）
- Telegram `api_id` 和 `api_hash`，可在 [my.telegram.org/apps](https://my.telegram.org/apps) 申请
- OpenAI 兼容接口的 Base URL、API Key 和模型名
- 可选：Telegram Bot Token，用于把摘要推送回 Telegram

## 本地启动

### 推荐：使用预构建镜像启动（同时启动前端、后端和数据库）

```bash
cp .env.example .env
docker compose up -d
```

如果你没有显式设置 `TGTLDR_MASTER_KEY`，系统会在首次启动时自动生成一把随机主密钥，并把它持久化到 app 容器的数据卷中。

如果你想拉取指定版本的镜像，可以在启动前设置：

```bash
export TGTLDR_IMAGE_NAMESPACE=fr0der1c
export TGTLDR_IMAGE_TAG=latest
docker compose up -d
```

如果宿主机的 `3000` 端口已被占用，或者你希望监听所有网卡而不是仅监听本机，可以在 `.env` 中覆盖：

```bash
cp .env.example .env
# 编辑 .env，将下面这些项改成你想使用的值：
# TGTLDR_HOST_BIND=0.0.0.0
# TGTLDR_HOST_WEB_PORT=13000
docker compose up -d
```

其中：

- `TGTLDR_HOST_BIND=127.0.0.1` 表示只监听本机，适合默认本地使用
- `TGTLDR_HOST_BIND=0.0.0.0` 表示监听所有网卡，适合部署到服务器或 NAS

`TGTLDR_MASTER_KEY` 是本地数据加密主密钥，用来加密保存 Telegram 登录 session、OpenAI API Key 和 Bot Token。它不会发送给外部服务。默认情况下，这把 key 会保存在 app 数据卷中的 `/var/lib/tgtldr/master.key`；如果你删除了这个数据卷，已经保存的这些敏感数据将无法解密。

启动后访问：

- 前端：`http://localhost:${TGTLDR_HOST_WEB_PORT}`（默认 `http://localhost:3000`）

首次访问前端后，按照页面向导完成访问密码、Telegram、OpenAI 和群组摘要配置即可。

### 出站代理配置

如果运行环境不能直连 Telegram 或 OpenAI，可以在 `.env` 中配置出站代理：

```env
# OpenAI API 和 Telegram Bot API 使用 Go 标准 HTTP 代理环境变量
HTTPS_PROXY=socks5h://host.docker.internal:7890
HTTP_PROXY=socks5h://host.docker.internal:7890
NO_PROXY=localhost,127.0.0.1,postgres,app,web

# Telegram 用户客户端（登录、监听、同步群组、历史回补）使用这个变量
TGTLDR_TELEGRAM_PROXY_URL=socks5h://host.docker.internal:7890
```

`TGTLDR_TELEGRAM_PROXY_URL` 支持 `socks5://` 和 `socks5h://`，推荐使用 `socks5h://`。如果代理运行在宿主机上，Docker Desktop 用户通常需要写 `host.docker.internal:端口`，不要写 `127.0.0.1:端口`；Linux 用户需要确保容器可以访问宿主机代理地址。代理软件也需要允许来自 Docker 容器的连接，例如启用 LAN 访问或监听 `0.0.0.0`。

### 开发者：本地 Docker 构建启动

如果你需要在本地修改代码并重新构建镜像，请使用开发 override：

```bash
cp .env.example .env
docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build
```

### 手动开发启动

如果你已经使用 Docker 启动，不需要执行本节。手动方式适合开发调试，需要你自行准备 PostgreSQL、Go 和 Node.js 环境。

启动后端：

```bash
cd app
export TGTLDR_DATABASE_URL='postgres://postgres:postgres@localhost:5432/tgtldr?sslmode=disable'
export TGTLDR_MASTER_KEY_FILE="$HOME/.tgtldr/master.key"
export TGTLDR_MASTER_KEY='替换为 openssl rand -base64 32 生成的值'
# 可选：Telegram 用户客户端代理
# export TGTLDR_TELEGRAM_PROXY_URL='socks5h://127.0.0.1:7890'
go run ./cmd/server
```

启动前端：

```bash
cd web
npm install
TGTLDR_INTERNAL_API_BASE_URL=http://127.0.0.1:8080 npm run dev
```

## 安全提示

- `TGTLDR_MASTER_KEY` 用于加密保存 Telegram session、OpenAI API Key 和 Bot Token。
- 如果你不显式设置 `TGTLDR_MASTER_KEY`，系统会自动生成一把随机 key，并持久化到 `/var/lib/tgtldr/master.key`。
- 请妥善保存这把 key 或对应的数据卷；如果丢失，已经保存到数据库里的密钥和 Telegram session 将无法解密。
- 建议只部署在本机或可信内网；如果要暴露到公网，请先确认已经完成访问密码设置，并放在可信反向代理之后。

## 反向代理部署

如果你准备通过反向代理对外提供服务，请先在 `.env` 中配置这些值：

```env
TGTLDR_HOST_BIND=0.0.0.0
TGTLDR_WEB_ORIGIN=https://tgtldr.example.com
TGTLDR_HOST_WEB_PORT=13000
```

其中：

- `TGTLDR_HOST_BIND`：让容器监听服务器上的所有网卡
- `TGTLDR_WEB_ORIGIN`：填写用户实际访问的公网地址
- `TGTLDR_HOST_WEB_PORT`：反向代理转发到的本机端口

然后启动服务：

```bash
cp .env.example .env
# 编辑 .env
docker compose up -d
```

反向代理只需要转发到 `TGTLDR_HOST_WEB_PORT` 对应的本机端口即可。

Nginx 示例（假设 `TGTLDR_HOST_WEB_PORT=13000`）：

```nginx
server {
    listen 80;
    server_name tgtldr.example.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name tgtldr.example.com;

    ssl_certificate     /path/to/fullchain.pem;
    ssl_certificate_key /path/to/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:13000;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

## 镜像发布

- 默认 `docker-compose.yml` 面向普通用户，直接使用预构建镜像。
- `docker-compose.dev.yml` 面向开发者，保留本地 build 工作流。
- GitHub Actions 会在推送 `main` 或 `v*` tag 时，自动构建并推送：
  - `fr0der1c/tgtldr-app`
  - `fr0der1c/tgtldr-web`

## 贡献指南

本项目是一个由 AI 辅助编码的实验性项目。我们欢迎 AI 生成的代码，但这并不意味着你可以不用为提交的代码质量负责。无论代码是否由 AI 编写，提交 Pull Request 的人都是代码的第一责任人。

使用 AI 进行编码时，请确保遵守本项目中的约定，尤其是先阅读并遵循 [AGENTS.md](AGENTS.md)。和前 AI 时代一样，你仍然需要确保 commit message 有意义，能够准确描述修改内容，并在提交 Pull Request 前将相关修改 squash 成一个单独的 commit。

一个推荐的习惯是：在提交 Pull Request 前，可以新开一个 agent，让它完全以 reviewer 的视角 review 你的变更，确认没有明显问题后再提交。

## License

本项目使用 [PolyForm Noncommercial License 1.0.0](LICENSE)。

你可以基于非商业目的使用、fork、修改和分发本项目。商业使用需要获得作者单独授权。

## 文档

- [架构方案](docs/ARCHITECTURE.md)
- [产品流程与实施计划](docs/PRODUCT_FLOW.md)
