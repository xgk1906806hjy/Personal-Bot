import type * as lark from "@larksuiteoapi/node-sdk";

export async function replyText(
  client: lark.Client,
  messageId: string,
  text: string,
): Promise<void> {
  const content = JSON.stringify({ text: truncate(text, 4000) });
  await client.im.message.reply({
    path: { message_id: messageId },
    data: {
      content,
      msg_type: "text",
    },
  });
}

function truncate(text: string, max: number): string {
  if (text.length <= max) return text;
  return `${text.slice(0, max - 20)}\n…(truncated)`;
}
