package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

// PayloadCipher encrypts and decrypts sensitive tokens stored inside transactional outbox JSON payloads
// using AES-256-GCM so plaintext one-time tokens never sit at rest in outbox_events.
type PayloadCipher struct {
	aead cipher.AEAD
}

// NewPayloadCipher derives a 256-bit AES-GCM key from secret using SHA-256.
func NewPayloadCipher(secret string) (*PayloadCipher, error) {
	if len(secret) == 0 {
		return nil, fmt.Errorf("payload cipher secret must not be empty")
	}
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("init aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("init aes-gcm: %w", err)
	}
	return &PayloadCipher{aead: aead}, nil
}

// Encrypt encrypts plaintext with a fresh random nonce and returns a base64url string.
func (c *PayloadCipher) Encrypt(plaintext string) (string, error) {
	if c == nil || c.aead == nil {
		return "", fmt.Errorf("payload cipher is nil")
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate gcm nonce: %w", err)
	}
	sealed := c.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

// Decrypt decodes and authenticates a base64url AES-256-GCM ciphertext.
func (c *PayloadCipher) Decrypt(ciphertextB64 string) (string, error) {
	if c == nil || c.aead == nil {
		return "", fmt.Errorf("payload cipher is nil")
	}
	raw, err := base64.RawURLEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	nonceSize := c.aead.NonceSize()
	if len(raw) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ct := raw[:nonceSize], raw[nonceSize:]
	pt, err := c.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt ciphertext: %w", err)
	}
	return string(pt), nil
}
