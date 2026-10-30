import type { AppConfig } from "../../config.js";
import type { ModelChannel, ModelRequest, ModelResponse } from "../types.js";

interface ChatCompletionResponse {
  id?: string;
  choices?: Array<{
    message?: { role?: string; content?: string | null };
  }>;
  error?: { message?: string };
}

/**
 * OpenAI-compatible gateway channel.
 * POST {base}/chat/completions — works with OpenAI, Azure-compatible, or private gateways.
 */
export class GatewayApiChannel implements ModelChannel {
  readonly id = "gateway_api" as const;

  private readonly baseUrl: string;
  private readonly apiKey: string;
  private readonly model: string;
  private readonly systemPrompt: string;
  private readonly extraHeaderName: string;
  private readonly extraHeaderValue: string;

  /** In-memory multi-turn history keyed by conversationId / sessionKey */
  private readonly histories = new Map<string, Array<{ role: string; content: string }>>();

  constructor(config: AppConfig) {
    this.baseUrl = config.GATEWAY_API_BASE_URL.replace(/\/$/, "");
    this.apiKey = config.GATEWAY_API_KEY;
    this.model = config.GATEWAY_API_MODEL;
    this.systemPrompt = config.BOT_SYSTEM_PROMPT;
    this.extraHeaderName = config.GATEWAY_API_EXTRA_HEADER_NAME;
    this.extraHeaderValue = config.GATEWAY_API_EXTRA_HEADER_VALUE;
  }

  async chat(req: ModelRequest): Promise<ModelResponse> {
    const conversationId = req.conversationId || req.sessionKey;
    const history = this.histories.get(conversationId) ?? [];

    const messages = [
      { role: "system", content: this.systemPrompt },
      ...history,
      { role: "user", content: req.prompt },
    ];

    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      Authorization: `Bearer ${this.apiKey}`,
    };
    if (this.extraHeaderName && this.extraHeaderValue) {
      headers[this.extraHeaderName] = this.extraHeaderValue;
    }

    const res = await fetch(`${this.baseUrl}/chat/completions`, {
      method: "POST",
      headers,
      body: JSON.stringify({
        model: this.model,
        messages,
        temperature: 0.2,
      }),
    });

    const data = (await res.json()) as ChatCompletionResponse;
    if (!res.ok) {
      const msg = data.error?.message || res.statusText || `HTTP ${res.status}`;
      throw new Error(`Gateway API error: ${msg}`);
    }

    const text = data.choices?.[0]?.message?.content?.trim();
    if (!text) {
      throw new Error("Gateway API returned empty content");
    }

    history.push({ role: "user", content: req.prompt });
    history.push({ role: "assistant", content: text });
    // Keep a bounded window
    const trimmed = history.slice(-20);
    this.histories.set(conversationId, trimmed);

    return {
      channel: this.id,
      text,
      conversationId,
      raw: data,
    };
  }
}
