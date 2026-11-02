package qq

import (
	"crypto/ed25519"
	"encoding/hex"
)

// expandBotSecretSeed 将 bot secret 重复拼接至至少 32 字节后截断。
// 与官方 QQ Go 示例一致：ed25519.GenerateKey(strings.NewReader(seed))。
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

// SignValidationPayload 对 event_ts + plain_token 签名，用于 webhook URL 校验（op=13）。
func SignValidationPayload(secret, eventTs, plainToken string) string {
	sig := ed25519.Sign(privateKeyFromSecret(secret), []byte(eventTs+plainToken))
	return hex.EncodeToString(sig)
}

// VerifyCallbackSignature 校验 X-Signature-Ed25519（签名内容为 timestamp + rawBody）。
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
