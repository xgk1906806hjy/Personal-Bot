import path from "node:path";
import { config as loadEnv } from "dotenv";
import { z } from "zod";
import type { MessagingPlatformId } from "./messaging/types.js";
import type { ModelChannelId } from "./models/types.js";

loadEnv();

const messagingPlatformSchema = z.enum(["feishu", "wechat", "qq"]);
const modelChannelSchema = z.enum(["codex_local", "gateway_api"]);

const envSchema = z.object({
  PORT: z.coerce.number().default(8787),
  HOST: z.string().default("0.0.0.0"),

  /** Comma-separated messaging platforms, e.g. "feishu" or "feishu,wechat" */
  MESSAGING_CHANNELS: z.string().default("feishu"),
  /** Active model backend: codex_local | gateway_api */
  MODEL_CHANNEL: modelChannelSchema.default("gateway_api"),

  FEISHU_APP_ID: z.string().optional().default(""),
  FEISHU_APP_SECRET: z.string().optional().default(""),
  FEISHU_ENCRYPT_KEY: z.string().optional().default(""),
  FEISHU_VERIFICATION_TOKEN: z.string().optional().default(""),

  /** QQ Open Platform official bot */
  QQ_APP_ID: z.string().optional().default(""),
  QQ_APP_SECRET: z.string().optional().default(""),
  QQ_API_BASE_URL: z.string().url().default("https://api.bot.qq.com"),
  /** Event transport: websocket (no public domain) | webhook | both */
  QQ_TRANSPORT: z.enum(["websocket", "webhook", "both"]).default("websocket"),

  CODEX_WORKING_DIRECTORY: z.string().optional().default(""),
  CODEX_BIN: z.string().optional().default(""),

  GATEWAY_API_BASE_URL: z.string().url().default("https://api.openai.com/v1"),
  GATEWAY_API_KEY: z.string().optional().default(""),
  GATEWAY_API_MODEL: z.string().default("gpt-4o-mini"),
  GATEWAY_API_EXTRA_HEADER_NAME: z.string().optional().default(""),
  GATEWAY_API_EXTRA_HEADER_VALUE: z.string().optional().default(""),

  SESSION_TTL_HOURS: z.coerce.number().positive().default(2),
  BOT_SYSTEM_PROMPT: z
    .string()
    .default(
      "You are my personal exclusive assistant. Be concise, practical, and clear. If you are unsure, say what is missing instead of guessing.",
    ),
});

export type AppConfig = Omit<z.infer<typeof envSchema>, "MESSAGING_CHANNELS" | "MODEL_CHANNEL"> & {
  MESSAGING_CHANNELS: MessagingPlatformId[];
  MODEL_CHANNEL: ModelChannelId;
};

function parseMessagingChannels(raw: string): MessagingPlatformId[] {
  const parts = raw
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);
  if (parts.length === 0) {
    throw new Error("MESSAGING_CHANNELS is empty");
  }
  return parts.map((p) => messagingPlatformSchema.parse(p));
}

export function loadConfig(): AppConfig {
  const parsed = envSchema.safeParse(process.env);
  if (!parsed.success) {
    const detail = parsed.error.issues
      .map((i) => `${i.path.join(".")}: ${i.message}`)
      .join("\n");
    throw new Error(`Invalid environment:\n${detail}`);
  }

  const data = parsed.data;
  const messagingChannels = parseMessagingChannels(data.MESSAGING_CHANNELS);

  if (messagingChannels.includes("feishu")) {
    if (!data.FEISHU_APP_ID || !data.FEISHU_APP_SECRET) {
      throw new Error(
        "FEISHU_APP_ID and FEISHU_APP_SECRET are required when MESSAGING_CHANNELS includes feishu",
      );
    }
  }

  if (messagingChannels.includes("qq")) {
    if (!data.QQ_APP_ID || !data.QQ_APP_SECRET) {
      throw new Error(
        "QQ_APP_ID and QQ_APP_SECRET are required when MESSAGING_CHANNELS includes qq",
      );
    }
  }

  if (data.MODEL_CHANNEL === "gateway_api" && !data.GATEWAY_API_KEY) {
    throw new Error("GATEWAY_API_KEY is required when MODEL_CHANNEL=gateway_api");
  }

  return {
    ...data,
    MESSAGING_CHANNELS: messagingChannels,
    MODEL_CHANNEL: data.MODEL_CHANNEL,
    CODEX_WORKING_DIRECTORY: data.CODEX_WORKING_DIRECTORY
      ? path.resolve(data.CODEX_WORKING_DIRECTORY)
      : process.cwd(),
  };
}
