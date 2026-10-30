# Personal Bot

自用的**个人专属**对话助手。

**当前主运行时：Go。**  
旧版 Node/TypeScript 代码在 `legacy/node/`。

```
QQ / 飞书 / 微信          gateway_api / codex_local
        │                              │
        ▼                              ▼
  MessagingAdapter               ModelChannel
        │                              │
        └──────────► Orchestrator ◄────┘
```

端到端总体流程图（启动 → 消息 → 会话/工具/压缩 → 回复）见 **[docs/MODULES.md §1.1](docs/MODULES.md#11-总体流程图)**。

## 现状（Go）

| 层 | 已就绪 | 说明 |
|----|--------|------|
| 消息平台 | `qq`（WebSocket + Webhook）、`feishu`（Webhook） | `wechat` 仅为占位（Mount 会报错） |
| 模型通道 | `gateway_api`、`codex_local` | Codex 通过本机 `codex` CLI 调用 |
| 工具 | `read_file`、`write_file`、`list_dir`、`run_command` | 在 `gateway_api` 上走 OpenAI tools 循环；限制在 `TOOL_WORKSPACE` |

## 快速开始

```bash
go mod tidy
copy .env.example .env
# 填写凭证
go run ./cmd/personal-bot
```

编译：

```bash
go build -o bin/personal-bot.exe ./cmd/personal-bot
.\bin\personal-bot.exe
```

健康检查：`GET http://127.0.0.1:8787/healthz`

### QQ（推荐 WebSocket）

```env
MESSAGING_CHANNELS=qq
QQ_TRANSPORT=websocket
QQ_APP_ID=...
QQ_APP_SECRET=...
```

Webhook / 同时开启：

```env
QQ_TRANSPORT=webhook
# 或 QQ_TRANSPORT=both
# 回调地址：https://你的域名/qq/webhook（端口仅 80/443/8080/8443）
```

### 飞书

```env
MESSAGING_CHANNELS=feishu
# 或 MESSAGING_CHANNELS=qq,feishu
FEISHU_APP_ID=...
FEISHU_APP_SECRET=...
FEISHU_VERIFICATION_TOKEN=...
FEISHU_ENCRYPT_KEY=...
# 回调地址：https://你的域名/feishu/webhook
```

### 模型

```env
MODEL_CHANNEL=gateway_api
GATEWAY_API_BASE_URL=https://你的网关/v1
GATEWAY_API_KEY=sk-xxx
GATEWAY_API_MODEL=你的模型名

# 或本机 Codex CLI
MODEL_CHANNEL=codex_local
CODEX_WORKING_DIRECTORY=E:\path\to\repo
# CODEX_BIN=codex
```

### 工具（`gateway_api`）

```env
TOOLS_ENABLED=true
TOOL_WORKSPACE=E:\path\to\workspace
TOOL_COMMAND_TIMEOUT_SEC=60
TOOL_MAX_ROUNDS=8
TOOL_MAX_OUTPUT_BYTES=100000
```

内置工具：`read_file`、`write_file`、`list_dir`、`run_command`。  
文件路径与命令工作目录都限制在 `TOOL_WORKSPACE`（默认取 `CODEX_WORKING_DIRECTORY`，否则为进程当前目录）。

### 多会话与上下文压缩

按 `平台:聊天:用户` 隔离；每人最多 10 个会话，内存存储（进程重启清空）。TTL 默认 2 小时（`SESSION_TTL_HOURS`）。

斜杠指令（群里需 @ 机器人）：

| 指令 | 说明 |
|------|------|
| `/help` | 帮助 |
| `/list` | 列出会话（`*` 为当前） |
| `/new [标题]` | 新建并切换 |
| `/use N` | 切换到第 N 个 |
| `/rename 标题` | 重命名当前 |
| `/clear` | 清空当前上下文 |

消息条数超过 `SESSION_COMPRESS_AFTER`（默认 30）时，自动把更早内容压成摘要，保留最近 `SESSION_KEEP_RECENT`（默认 16）条原文。

```env
SESSION_TTL_HOURS=2
SESSION_COMPRESS_AFTER=30
SESSION_KEEP_RECENT=16
```

## 目录结构

```
cmd/personal-bot/main.go
docs/MODULES.md        # 各模块细节说明
internal/
  config/
  httpapi/
  orchestrator/
  session/
  app/                 # 适配器装配
  tools/               # 文件与命令工具
  messaging/
    qq/                # WS + Webhook + Ed25519
    feishu/            # Webhook + 回复 API
    wechat/            # 占位
  models/
    gateway.go         # 对话 + tools 循环
    codex.go           # `codex exec` 子进程
legacy/node/
```

模块职责、数据流与扩展点见 **[docs/MODULES.md](docs/MODULES.md)**。

## 说明

- `gateway_api` 可调工具；`codex_local` 依赖 Codex CLI 沙箱。
- 微信适配器尚未实现，启用后会在 Mount 时报明确错误。
- 密钥只放在 `.env`（已 gitignore）。
