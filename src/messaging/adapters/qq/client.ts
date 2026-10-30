import type { AppConfig } from "../../../config.js";

interface TokenCache {
  accessToken: string;
  expiresAtMs: number;
}

export type QqReplyTarget =
  | { kind: "group"; groupOpenid: string }
  | { kind: "c2c"; userOpenid: string };

/**
 * QQ Bot OpenAPI client (access token + passive message reply).
 * Docs: https://bot.q.qq.com/wiki/develop/api-v2/
 */
export class QqApiClient {
  private readonly appId: string;
  private readonly clientSecret: string;
  private readonly apiBase: string;
  private tokenCache: TokenCache | null = null;
  private readonly msgSeqByMessageId = new Map<string, number>();

  constructor(config: AppConfig) {
    this.appId = config.QQ_APP_ID;
    this.clientSecret = config.QQ_APP_SECRET;
    this.apiBase = config.QQ_API_BASE_URL.replace(/\/$/, "");
  }

  async getAccessToken(): Promise<string> {
    const now = Date.now();
    if (this.tokenCache && now < this.tokenCache.expiresAtMs - 60_000) {
      return this.tokenCache.accessToken;
    }

    const res = await fetch(`${this.apiBase}/app/getAppAccessToken`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        appId: this.appId,
        clientSecret: this.clientSecret,
      }),
    });

    const data = (await res.json()) as {
      access_token?: string;
      expires_in?: string | number;
      code?: number;
      message?: string;
    };

    if (!data.access_token) {
      throw new Error(
        `QQ getAppAccessToken failed: ${data.message || data.code || res.statusText}`,
      );
    }

    const expiresInSec = Number(data.expires_in ?? 7200);
    this.tokenCache = {
      accessToken: data.access_token,
      expiresAtMs: now + expiresInSec * 1000,
    };
    return data.access_token;
  }

  /** GET /gateway — WebSocket entry URL */
  async getGatewayUrl(): Promise<string> {
    const token = await this.getAccessToken();
    const res = await fetch(`${this.apiBase}/gateway`, {
      headers: { Authorization: `QQBot ${token}` },
    });
    const data = (await res.json()) as { url?: string; message?: string; code?: number };
    if (!data.url) {
      throw new Error(
        `QQ get gateway failed: ${data.message || data.code || res.statusText}`,
      );
    }
    return data.url;
  }

  async replyText(
    target: QqReplyTarget,
    messageId: string,
    text: string,
  ): Promise<void> {
    const token = await this.getAccessToken();
    const msgSeq = this.nextMsgSeq(messageId);
    const path =
      target.kind === "group"
        ? `/v2/groups/${encodeURIComponent(target.groupOpenid)}/messages`
        : `/v2/users/${encodeURIComponent(target.userOpenid)}/messages`;

    const res = await fetch(`${this.apiBase}${path}`, {
      method: "POST",
      headers: {
        Authorization: `QQBot ${token}`,
        "Content-Type": "application/json; charset=utf-8",
      },
      body: JSON.stringify({
        content: truncate(text, 4000),
        msg_type: 0,
        msg_id: messageId,
        msg_seq: msgSeq,
      }),
    });

    const data = (await res.json().catch(() => ({}))) as {
      err_code?: number;
      message?: string;
      id?: string;
    };

    if (!res.ok || (typeof data.err_code === "number" && data.err_code !== 0)) {
      const detail = data.message || res.statusText || `HTTP ${res.status}`;
      throw new Error(`QQ reply failed: ${detail}`);
    }
  }

  private nextMsgSeq(messageId: string): number {
    const next = (this.msgSeqByMessageId.get(messageId) ?? 0) + 1;
    this.msgSeqByMessageId.set(messageId, next);
    if (this.msgSeqByMessageId.size > 5000) {
      const first = this.msgSeqByMessageId.keys().next().value;
      if (first) this.msgSeqByMessageId.delete(first);
    }
    return next;
  }
}

function truncate(text: string, max: number): string {
  if (text.length <= max) return text;
  return `${text.slice(0, max - 20)}\n…(truncated)`;
}
