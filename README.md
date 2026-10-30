# Personal Bot

自用的**个人专属**对话助手。

**当前主运行时：Go。**  
旧版 Node/TypeScript 代码在 `legacy/node/`。

```
QQ / 飞书 / 微信     gateway_api / codex_local / cursor_local
        │                              │
        ▼                              ▼
  MessagingAdapter               ModelChannel
        │                              │
        └──────────► Orchestrator ◄────┘
                         │
                    session + tools
```

端到端总体流程图见 **[docs/MODULES.md §1.1](docs/MODULES.md#11-总体流程图)**；分层框架图见 **[§1.2](docs/MODULES.md#12-框架图)**。

## 现状（Go）

| 层 | 已就绪 | 说明 |
|----|--------|------|
| 消息平台 | `qq`（WebSocket + Webhook）、`feishu`（Webhook） | `wechat` 仅为占位 |
| 模型通道 | `gateway_api`、`codex_local`、`cursor_local` | 三者同级；另可用 `/cursor` 会话级进入 Cursor 模式 |
| 工具 | `read_file` / `write_file` / `list_dir` / `run_command` / `get_workdir` / `set_workdir` / `web_search` / `fetch_url` | 仅 `gateway_api`；按需调用；会话级 workdir；无 Key 联网检索 |
| 会话 | 多会话、压缩、token 统计、Cursor 模式与 Agent resume | 内存存储，重启清空；配置在 `.env`，改完需重启 |

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
# 日常对话（推荐）
MODEL_CHANNEL=gateway_api
GATEWAY_API_BASE_URL=https://你的网关/v1
GATEWAY_API_KEY=sk-xxx
GATEWAY_API_MODEL=你的模型名

# 或本机 Codex CLI（同级通道）
MODEL_CHANNEL=codex_local
CODEX_WORKING_DIRECTORY=E:\path\to\repo
# CODEX_BIN=codex

# 或全程 Cursor（同级通道）
MODEL_CHANNEL=cursor_local
CURSOR_API_KEY=cursor_...
CURSOR_MODEL=composer-2.5
TOOL_WORKSPACE=E:\cursor
```

### Cursor 模式（推荐：主通道仍用 gateway）

保持 `MODEL_CHANNEL=gateway_api`，配置 Cursor 后用斜杠进出模式：

```env
CURSOR_API_KEY=cursor_...
CURSOR_MODEL=composer-2.5
TOOL_WORKSPACE=E:\cursor
```

```bash
npm install --prefix scripts/cursor-agent   # 需 Node ≥ 22.13
```

推荐流程：

1. 普通对话让模型 `list_dir` / `set_workdir` 切到目标仓库（**任意盘符路径**）
2. `/cursor` 进入 Cursor 模式
3. 直接发改代码需求（走 Cursor SDK，同会话 resume）
4. `/cursor exit` 退回普通对话

也支持 `/cursor D:\repos\foo`（先设目录再进入模式），或 `/cursor 修登录`（进入并立刻执行第一条）。

### 工具（`gateway_api`）

```env
TOOLS_ENABLED=true
WEB_TOOLS_ENABLED=true
TOOL_WORKSPACE=E:\cursor
TOOL_COMMAND_TIMEOUT_SEC=60
TOOL_HTTP_TIMEOUT_SEC=20
TOOL_MAX_ROUNDS=8
TOOL_MAX_OUTPUT_BYTES=100000
```

| 工具 | 作用 |
|------|------|
| `list_dir` / `read_file` / `write_file` / `run_command` | 相对**当前会话 workdir** |
| `get_workdir` / `set_workdir` | 查看 / 切换工作目录（可绝对路径到任意盘） |
| `web_search` | 无 Key 联网搜索（DuckDuckGo HTML）；返回标题/链接/摘要 |
| `fetch_url` | 拉取 http(s) 页面正文（HTML 去标签）；受 `TOOL_MAX_OUTPUT_BYTES` 截断 |

模型按需调用工具（`tool_choice=auto`），不会每次全调。摘要压缩请求会 `DisableTools`，避免压缩时再调工具；`/cursor` 走 Cursor SDK，不用这套 Go 工具。

DDG HTML 可能偶发风控或页面结构变化。`WEB_TOOLS_ENABLED=false` 可关掉联网工具。上述 `TOOLS_*` / `WEB_*` 均读自 `.env`，进程启动时载入，**改配置需重启**。

`TOOL_WORKSPACE` 只是**默认起点**，不是路径监狱。切换后文件工具与 `/cursor` 共用该会话 workdir。

### 多会话、压缩与 Token

按 `平台:聊天:用户` 隔离；每人最多 10 个会话；内存存储（进程重启清空）。TTL 默认 2 小时。

| 指令 | 说明 |
|------|------|
| `/help` | 帮助 |
| `/list` | 列出会话（`*` 为当前） |
| `/new [标题]` | 新建并切换 |
| `/use N` | 切换到第 N 个 |
| `/rename 标题` | 重命名当前 |
| `/ctx` | 查看当前上下文 |
| `/tokens` | 查看 token 消耗 |
| `/tokens clear` / `/tokens clear all` | 清空统计 |
| `/workspace` | 查看当前工作目录；`clear` 回默认 |
| `/cursor` | 进入 Cursor 模式 |
| `/cursor exit` | 退出 Cursor 模式 |
| `/cursor status` | 查看模式状态 |
| `/clear` | 清空上下文（保留 Cursor 模式与 workdir） |
| `/clear tokens` | 同 `/tokens clear` |

消息超过 `SESSION_COMPRESS_AFTER`（默认 30）自动摘要压缩，保留最近 `SESSION_KEEP_RECENT`（默认 16）条。Cursor 模式下不做条数压缩。

```env
SESSION_TTL_HOURS=2
SESSION_COMPRESS_AFTER=30
SESSION_KEEP_RECENT=16
```

## 目录结构

```
cmd/personal-bot/main.go
docs/MODULES.md
scripts/cursor-agent/          # Cursor SDK Node 助手
internal/
  config/
  httpapi/
  orchestrator/                # 指令、Cursor 模式、workdir
  session/                     # 多会话 / 压缩 / token / CursorMode
  app/
  tools/                       # fs + workdir + shell + web_search/fetch_url
  messaging/ qq|feishu|wechat
  models/ gateway|codex|cursor
legacy/node/
```

模块细节见 **[docs/MODULES.md](docs/MODULES.md)**。

## 说明

- `gateway_api` 可调工具；`codex_local` / `cursor_local` 为同级本地通道。
- Cursor 也可不改 `MODEL_CHANNEL`，用会话级 `/cursor` 模式临时切入。
- 联网检索无需第三方 API Key（DuckDuckGo HTML + 直接拉页）。
- 微信适配器尚未实现。
- 密钥只放 `.env`（已 gitignore）；`scripts/cursor-agent/node_modules` 不入库。
