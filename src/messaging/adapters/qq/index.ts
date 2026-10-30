import type { Request, Response, Express } from "express";
import express from "express";
import type {
  MessagingAdapter,
  MessagingAdapterContext,
} from "../../types.js";
import { QqApiClient } from "./client.js";
import { signValidationPayload, verifyCallbackSignature } from "./crypto.js";
import { handleQqInbound } from "./handler.js";
import { QqWsClient } from "./ws-client.js";

const OP_DISPATCH = 0;
const OP_HTTP_CALLBACK_ACK = 12;
const OP_CALLBACK_VERIFY = 13;

interface QqPayload {
  id?: string;
  op?: number;
  t?: string;
  d?: Record<string, unknown>;
}

interface RawBodyRequest extends Request {
  rawBody?: Buffer;
}

/**
 * QQ Open Platform official bot.
 * Transport: websocket (default, no public domain) | webhook | both
 */
export class QqAdapter implements MessagingAdapter {
  readonly id = "qq" as const;
  private wsClient: QqWsClient | null = null;

  mount(app: Express, ctx: MessagingAdapterContext): void {
    const { config, orchestrator } = ctx;
    const client = new QqApiClient(config);
    const transport = config.QQ_TRANSPORT;

    if (transport === "webhook" || transport === "both") {
      this.mountWebhook(app, client, orchestrator, config.QQ_APP_SECRET);
    }

    if (transport === "websocket" || transport === "both") {
      this.wsClient = new QqWsClient({ api: client, orchestrator });
      this.wsClient.start();
      console.info(
        "[boot] messaging adapter mounted: qq → WebSocket gateway (no public domain needed)",
      );
    }
  }

  private mountWebhook(
    app: Express,
    client: QqApiClient,
    orchestrator: MessagingAdapterContext["orchestrator"],
    botSecret: string,
  ): void {
    app.post(
      "/qq/webhook",
      express.json({
        verify: (req, _res, buf) => {
          (req as RawBodyRequest).rawBody = Buffer.from(buf);
        },
      }),
      (req, res) => {
        void this.handleWebhook(
          req as RawBodyRequest,
          res,
          client,
          orchestrator,
          botSecret,
        );
      },
    );

    console.info(
      "[boot] messaging adapter mounted: qq → POST /qq/webhook (ports allowed by QQ: 80/443/8080/8443)",
    );
  }

  private async handleWebhook(
    req: RawBodyRequest,
    res: Response,
    client: QqApiClient,
    orchestrator: MessagingAdapterContext["orchestrator"],
    botSecret: string,
  ): Promise<void> {
    try {
      const payload = req.body as QqPayload;
      const op = payload.op;

      if (op === OP_CALLBACK_VERIFY) {
        const plainToken = String(payload.d?.plain_token ?? "");
        const eventTs = String(payload.d?.event_ts ?? "");
        if (!plainToken || !eventTs) {
          res.status(400).json({ error: "invalid validation payload" });
          return;
        }
        const signature = signValidationPayload(botSecret, eventTs, plainToken);
        res.status(200).json({ plain_token: plainToken, signature });
        return;
      }

      const sig = req.header("X-Signature-Ed25519") ?? "";
      const ts = req.header("X-Signature-Timestamp") ?? "";
      const rawBody = req.rawBody;
      if (sig && ts && rawBody) {
        if (!verifyCallbackSignature(botSecret, ts, rawBody, sig)) {
          console.warn("[qq] signature verify failed");
          res.status(401).json({ error: "invalid signature" });
          return;
        }
      }

      res.status(200).json({ op: OP_HTTP_CALLBACK_ACK, d: 0 });

      if (op !== OP_DISPATCH) {
        return;
      }

      const eventType = payload.t ?? "";
      void handleQqInbound(eventType, payload.d ?? {}, client, orchestrator);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      console.error(`[qq] webhook error: ${msg}`);
      if (!res.headersSent) {
        res.status(500).json({ error: msg });
      }
    }
  }
}
