package datastore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	keyFileName = "secret.key"
	encPrefix   = "enc:"
)

// loadOrCreateKey reads the AES-256 encryption key from the given directory.
// If the key file does not exist, a new random 32-byte key is generated and saved.
// The key file is created with permission 0600 (owner read/write only).
func loadOrCreateKey(keyDir string) ([]byte, error) {
	keyPath := filepath.Join(keyDir, keyFileName)

	data, err := os.ReadFile(keyPath)
	if err == nil && len(data) == 32 {
		return data, nil
	}

	// Generate a new random 32-byte key (AES-256)
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate encryption key: %w", err)
	}

	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		return nil, fmt.Errorf("failed to write encryption key to %s: %w", keyPath, err)
	}

	return key, nil
}

// encryptField encrypts a plaintext string using AES-256-GCM and returns an
// "enc:" prefixed base64-encoded ciphertext. Empty strings are returned as-is.
func encryptField(key []byte, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	// If already encrypted, skip re-encryption.
	if strings.HasPrefix(plaintext, encPrefix) {
		return plaintext, nil
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes cipher error: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm error: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nonce generation error: %w", err)
	}

	// Seal appends ciphertext + tag to nonce
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return encPrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptField decrypts an "enc:"-prefixed value produced by encryptField.
// If the value does not have the prefix (e.g. legacy plain-text), it is returned
// as-is to maintain backward compatibility.
func decryptField(key []byte, value string) string {
	if value == "" || !strings.HasPrefix(value, encPrefix) {
		// Legacy plain-text value — return unchanged.
		return value
	}

	encoded := strings.TrimPrefix(value, encPrefix)
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		// Corrupted value — return as-is rather than losing the key.
		return value
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return value
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return value
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return value
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		// Authentication failure — return as-is.
		return value
	}

	return string(plaintext)
}
