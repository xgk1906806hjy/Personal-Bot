import type { Express } from "express";
import type { AppConfig } from "../config.js";
import type { Orchestrator } from "../orchestrator.js";

/** Chat platforms. Add wechat / qq when adapters are ready. */
export type MessagingPlatformId = "feishu" | "wechat" | "qq";

/** Normalized inbound message — platform adapters must produce this shape. */
export interface IncomingUserMessage {
  platform: MessagingPlatformId;
  chatId: string;
  userId: string;
  messageId: string;
  text: string;
}

export interface MessagingAdapterContext {
  config: AppConfig;
  orchestrator: Orchestrator;
}

/**
 * Pluggable messaging adapter.
 * Each platform owns its webhook/auth/reply details; Orchestrator stays platform-agnostic.
 */
export interface MessagingAdapter {
  readonly id: MessagingPlatformId;
  /** Mount HTTP routes (webhooks, verification, etc.) */
  mount(app: Express, ctx: MessagingAdapterContext): void;
}
