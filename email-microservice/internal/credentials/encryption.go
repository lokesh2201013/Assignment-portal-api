package credentials

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

const encryptedPrefix = "enc:v1:"

// ParseKey decodes a base64-encoded 32-byte AES-256 key.
func ParseKey(value string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("SMTP_CREDENTIALS_KEY must be base64-encoded: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("SMTP_CREDENTIALS_KEY must decode to exactly 32 bytes")
	}
	return key, nil
}

// Encrypt encrypts a credential using AES-256-GCM.
func Encrypt(key []byte, plaintext string) (string, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate credential nonce: %w", err)
	}
	sealed := aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return encryptedPrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt decrypts an AES-256-GCM credential written by Encrypt.
func Decrypt(key []byte, ciphertext string) (string, error) {
	if !strings.HasPrefix(ciphertext, encryptedPrefix) {
		return "", fmt.Errorf("SMTP credential is not encrypted")
	}
	sealed, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(ciphertext, encryptedPrefix))
	if err != nil {
		return "", fmt.Errorf("decode encrypted SMTP credential: %w", err)
	}
	aead, err := newAEAD(key)
	if err != nil {
		return "", err
	}
	if len(sealed) < aead.NonceSize() {
		return "", fmt.Errorf("encrypted SMTP credential is truncated")
	}
	nonce, ciphertextBytes := sealed[:aead.NonceSize()], sealed[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt SMTP credential: %w", err)
	}
	return string(plaintext), nil
}

func IsEncrypted(value string) bool {
	return strings.HasPrefix(value, encryptedPrefix)
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("invalid SMTP credentials encryption key: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("initialize SMTP credentials encryption: %w", err)
	}
	return aead, nil
}
