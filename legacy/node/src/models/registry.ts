import type { AppConfig } from "../config.js";
import { CodexLocalChannel } from "./channels/codex-local.js";
import { GatewayApiChannel } from "./channels/gateway-api.js";
import type { ModelChannel, ModelChannelId } from "./types.js";

type ModelFactory = (config: AppConfig) => ModelChannel;

const FACTORIES: Record<ModelChannelId, ModelFactory> = {
  codex_local: (config) => new CodexLocalChannel(config),
  gateway_api: (config) => new GatewayApiChannel(config),
};

export function createModelChannel(
  id: ModelChannelId,
  config: AppConfig,
): ModelChannel {
  const factory = FACTORIES[id];
  if (!factory) {
    throw new Error(`Unknown model channel: ${id}`);
  }
  return factory(config);
}

export function listModelChannels(): ModelChannelId[] {
  return Object.keys(FACTORIES) as ModelChannelId[];
}
