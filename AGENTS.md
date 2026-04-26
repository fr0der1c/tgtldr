# TGTLDR Agent Guide

## 项目定位

TGTLDR 是一个单用户自部署的 Telegram 群消息监听与 AI 摘要系统。

系统目标：
- 监听用户已加入的 Telegram 群组消息并落库
- 按配置时间为前一天消息生成摘要
- 支持网页端阅读摘要
- 可选通过 Telegram Bot 推送摘要
- 支持历史消息回补、手动重跑、原始 prompt 预览与摘要搜索

## 核心设计要点

### 1. 部署模型
- 单用户、自部署
- 默认通过 Docker Compose 运行
- 初始化完成后，所有页面和 API 都需要本地密码登录

### 2. 技术栈
- 后端：Go
- Telegram 客户端：`gotd/td`
- 数据库：PostgreSQL
- 前端：Next.js App Router

### 3. 主要模块
- `app/internal/api`
  - HTTP 路由与接口层
- `app/internal/telegram`
  - Telegram 登录、长连接、群组同步、历史消息回补
- `app/internal/summary`
  - 摘要输入构建、动态 budget、chunk 切分、回复关系处理
- `app/internal/scheduler`
  - 定时摘要、自动发送、重跑策略
- `app/internal/bot`
  - Telegram Bot 发送与格式化
- `app/internal/store`
  - PostgreSQL 数据访问层
- `web/app`
  - 页面路由
- `web/components`
  - UI 组件和页面组合逻辑
- `web/lib`
  - 前端 API 调用与类型定义

### 4. 摘要生成规则
- 按系统默认时区切分自然日
- 同一天白天手动生成的摘要视为预览版
- 定时发送前会检查：
  - 该摘要是否在目标日期结束后生成
  - 若不是，则强制重跑后再发送
- 多 chunk 阶段摘要并行执行
- 最终汇总串行执行
- `Max Output Tokens` 支持：
  - `auto`
  - `manual`

### 5. 摘要输入构建规则
- 只使用最终摘要正文做搜索，不搜索原始 prompt
- reply 关系会被显式编码进 transcript：
  - `reply_to=[m001]`
  - `reply_excerpt="..."`
- 对无文本的引用消息会给出占位说明：
  - `[图片消息，无文字说明]`
  - `[文件消息，无文字说明]`
  - `[非文本消息，无文字说明]`
- 群组级可配置：
  - 是否保留机器人消息
  - 过滤发言人
  - 过滤关键词

### 6. 历史回补
- 支持按单个群组回补历史消息
- 支持最近 7 天、最近 30 天或自定义日期范围
- 历史回补是异步任务
- 遇到 `FLOOD_WAIT` 时会自动等待并继续
- 历史回补完成后，如果回补范围内存在“0 消息 / 0 分块”的空摘要，且现在已有消息，会自动重跑该摘要

### 7. 摘要页交互
- 主页面使用“列表 + 右侧 Drawer”模式
- 左侧列表负责：
  - 搜索
  - 筛选
  - 分页
  - 结果浏览
- 点击某条摘要后，从右侧 Drawer 打开完整正文
- 原始 prompt 在弹窗中按：
  - 系统提示词
  - Chunk 1 / Chunk 2 / ...
  - 合并提示词
  进行折叠展示

### 8. 搜索实现
- 使用 PostgreSQL `pg_trgm` + GIN 索引
- 搜索范围：
  - `summaries.content`
  - `chats.title`
- 多关键词按空格拆分，并采用 `AND`
- 左侧搜索结果会展示命中片段
- 命中片段支持关键词高亮
- 顶部统计与搜索结果总数解耦，搜索不会影响摘要总数卡片

### 9. Bot 推送
- 只有在以下条件都满足时，才允许手动发送：
  - 群组 `deliveryMode = bot`
  - 全局 `botEnabled = true`
  - `botToken` 已配置
  - `botTargetChatId` 已配置
  - 摘要已生成成功
- Telegram 消息发送时使用 `parse_mode=HTML`
- 会将 Markdown 摘要转换成更适合 Telegram 的 HTML 子集

### 10. 认证
- 首次初始化时，需要设置本地访问密码
- 初始化完成后：
  - 页面需要登录
  - API 需要 Cookie Session
- 系统配置页支持修改密码

## 常见命令

### 1. Docker 开发与发布

首次启动：

```bash
cp .env.example .env
docker compose up -d
```

只更新后端：

```bash
docker compose -f docker-compose.yml -f docker-compose.dev.yml build app
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d app
```

只更新前端：

```bash
docker compose -f docker-compose.yml -f docker-compose.dev.yml build web
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d web
```

同时更新前后端：

```bash
docker compose -f docker-compose.yml -f docker-compose.dev.yml build app web
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d app web
```

查看容器状态：

```bash
docker compose ps
```

查看日志：

```bash
docker compose logs -f app
docker compose logs -f web
docker compose logs -f postgres
```

### 2. 后端开发

安装依赖并整理模块：

```bash
cd app
go mod tidy
```

运行测试：

```bash
cd app
go test ./...
```

格式化：

```bash
cd app
gofmt -w $(find . -name '*.go')
```

本地启动：

```bash
cd app
go run ./cmd/server
```

### 3. 前端开发

安装依赖：

```bash
cd web
npm install
```

开发模式：

```bash
cd web
npm run dev
```

生产构建：

```bash
cd web
npm run build
```

### 4. 数据库与健康检查

健康检查：

```bash
curl http://127.0.0.1:8080/api/health
```

查看摘要接口：

```bash
curl --cookie "tgtldr_session=<session>" "http://127.0.0.1:8080/api/summaries?page=1&pageSize=20"
```

### 5. Git

查看状态：

```bash
git status
```

查看变更：

```bash
git diff
```

提交：

```bash
git add .
git commit -m "your message"
```

## 维护建议

- 优先保持函数短小、职责清晰
- 新增逻辑时优先考虑是否可以拆成独立小文件
- 不要把“开发者解释文案”写进最终 UI
- 涉及时间判断时，优先使用系统默认时区，不要依赖服务器本地默认行为
- 能复用现有 API、调度器和组件时，不要额外平行造轮子
