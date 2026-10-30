import * as lark from "@larksuiteoapi/node-sdk";
import type { AppConfig } from "../../../config.js";

export function createFeishuClient(config: AppConfig): lark.Client {
  return new lark.Client({
    appId: config.FEISHU_APP_ID,
    appSecret: config.FEISHU_APP_SECRET,
    appType: lark.AppType.SelfBuild,
    domain: lark.Domain.Feishu,
  });
}

export function createFeishuEventDispatcher(config: AppConfig): lark.EventDispatcher {
  return new lark.EventDispatcher({
    encryptKey: config.FEISHU_ENCRYPT_KEY || undefined,
    verificationToken: config.FEISHU_VERIFICATION_TOKEN || undefined,
  });
}
