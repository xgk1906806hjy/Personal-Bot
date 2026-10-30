import { loadConfig } from "./config.js";
import { createHttpApp } from "./http/app.js";
import { createMessagingAdapters, listReadyMessagingPlatforms } from "./messaging/registry.js";
import { createModelChannel, listModelChannels } from "./models/registry.js";
import { Orchestrator } from "./orchestrator.js";
import { SessionStore } from "./session/store.js";

async function main() {
  const config = loadConfig();
  const model = createModelChannel(config.MODEL_CHANNEL, config);
  const sessions = new SessionStore(config.SESSION_TTL_HOURS);
  const orchestrator = new Orchestrator(config, model, sessions);
  const messaging = createMessagingAdapters(config.MESSAGING_CHANNELS, config);
  const app = createHttpApp(config, orchestrator, messaging);

  app.listen(config.PORT, config.HOST, () => {
    console.info(
      `[boot] listening on http://${config.HOST}:${config.PORT}`,
    );
    console.info(
      `[boot] messaging = ${config.MESSAGING_CHANNELS.join(", ")} (ready: ${listReadyMessagingPlatforms().join(", ")})`,
    );
    console.info(`[boot] model = ${config.MODEL_CHANNEL}`);
    console.info(`[boot] model plugins = ${listModelChannels().join(", ")}`);
    console.info(`[boot] health = /healthz`);
  });
}

main().catch((err) => {
  console.error("[boot] fatal", err);
  process.exit(1);
});
