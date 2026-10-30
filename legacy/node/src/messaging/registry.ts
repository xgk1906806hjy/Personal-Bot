import type { AppConfig } from "../config.js";
import { FeishuAdapter } from "./adapters/feishu/index.js";
import { QqAdapter } from "./adapters/qq/index.js";
import { WechatAdapter } from "./adapters/wechat/index.js";
import type { MessagingAdapter, MessagingPlatformId } from "./types.js";

const FACTORIES: Record<MessagingPlatformId, () => MessagingAdapter> = {
  feishu: () => new FeishuAdapter(),
  wechat: () => new WechatAdapter(),
  qq: () => new QqAdapter(),
};

/** Platforms that are wired enough to enable in MESSAGING_CHANNELS today. */
const READY: MessagingPlatformId[] = ["feishu", "qq"];

export function listMessagingPlatforms(): MessagingPlatformId[] {
  return Object.keys(FACTORIES) as MessagingPlatformId[];
}

export function listReadyMessagingPlatforms(): MessagingPlatformId[] {
  return [...READY];
}

export function createMessagingAdapter(id: MessagingPlatformId): MessagingAdapter {
  const factory = FACTORIES[id];
  if (!factory) {
    throw new Error(`Unknown messaging platform: ${id}`);
  }
  if (!READY.includes(id)) {
    throw new Error(
      `Messaging platform "${id}" is registered but not ready. Implemented: ${READY.join(", ")}`,
    );
  }
  return factory();
}

export function createMessagingAdapters(
  ids: MessagingPlatformId[],
  _config: AppConfig,
): MessagingAdapter[] {
  if (ids.length === 0) {
    throw new Error("MESSAGING_CHANNELS must include at least one platform");
  }
  return ids.map((id) => createMessagingAdapter(id));
}
