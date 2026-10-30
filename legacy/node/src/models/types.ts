/**
 * Pluggable model backends.
 * To add a third path: extend ModelChannelId, implement ModelChannel, register in registry.ts.
 */
export type ModelChannelId = "codex_local" | "gateway_api";

export type ChatRole = "system" | "user" | "assistant";

export interface ChatMessage {
  role: ChatRole;
  content: string;
}

export interface ModelRequest {
  /** Stable id for multi-turn mapping, e.g. feishu:{chatId}:{userId} */
  sessionKey: string;
  /** Provider-specific conversation id if already known */
  conversationId?: string;
  messages: ChatMessage[];
  /** Latest user text (convenience; also present in messages) */
  prompt: string;
}

export interface ModelResponse {
  channel: ModelChannelId;
  text: string;
  /** Persist this to continue the same conversation later */
  conversationId?: string;
  raw?: unknown;
}

export interface ModelChannel {
  readonly id: ModelChannelId;
  chat(req: ModelRequest): Promise<ModelResponse>;
}
