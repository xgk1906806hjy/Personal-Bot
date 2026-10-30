# Personal Bot

A **personal exclusive** chat assistant you run yourself.

**Active runtime: Go.**  
Previous Node/TypeScript code is under `legacy/node/`.

```
QQ / Feishu / WeChat          gateway_api / codex_local
        │                              │
        ▼                              ▼
  MessagingAdapter               ModelChannel
        │                              │
        └──────────► Orchestrator ◄────┘
```

## Status (Go)

| Layer | Ready | Notes |
|-------|-------|-------|
| Messaging | `qq` (WebSocket + Webhook), `feishu` (Webhook) | `wechat` placeholder (Mount errors until implemented) |
| Models | `gateway_api`, `codex_local` | Codex shells out to local `codex` CLI |

## Quick start

```bash
go mod tidy
copy .env.example .env
# fill credentials
go run ./cmd/personal-bot
```

Build:

```bash
go build -o bin/personal-bot.exe ./cmd/personal-bot
.\bin\personal-bot.exe
```

Health: `GET http://127.0.0.1:8787/healthz`

### QQ (recommended WebSocket)

```env
MESSAGING_CHANNELS=qq
QQ_TRANSPORT=websocket
QQ_APP_ID=...
QQ_APP_SECRET=...
```

Webhook / both:

```env
QQ_TRANSPORT=webhook
# or QQ_TRANSPORT=both
# callback: https://your-host/qq/webhook  (ports 80/443/8080/8443)
```

### Feishu

```env
MESSAGING_CHANNELS=feishu
# or MESSAGING_CHANNELS=qq,feishu
FEISHU_APP_ID=...
FEISHU_APP_SECRET=...
FEISHU_VERIFICATION_TOKEN=...
FEISHU_ENCRYPT_KEY=...
# callback: https://your-host/feishu/webhook
```

### Models

```env
MODEL_CHANNEL=gateway_api
GATEWAY_API_BASE_URL=https://your-gateway/v1
GATEWAY_API_KEY=sk-xxx
GATEWAY_API_MODEL=your-model

# or local Codex CLI
MODEL_CHANNEL=codex_local
CODEX_WORKING_DIRECTORY=E:\path\to\repo
# CODEX_BIN=codex
```

## Layout

```
cmd/personal-bot/main.go
internal/
  config/
  httpapi/
  orchestrator/
  session/
  app/                 # adapter wiring
  messaging/
    qq/                # WS + Webhook + Ed25519
    feishu/            # Webhook + reply API
    wechat/            # placeholder
  models/
    gateway.go
    codex.go           # `codex exec` subprocess
legacy/node/
```

## Notes

- Chat-only by default; `codex_local` can use the Codex CLI sandbox in its working directory.
- WeChat adapter is intentionally not ready — selecting it will fail at Mount with a clear error.
- Secrets stay in `.env` (gitignored).
