#!/usr/bin/env node
/**
 * Cursor SDK local agent helper for Personal Bot (Go cursor_local channel).
 * Reads one JSON object from stdin, writes one JSON object to stdout.
 *
 * Input:  { prompt, cwd, apiKey, model, agentId? }
 * Output: { ok, text, agentId, status, usage? } | { ok:false, error, retryable?, agentId? }
 *
 * SDK may log to stdout; we redirect process.stdout writes to stderr until the
 * final result line, so Go always sees a single JSON object on stdout.
 */
import { readFileSync } from "node:fs";

const realStdoutWrite = process.stdout.write.bind(process.stdout);
process.stdout.write = (chunk, encoding, cb) => {
  if (typeof encoding === "function") {
    cb = encoding;
    encoding = undefined;
  }
  return process.stderr.write(chunk, encoding, cb);
};

const { Agent, CursorAgentError } = await import("@cursor/sdk");

function readInput() {
  const raw = readFileSync(0, "utf8").trim();
  if (!raw) {
    throw new Error("empty stdin; expected JSON payload");
  }
  return JSON.parse(raw);
}

function usageFrom(result) {
  const u = result?.usage;
  if (!u) return undefined;
  const promptTokens = u.inputTokens ?? u.promptTokens ?? 0;
  const completionTokens = u.outputTokens ?? u.completionTokens ?? 0;
  const totalTokens = u.totalTokens ?? promptTokens + completionTokens;
  return { promptTokens, completionTokens, totalTokens };
}

function writeResult(out) {
  realStdoutWrite(JSON.stringify(out || { ok: false, error: "unknown error" }) + "\n");
}

async function main() {
  let exitCode = 0;
  let out;
  let agent;

  try {
    const input = readInput();
    const prompt = String(input.prompt ?? "").trim();
    const cwd = String(input.cwd ?? "").trim();
    const apiKey = String(input.apiKey ?? process.env.CURSOR_API_KEY ?? "").trim();
    const model = String(input.model ?? process.env.CURSOR_MODEL ?? "grok-4.7").trim();
    const prevAgentId = input.agentId ? String(input.agentId).trim() : "";

    if (!prompt) {
      out = { ok: false, error: "prompt is required" };
      exitCode = 1;
    } else if (!cwd) {
      out = { ok: false, error: "cwd is required" };
      exitCode = 1;
    } else if (!apiKey) {
      out = { ok: false, error: "apiKey / CURSOR_API_KEY is required" };
      exitCode = 1;
    } else {
      if (prevAgentId) {
        agent = await Agent.resume(prevAgentId, { apiKey });
      } else {
        agent = await Agent.create({
          apiKey,
          model: { id: model },
          local: { cwd },
        });
      }

      const run = await agent.send(prompt);
      const result = await run.wait();
      const id = agent.agentId || run.agentId || prevAgentId || "";

      if (result.status === "error") {
        out = {
          ok: false,
          error: result.error?.message || `run failed: ${result.id || "unknown"}`,
          agentId: id,
          status: "error",
        };
        exitCode = 2;
      } else {
        out = {
          ok: true,
          text: (result.result && String(result.result).trim()) || "(Cursor agent finished with empty text)",
          agentId: id,
          status: result.status || "finished",
          usage: usageFrom(result),
        };
      }
    }
  } catch (err) {
    exitCode = 1;
    if (err instanceof CursorAgentError) {
      out = {
        ok: false,
        error: err.message || String(err),
        retryable: Boolean(err.isRetryable),
      };
    } else {
      out = { ok: false, error: err?.message || String(err) };
    }
  } finally {
    if (agent) {
      try {
        if (typeof agent[Symbol.asyncDispose] === "function") {
          await agent[Symbol.asyncDispose]();
        } else if (typeof agent.close === "function") {
          await agent.close();
        }
      } catch {
        // ignore dispose errors
      }
    }
  }

  writeResult(out);
  process.exit(exitCode);
}

main();
