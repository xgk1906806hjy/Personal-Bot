package qq

import (
	"crypto/ed25519"
	"encoding/hex"
)

// expandBotSecretSeed repeats the bot secret until >= 32 bytes, then truncates.
// Matches the official QQ Go sample using ed25519.GenerateKey(strings.NewReader(seed)).
func expandBotSecretSeed(secret string) []byte {
	seed := secret
	for len(seed) < ed25519.SeedSize {
		seed += seed
	}
	return []byte(seed[:ed25519.SeedSize])
}

func privateKeyFromSecret(secret string) ed25519.PrivateKey {
	return ed25519.NewKeyFromSeed(expandBotSecretSeed(secret))
}

func publicKeyFromSecret(secret string) ed25519.PublicKey {
	return privateKeyFromSecret(secret).Public().(ed25519.PublicKey)
}

// SignValidationPayload signs event_ts + plain_token for webhook URL validation (op=13).
func SignValidationPayload(secret, eventTs, plainToken string) string {
	sig := ed25519.Sign(privateKeyFromSecret(secret), []byte(eventTs+plainToken))
	return hex.EncodeToString(sig)
}

// VerifyCallbackSignature verifies X-Signature-Ed25519 over (timestamp + rawBody).
func VerifyCallbackSignature(secret, timestamp string, rawBody []byte, signatureHex string) bool {
	sig, err := hex.DecodeString(signatureHex)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return false
	}
	msg := make([]byte, 0, len(timestamp)+len(rawBody))
	msg = append(msg, timestamp...)
	msg = append(msg, rawBody...)
	return ed25519.Verify(publicKeyFromSecret(secret), msg, sig)
}
