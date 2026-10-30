import express from "express";
import * as lark from "@larksuiteoapi/node-sdk";
import type {
  MessagingAdapter,
  MessagingAdapterContext,
} from "../../types.js";
import { createFeishuClient, createFeishuEventDispatcher } from "./client.js";
import { replyText } from "./reply.js";
import { extractMessageText } from "./text.js";

/** Dedup Feishu retries within process lifetime */
const processedMessageIds = new Set<string>();

export class FeishuAdapter implements MessagingAdapter {
  readonly id = "feishu" as const;

  mount(app: express.Express, ctx: MessagingAdapterContext): void {
    const { config, orchestrator } = ctx;
    const client = createFeishuClient(config);
    const dispatcher = createFeishuEventDispatcher(config);

    dispatcher.register({
      "im.message.receive_v1": async (data) => {
        const messageId = data.message?.message_id;
        const chatId = data.message?.chat_id;
        const userId = data.sender?.sender_id?.open_id;
        const messageType = data.message?.message_type ?? "text";
        const contentRaw = data.message?.content ?? "{}";

        if (!messageId || !chatId || !userId) {
          console.warn("[feishu] missing message/chat/user id, skip");
          return;
        }

        if (processedMessageIds.has(messageId)) {
          console.info(`[feishu] duplicate message ${messageId}, skip`);
          return;
        }
        processedMessageIds.add(messageId);
        if (processedMessageIds.size > 5000) {
          const first = processedMessageIds.values().next().value;
          if (first) processedMessageIds.delete(first);
        }

        const text = extractMessageText(contentRaw, messageType);
        if (!text) {
          console.info(`[feishu] empty text after mention strip, skip ${messageId}`);
          return;
        }

        console.info(
          `[feishu] recv chat=${chatId} user=${userId} model=${orchestrator.modelChannelId} text=${text.slice(0, 80)}`,
        );

        try {
          await replyText(client, messageId, "Got it — thinking…");
          const answer = await orchestrator.handle({
            platform: "feishu",
            chatId,
            userId,
            messageId,
            text,
          });
          await replyText(client, messageId, answer);
        } catch (err) {
          const msg = err instanceof Error ? err.message : String(err);
          console.error(`[feishu] handle failed: ${msg}`);
          try {
            await replyText(client, messageId, `Failed: ${msg}`);
          } catch (replyErr) {
            console.error("[feishu] failed to send error reply", replyErr);
          }
        }
      },
    });

    const handler = lark.adaptExpress(dispatcher, { autoChallenge: true });
    app.use("/feishu/webhook", express.json(), (req, res, next) => {
      Promise.resolve(handler(req, res)).catch(next);
    });

    console.info("[boot] messaging adapter mounted: feishu → POST /feishu/webhook");
  }
}
