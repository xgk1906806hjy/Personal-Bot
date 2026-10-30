/** Strip QQ mention tags like <@!openid> so the model sees the real question. */
export function cleanQqMessageText(content: string | undefined): string {
  if (!content) return "";
  return content
    .replace(/<@!?>[^>]+>/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}
