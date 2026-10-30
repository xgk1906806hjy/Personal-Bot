import type { Orchestrator } from "../../../orchestrator.js";
import type { QqApiClient, QqReplyTarget } from "./client.js";
import { cleanQqMessageText } from "./text.js";

/** Dedup QQ retries / WS replays within process lifetime */
const processedMessageIds = new Set<string>();

export interface ParsedInbound {
  messageId: string;
  chatId: string;
  userId: string;
  text: string;
  target: QqReplyTarget;
}

export function parseInboundEvent(
  eventType: string,
  d: Record<string, unknown>,
): ParsedInbound | null {
  if (eventType === "GROUP_AT_MESSAGE_CREATE") {
    const messageId = String(d.id ?? "");
    const groupOpenid = String(d.group_openid ?? "");
    const author = (d.author ?? {}) as { member_openid?: string };
    const userId = String(author.member_openid ?? "");
    const text = cleanQqMessageText(typeof d.content === "string" ? d.content : "");
    if (!messageId || !groupOpenid || !userId) return null;
    return {
      messageId,
      chatId: groupOpenid,
      userId,
      text,
      target: { kind: "group", groupOpenid },
    };
  }

  if (eventType === "C2C_MESSAGE_CREATE") {
    const messageId = String(d.id ?? "");
    const author = (d.author ?? {}) as { user_openid?: string; id?: string };
    const userId = String(author.user_openid ?? author.id ?? "");
    const text = cleanQqMessageText(typeof d.content === "string" ? d.content : "");
    if (!messageId || !userId) return null;
    return {
      messageId,
      chatId: userId,
      userId,
      text,
      target: { kind: "c2c", userOpenid: userId },
    };
  }

  return null;
}

export async function handleQqInbound(
  eventType: string,
  data: Record<string, unknown>,
  client: QqApiClient,
  orchestrator: Orchestrator,
): Promise<void> {
  const parsed = parseInboundEvent(eventType, data);
  if (!parsed) {
    if (eventType && eventType !== "READY" && eventType !== "RESUMED") {
      console.info(`[qq] ignore event type=${eventType}`);
    }
    return;
  }

  if (processedMessageIds.has(parsed.messageId)) {
    console.info(`[qq] duplicate message ${parsed.messageId}, skip`);
    return;
  }
  processedMessageIds.add(parsed.messageId);
  if (processedMessageIds.size > 5000) {
    const first = processedMessageIds.values().next().value;
    if (first) processedMessageIds.delete(first);
  }

  if (!parsed.text) {
    console.info(`[qq] empty text, skip ${parsed.messageId}`);
    return;
  }

  console.info(
    `[qq] recv chat=${parsed.chatId} user=${parsed.userId} model=${orchestrator.modelChannelId} text=${parsed.text.slice(0, 80)}`,
  );

  try {
    await client.replyText(parsed.target, parsed.messageId, "Got it — thinking…");
    const answer = await orchestrator.handle({
      platform: "qq",
      chatId: parsed.chatId,
      userId: parsed.userId,
      messageId: parsed.messageId,
      text: parsed.text,
    });
    await client.replyText(parsed.target, parsed.messageId, answer);
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    console.error(`[qq] handle failed: ${msg}`);
    try {
      await client.replyText(parsed.target, parsed.messageId, `Failed: ${msg}`);
    } catch (replyErr) {
      console.error("[qq] failed to send error reply", replyErr);
    }
  }
}
