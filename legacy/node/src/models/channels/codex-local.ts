import { Codex } from "@openai/codex-sdk";
import type { AppConfig } from "../../config.js";
import type { ModelChannel, ModelRequest, ModelResponse } from "../types.js";

/**
 * Local Codex channel.
 * Spawns the Codex CLI via @openai/codex-sdk and continues threads by thread id.
 */
export class CodexLocalChannel implements ModelChannel {
  readonly id = "codex_local" as const;
  private readonly codex: Codex;
  private readonly systemPrompt: string;
  private readonly workingDirectory: string;

  constructor(config: AppConfig) {
    this.systemPrompt = config.BOT_SYSTEM_PROMPT;
    this.workingDirectory = config.CODEX_WORKING_DIRECTORY;
    this.codex = new Codex({
      ...(config.CODEX_BIN ? { codexPathOverride: config.CODEX_BIN } : {}),
      config: {
        // Keep analysis read-oriented by default; adjust when you need writes.
      },
    });
  }

  async chat(req: ModelRequest): Promise<ModelResponse> {
    const prompt = this.buildPrompt(req);

    const thread = req.conversationId
      ? this.codex.resumeThread(req.conversationId, {
          workingDirectory: this.workingDirectory,
        })
      : this.codex.startThread({
          workingDirectory: this.workingDirectory,
        });

    const turn = await thread.run(prompt);
    const conversationId = thread.id ?? req.conversationId;

    return {
      channel: this.id,
      text: turn.finalResponse?.trim() || "(Codex returned empty response)",
      conversationId: conversationId ?? undefined,
      raw: {
        items: turn.items,
        usage: turn.usage,
      },
    };
  }

  private buildPrompt(req: ModelRequest): string {
    const history = req.messages
      .filter((m) => m.role !== "system")
      .map((m) => `${m.role === "user" ? "User" : "Assistant"}: ${m.content}`)
      .join("\n\n");

    return [
      this.systemPrompt,
      "",
      "Conversation context (if any). Answer the latest user message.",
      history || `User: ${req.prompt}`,
      "",
      `Working directory: ${this.workingDirectory}`,
    ].join("\n");
  }
}
