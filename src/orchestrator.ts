import type { AppConfig } from "./config.js";
import type { IncomingUserMessage } from "./messaging/types.js";
import type { ModelChannel } from "./models/types.js";
import { SessionStore, buildSessionKey } from "./session/store.js";

/**
 * Platform-agnostic conversation brain.
 * Messaging adapters normalize events → IncomingUserMessage; model channels are swapped via config.
 */
export class Orchestrator {
  constructor(
    private readonly config: AppConfig,
    private readonly model: ModelChannel,
    private readonly sessions: SessionStore,
  ) {}

  async handle(msg: IncomingUserMessage): Promise<string> {
    const sessionKey = buildSessionKey(msg.platform, msg.chatId, msg.userId);
    const existing = this.sessions.get(sessionKey);

    const result = await this.model.chat({
      sessionKey,
      conversationId: existing?.conversationId,
      prompt: msg.text,
      messages: [{ role: "user", content: msg.text }],
    });

    this.sessions.touch(sessionKey, result.conversationId);
    return result.text;
  }

  get modelChannelId() {
    return this.model.id;
  }

  get systemPrompt() {
    return this.config.BOT_SYSTEM_PROMPT;
  }
}
