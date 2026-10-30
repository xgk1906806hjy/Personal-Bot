/**
 * Extract plain text from a Feishu IM message content JSON string.
 * Mentions like @_user_1 are stripped so the model sees the real question.
 */
export function extractMessageText(contentRaw: string, messageType: string): string {
  if (messageType !== "text") {
    return `[Unsupported message type: ${messageType}]`;
  }

  try {
    const parsed = JSON.parse(contentRaw) as { text?: string };
    const text = parsed.text ?? "";
    return text
      .replace(/@_user_\d+/g, " ")
      .replace(/@_\w+/g, " ")
      .replace(/\s+/g, " ")
      .trim();
  } catch {
    return contentRaw.trim();
  }
}
