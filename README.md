# Personal Bot

A **personal exclusive** chat assistant you run yourself: pluggable messaging fronts + pluggable model backends.

```
QQ / Feishu / WeChat          codex_local / gateway_api / …
        │                              │
        ▼                              ▼
  MessagingAdapter               ModelChannel
        │                              │
        └──────────► Orchestrator ◄────┘
```

Built for one owner (you): talk to your bot on QQ (or later Feishu/WeChat), backed by your own model gateway or local Codex.

## Status

| Layer | Ready | Planned |
|-------|-------|---------|
| Messaging | `qq` (WebSocket + Webhook), `feishu` | `wechat` |
| Models | `gateway_api`, `codex_local` | add factories in `models/registry.ts` |

Default path today: **QQ WebSocket** — no public domain or server required.

## Quick start (QQ WebSocket)

```bash
cd E:\workspace\feishu-codex-bot
npm install
copy .env.example .env
# Fill QQ_APP_ID / QQ_APP_SECRET and model gateway settings
npm run dev
```

When you see `[qq:ws] READY`, the bot is online. Keep the process running.

Health check: `GET http://127.0.0.1:8787/healthz`

### Minimal `.env`

```env
MESSAGING_CHANNELS=qq
QQ_TRANSPORT=websocket
QQ_APP_ID=your_app_id
QQ_APP_SECRET=your_app_secret
QQ_API_BASE_URL=https://api.bot.qq.com

MODEL_CHANNEL=gateway_api
GATEWAY_API_BASE_URL=https://your-gateway/v1
GATEWAY_API_KEY=sk-xxx
GATEWAY_API_MODEL=your-model

BOT_SYSTEM_PROMPT=You are my personal assistant. Be concise, practical, and honest when unsure.
```

1. Create a bot at [QQ Open Platform](https://q.qq.com/).
2. Choose **WebSocket** as the event transport.
3. Add the bot to your chat / group, then `@bot` with a message.

## Architecture

- **MessagingAdapter** — platform webhooks / sockets → normalized `IncomingUserMessage`
- **Orchestrator** — session mapping `{platform}:{chatId}:{userId}` → model call
- **ModelChannel** — `gateway_api` (OpenAI-compatible) or `codex_local` (`@openai/codex-sdk`)

Swap either side via env without rewriting the core loop.

### Messaging

```env
MESSAGING_CHANNELS=qq
# Feishu: MESSAGING_CHANNELS=feishu
# Both:   MESSAGING_CHANNELS=qq,feishu
```

QQ transport:

```env
QQ_TRANSPORT=websocket   # default — no public URL
# QQ_TRANSPORT=webhook   # needs HTTPS on 80/443/8080/8443 → /qq/webhook
# QQ_TRANSPORT=both
```

### Models

```env
MODEL_CHANNEL=gateway_api

# or local Codex CLI
MODEL_CHANNEL=codex_local
CODEX_WORKING_DIRECTORY=E:\path\to\repo
```

## Project layout

```
src/
  index.ts                 # boot
  config.ts                # env
  orchestrator.ts          # platform-agnostic brain
  http/app.ts              # health + adapter mounts
  session/store.ts         # in-memory sessions
  messaging/
    types.ts
    registry.ts
    adapters/
      qq/                  # WebSocket + Webhook
      feishu/
      wechat/              # stub
  models/
    types.ts
    registry.ts
    channels/
      gateway-api.ts
      codex-local.ts
```

## Scripts

| Command | Description |
|---------|-------------|
| `npm run dev` | Dev server with reload (`tsx watch`) |
| `npm start` | Run once |
| `npm run typecheck` | TypeScript check |
| `npm run build` | Emit `dist/` |

## Notes

- Replies are English by default (ack → answer). Change `BOT_SYSTEM_PROMPT` to reshape personality.
- QQ passive replies are time-windowed; the bot ACKs “Got it — thinking…” then sends the final answer.
- Secrets stay in `.env` (gitignored). Rotate keys if they were ever pasted into chat.
- Tooling (device logs, repo tools, owner allowlists) can be layered on the Orchestrator / model channels next.
