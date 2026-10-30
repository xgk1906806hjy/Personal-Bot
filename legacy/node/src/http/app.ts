import express from "express";
import type { AppConfig } from "../config.js";
import type { MessagingAdapter } from "../messaging/types.js";
import { listMessagingPlatforms, listReadyMessagingPlatforms } from "../messaging/registry.js";
import { listModelChannels } from "../models/registry.js";
import type { Orchestrator } from "../orchestrator.js";

export function createHttpApp(
  config: AppConfig,
  orchestrator: Orchestrator,
  messagingAdapters: MessagingAdapter[],
): express.Express {
  const app = express();

  app.get("/healthz", (_req, res) => {
    res.json({
      ok: true,
      messaging: {
        enabled: messagingAdapters.map((a) => a.id),
        ready: listReadyMessagingPlatforms(),
        planned: listMessagingPlatforms(),
      },
      model: {
        active: orchestrator.modelChannelId,
        available: listModelChannels(),
      },
    });
  });

  for (const adapter of messagingAdapters) {
    adapter.mount(app, { config, orchestrator });
  }

  app.use(express.json());

  app.use(
    (
      err: unknown,
      _req: express.Request,
      res: express.Response,
      _next: express.NextFunction,
    ) => {
      const msg = err instanceof Error ? err.message : String(err);
      console.error("[http]", msg);
      res.status(500).json({ error: msg });
    },
  );

  return app;
}
