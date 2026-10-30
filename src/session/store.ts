import type { MessagingPlatformId } from "../messaging/types.js";

export interface SessionRecord {
  sessionKey: string;
  conversationId?: string;
  updatedAt: number;
}

/**
 * Simple in-memory session store.
 * Swap for Redis later when you scale to multiple workers.
 */
export class SessionStore {
  private readonly ttlMs: number;
  private readonly map = new Map<string, SessionRecord>();

  constructor(ttlHours: number) {
    this.ttlMs = ttlHours * 60 * 60 * 1000;
  }

  get(sessionKey: string): SessionRecord | undefined {
    const rec = this.map.get(sessionKey);
    if (!rec) return undefined;
    if (Date.now() - rec.updatedAt > this.ttlMs) {
      this.map.delete(sessionKey);
      return undefined;
    }
    return rec;
  }

  set(sessionKey: string, conversationId?: string): SessionRecord {
    const rec: SessionRecord = {
      sessionKey,
      conversationId,
      updatedAt: Date.now(),
    };
    this.map.set(sessionKey, rec);
    return rec;
  }

  touch(sessionKey: string, conversationId?: string): void {
    const existing = this.get(sessionKey);
    this.set(sessionKey, conversationId ?? existing?.conversationId);
  }
}

/** Prefixed by platform so Feishu / WeChat / QQ sessions never collide. */
export function buildSessionKey(
  platform: MessagingPlatformId,
  chatId: string,
  userId: string,
): string {
  return `${platform}:${chatId}:${userId}`;
}
