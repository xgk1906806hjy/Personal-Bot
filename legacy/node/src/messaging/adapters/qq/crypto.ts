import { createPrivateKey, createPublicKey, sign, verify } from "node:crypto";

/** PKCS8 prefix for a 32-byte Ed25519 seed (Node crypto). */
const ED25519_PKCS8_PREFIX = Buffer.from(
  "302e020100300506032b657004220420",
  "hex",
);

/**
 * QQ Bot Secret → 32-byte seed: repeat secret until >= 32 bytes, then truncate.
 * Matches the official Go sample using ed25519.GenerateKey(strings.NewReader(seed)).
 */
export function expandBotSecretSeed(secret: string): Buffer {
  let seed = secret;
  while (Buffer.byteLength(seed, "utf8") < 32) {
    seed += seed;
  }
  return Buffer.from(seed, "utf8").subarray(0, 32);
}

function privateKeyFromSecret(secret: string) {
  const seed = expandBotSecretSeed(secret);
  return createPrivateKey({
    key: Buffer.concat([ED25519_PKCS8_PREFIX, seed]),
    format: "der",
    type: "pkcs8",
  });
}

function publicKeyFromSecret(secret: string) {
  return createPublicKey(privateKeyFromSecret(secret));
}

/** Sign event_ts + plain_token for webhook URL validation (op=13). */
export function signValidationPayload(
  secret: string,
  eventTs: string,
  plainToken: string,
): string {
  const key = privateKeyFromSecret(secret);
  const msg = Buffer.from(`${eventTs}${plainToken}`, "utf8");
  return sign(null, msg, key).toString("hex");
}

/** Verify X-Signature-Ed25519 over (timestamp + rawBody). */
export function verifyCallbackSignature(
  secret: string,
  timestamp: string,
  rawBody: Buffer,
  signatureHex: string,
): boolean {
  try {
    const sig = Buffer.from(signatureHex, "hex");
    if (sig.length !== 64) return false;
    const key = publicKeyFromSecret(secret);
    const msg = Buffer.concat([Buffer.from(timestamp, "utf8"), rawBody]);
    return verify(null, msg, key, sig);
  } catch {
    return false;
  }
}
