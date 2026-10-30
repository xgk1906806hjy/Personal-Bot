import WebSocket from "ws";
import type { Orchestrator } from "../../../orchestrator.js";
import type { QqApiClient } from "./client.js";
import { handleQqInbound } from "./handler.js";

const OP_DISPATCH = 0;
const OP_HEARTBEAT = 1;
const OP_IDENTIFY = 2;
const OP_RESUME = 6;
const OP_RECONNECT = 7;
const OP_INVALID_SESSION = 9;
const OP_HELLO = 10;
const OP_HEARTBEAT_ACK = 11;

/** GROUP_AND_C2C_EVENT — group @ + C2C messages */
const INTENT_GROUP_AND_C2C = 1 << 25;

interface GatewayPayload {
  op: number;
  d?: unknown;
  s?: number | null;
  t?: string;
}

export interface QqWsClientOptions {
  api: QqApiClient;
  orchestrator: Orchestrator;
  /** Optional override; otherwise GET /gateway */
  gatewayUrl?: string;
}

/**
 * QQ Bot WebSocket gateway client.
 * Outbound long connection — no public domain/webhook required.
 * https://bot.q.qq.com/wiki/develop/api-v2/dev-prepare/event-emit/websocket.html
 */
export class QqWsClient {
  private readonly api: QqApiClient;
  private readonly orchestrator: Orchestrator;
  private readonly gatewayUrlOverride?: string;

  private ws: WebSocket | null = null;
  private heartbeatTimer: NodeJS.Timeout | null = null;
  private reconnectTimer: NodeJS.Timeout | null = null;
  private stopped = false;

  private sessionId: string | null = null;
  private lastSeq: number | null = null;
  private resumePreferred = false;
  private reconnectAttempt = 0;
  private heartbeatIntervalMs = 45_000;

  constructor(opts: QqWsClientOptions) {
    this.api = opts.api;
    this.orchestrator = opts.orchestrator;
    this.gatewayUrlOverride = opts.gatewayUrl;
  }

  start(): void {
    this.stopped = false;
    void this.connect();
  }

  stop(): void {
    this.stopped = true;
    this.clearHeartbeat();
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      try {
        this.ws.close(1000, "client stop");
      } catch {
        /* ignore */
      }
      this.ws = null;
    }
  }

  private async connect(): Promise<void> {
    if (this.stopped) return;

    try {
      // Ensure token is warm before GET /gateway
      await this.api.getAccessToken();
      const url = this.gatewayUrlOverride || (await this.api.getGatewayUrl());
      console.info(`[qq:ws] connecting ${url}`);

      const ws = new WebSocket(url);
      this.ws = ws;

      ws.on("open", () => {
        console.info("[qq:ws] socket open, waiting for Hello");
      });

      ws.on("message", (data) => {
        void this.onMessage(data);
      });

      ws.on("close", (code, reason) => {
        this.clearHeartbeat();
        this.ws = null;
        const why = reason?.toString() || "";
        console.warn(`[qq:ws] closed code=${code} reason=${why}`);
        this.scheduleReconnect(code);
      });

      ws.on("error", (err) => {
        console.error(`[qq:ws] error: ${err.message}`);
      });
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      console.error(`[qq:ws] connect failed: ${msg}`);
      this.scheduleReconnect();
    }
  }

  private async onMessage(raw: WebSocket.RawData): Promise<void> {
    let payload: GatewayPayload;
    try {
      payload = JSON.parse(raw.toString()) as GatewayPayload;
    } catch {
      console.warn("[qq:ws] non-json frame ignored");
      return;
    }

    if (typeof payload.s === "number") {
      this.lastSeq = payload.s;
    }

    switch (payload.op) {
      case OP_HELLO: {
        this.heartbeatIntervalMs = Number(
          (payload.d as { heartbeat_interval?: number } | undefined)?.heartbeat_interval ?? 45000,
        );
        // Auth first; start heartbeat after READY / RESUMED per QQ docs.
        if (this.resumePreferred && this.sessionId && this.lastSeq != null) {
          await this.sendResume();
        } else {
          await this.sendIdentify();
        }
        break;
      }
      case OP_DISPATCH: {
        const t = payload.t ?? "";
        const d = (payload.d ?? {}) as Record<string, unknown>;
        if (t === "READY") {
          this.sessionId = String(d.session_id ?? "") || null;
          this.resumePreferred = true;
          this.reconnectAttempt = 0;
          this.startHeartbeat(this.heartbeatIntervalMs);
          console.info(`[qq:ws] READY session=${this.sessionId ?? "(none)"}`);
          break;
        }
        if (t === "RESUMED") {
          this.reconnectAttempt = 0;
          this.startHeartbeat(this.heartbeatIntervalMs);
          console.info("[qq:ws] RESUMED");
          break;
        }
        void handleQqInbound(t, d, this.api, this.orchestrator);
        break;
      }
      case OP_HEARTBEAT_ACK:
        break;
      case OP_RECONNECT:
        console.info("[qq:ws] server requested reconnect");
        this.resumePreferred = true;
        this.ws?.close(4000, "reconnect");
        break;
      case OP_INVALID_SESSION: {
        const resumable = Boolean(payload.d);
        console.warn(`[qq:ws] invalid session resumable=${resumable}`);
        if (!resumable) {
          this.sessionId = null;
          this.lastSeq = null;
          this.resumePreferred = false;
        }
        this.ws?.close(4000, "invalid session");
        break;
      }
      default:
        console.info(`[qq:ws] unhandled op=${payload.op} t=${payload.t ?? ""}`);
    }
  }

  private async sendIdentify(): Promise<void> {
    const token = await this.api.getAccessToken();
    this.send({
      op: OP_IDENTIFY,
      d: {
        token: `QQBot ${token}`,
        intents: INTENT_GROUP_AND_C2C,
        shard: [0, 1],
        properties: {
          $os: process.platform,
          $browser: "personal-bot",
          $device: "personal-bot",
        },
      },
    });
    console.info("[qq:ws] Identify sent");
  }

  private async sendResume(): Promise<void> {
    if (!this.sessionId) {
      await this.sendIdentify();
      return;
    }
    const token = await this.api.getAccessToken();
    this.send({
      op: OP_RESUME,
      d: {
        token: `QQBot ${token}`,
        session_id: this.sessionId,
        seq: this.lastSeq ?? 0,
      },
    });
    console.info(`[qq:ws] Resume sent session=${this.sessionId} seq=${this.lastSeq}`);
  }

  private startHeartbeat(intervalMs: number): void {
    this.clearHeartbeat();
    const tick = () => {
      this.send({ op: OP_HEARTBEAT, d: this.lastSeq });
    };
    // Send once soon, then on interval (QQ: after auth, follow heartbeat_interval)
    tick();
    this.heartbeatTimer = setInterval(tick, intervalMs);
  }

  private clearHeartbeat(): void {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
  }

  private send(payload: GatewayPayload): void {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return;
    this.ws.send(JSON.stringify(payload));
  }

  private scheduleReconnect(closeCode?: number): void {
    if (this.stopped) return;

    // Fatal: banned / offline-only sandbox
    if (closeCode === 4914 || closeCode === 4915) {
      console.error(`[qq:ws] fatal close ${closeCode}, not reconnecting`);
      return;
    }

    // 4006 invalid session id → identify next time
    if (closeCode === 4006) {
      this.sessionId = null;
      this.lastSeq = null;
      this.resumePreferred = false;
    }

    const attempt = this.reconnectAttempt++;
    const delay = Math.min(30_000, 1000 * 2 ** Math.min(attempt, 5));
    console.info(`[qq:ws] reconnect in ${delay}ms (attempt ${attempt + 1})`);
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      void this.connect();
    }, delay);
  }
}
