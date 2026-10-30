import type { Express } from "express";
import type { MessagingAdapter, MessagingAdapterContext } from "../../types.js";

/**
 * Placeholder for WeChat / WeCom adapter.
 * Implement mount() with platform webhook + reply, then map to IncomingUserMessage.
 */
export class WechatAdapter implements MessagingAdapter {
  readonly id = "wechat" as const;

  mount(_app: Express, _ctx: MessagingAdapterContext): void {
    throw new Error(
      'Messaging adapter "wechat" is not implemented yet. Add webhook parsing + reply in src/messaging/adapters/wechat/.',
    );
  }
}
