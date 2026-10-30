package feishu

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"errors"
)

func decryptEncryptString(encryptKey, cryptoText string) ([]byte, error) {
	if encryptKey == "" {
		return nil, errors.New("encrypt key empty")
	}
	sum := sha256.Sum256([]byte(encryptKey))
	key := sum[:]

	ciphertext, err := base64.StdEncoding.DecodeString(cryptoText)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	iv := ciphertext[:aes.BlockSize]
	data := ciphertext[aes.BlockSize:]
	if len(data)%aes.BlockSize != 0 {
		return nil, errors.New("ciphertext is not a multiple of the block size")
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(data, data)

	// Trim PKCS7 padding and find JSON object bounds.
	left := -1
	right := -1
	for i, b := range data {
		if b == '{' && left < 0 {
			left = i
		}
		if b == '}' {
			right = i
		}
	}
	if left < 0 || right < left {
		return nil, errors.New("json object not found in decrypted payload")
	}
	return data[left : right+1], nil
}
