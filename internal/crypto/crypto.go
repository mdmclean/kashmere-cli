// internal/crypto/crypto.go
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/pbkdf2"
)

const (
	keyLen      = 32 // AES-256
	ivLen       = 12 // GCM nonce
	saltLen     = 32
	pbkdf2Iters = 600_000
	authTagLen  = 16
)

// DeriveKey derives a 32-byte AES-256 key from passphrase+salt using
// PBKDF2-SHA512 with 600,000 iterations — matches packages/shared-crypto.
func DeriveKey(passphrase string, salt []byte) ([]byte, error) {
	if len(salt) != saltLen {
		return nil, fmt.Errorf("salt must be %d bytes, got %d", saltLen, len(salt))
	}
	key := pbkdf2.Key([]byte(passphrase), salt, pbkdf2Iters, keyLen, sha512.New)
	return key, nil
}

// Encrypt encrypts plaintext with AES-256-GCM and returns
// base64(iv || ciphertext || authTag) — matches packages/shared-crypto wire format.
func Encrypt(key []byte, plaintext string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	iv := make([]byte, ivLen)
	if _, err := rand.Read(iv); err != nil {
		return "", err
	}

	// GCM Seal appends ciphertext+authTag
	sealed := gcm.Seal(nil, iv, []byte(plaintext), nil)
	combined := make([]byte, ivLen+len(sealed))
	copy(combined, iv)
	copy(combined[ivLen:], sealed)

	return base64.StdEncoding.EncodeToString(combined), nil
}

// Decrypt decrypts a base64(iv || ciphertext || authTag) string.
func Decrypt(key []byte, ciphertextBase64 string) (string, error) {
	combined, err := base64.StdEncoding.DecodeString(ciphertextBase64)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}
	if len(combined) < ivLen+authTagLen {
		return "", fmt.Errorf("ciphertext too short")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	iv := combined[:ivLen]
	ciphertextWithTag := combined[ivLen:]

	plaintext, err := gcm.Open(nil, iv, ciphertextWithTag, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w (wrong passphrase?)", err)
	}
	return string(plaintext), nil
}

// SaltToBase64 encodes a salt byte slice to base64 (matches TypeScript btoa).
func SaltToBase64(salt []byte) string {
	return base64.StdEncoding.EncodeToString(salt)
}

// SaltFromBase64 decodes a base64 salt string.
func SaltFromBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
