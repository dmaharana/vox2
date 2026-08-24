package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	// Prefix indicates that a stored value is encrypted with v1 scheme.
	Prefix = "enc:v1:"
	// KeySize is 32 bytes for AES-256.
	KeySize = 32
	// NonceSize is 12 bytes for GCM.
	NonceSize = 12
)

var (
	keyMu     sync.RWMutex
	globalKey []byte
)

// GetEncryptionKey returns the active 32-byte encryption key.
// It checks environment variables ENCRYPTION_KEY / HARNESS_ENCRYPTION_KEY,
// then falls back to a persistent key file in data/.encryption_key,
// or generates/persists a new random key.
func GetEncryptionKey() []byte {
	keyMu.RLock()
	if len(globalKey) == KeySize {
		defer keyMu.RUnlock()
		return globalKey
	}
	keyMu.RUnlock()

	keyMu.Lock()
	defer keyMu.Unlock()

	if len(globalKey) == KeySize {
		return globalKey
	}

	// 1. Check environment variables
	for _, envVar := range []string{"HARNESS_ENCRYPTION_KEY", "ENCRYPTION_KEY", "SECRET_KEY"} {
		if val := os.Getenv(envVar); strings.TrimSpace(val) != "" {
			h := sha256.Sum256([]byte(val))
			globalKey = h[:]
			return globalKey
		}
	}

	// 2. Check persistent key file
	keyPath := filepath.Join("data", ".encryption_key")
	if data, err := os.ReadFile(keyPath); err == nil && len(data) == KeySize {
		globalKey = data
		return globalKey
	}

	// 3. Generate a new random key and persist it to data/.encryption_key
	newKey := make([]byte, KeySize)
	if _, err := io.ReadFull(rand.Reader, newKey); err != nil {
		// Fallback deterministic fallback if rand fails
		h := sha256.Sum256([]byte("go-harness-default-key-fallback"))
		newKey = h[:]
	}

	_ = os.MkdirAll("data", 0700)
	_ = os.WriteFile(keyPath, newKey, 0600)

	globalKey = newKey
	return globalKey
}

// SetEncryptionKey manually overrides the active encryption key (useful for tests).
func SetEncryptionKey(key []byte) {
	keyMu.Lock()
	defer keyMu.Unlock()
	if len(key) == KeySize {
		globalKey = make([]byte, KeySize)
		copy(globalKey, key)
	} else {
		h := sha256.Sum256(key)
		globalKey = h[:]
	}
}

// IsEncrypted returns true if the string starts with the encryption prefix.
func IsEncrypted(val string) bool {
	return strings.HasPrefix(val, Prefix)
}

// Encrypt encrypts a plaintext string using AES-256-GCM and returns an enc:v1: formatted string.
func Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	// If already encrypted, return as is
	if IsEncrypted(plaintext) {
		return plaintext, nil
	}
	return EncryptWithKey(plaintext, GetEncryptionKey())
}

// Decrypt decrypts an enc:v1: formatted string. If the string is not encrypted,
// it returns the plaintext string as-is (supporting backward compatibility).
func Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	if !IsEncrypted(ciphertext) {
		// Legacy plaintext or unencrypted
		return ciphertext, nil
	}
	return DecryptWithKey(ciphertext, GetEncryptionKey())
}

// EncryptWithKey encrypts plaintext using AES-256-GCM with a specified 32-byte key.
func EncryptWithKey(plaintext string, key []byte) (string, error) {
	if len(key) != KeySize {
		h := sha256.Sum256(key)
		key = h[:]
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("crypto: failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: failed to create gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto: failed to generate nonce: %w", err)
	}

	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	encoded := base64.StdEncoding.EncodeToString(sealed)
	return Prefix + encoded, nil
}

// DecryptWithKey decrypts an enc:v1: ciphertext using AES-256-GCM with a specified key.
func DecryptWithKey(ciphertext string, key []byte) (string, error) {
	if !IsEncrypted(ciphertext) {
		return ciphertext, nil
	}

	if len(key) != KeySize {
		h := sha256.Sum256(key)
		key = h[:]
	}

	rawB64 := strings.TrimPrefix(ciphertext, Prefix)
	data, err := base64.StdEncoding.DecodeString(rawB64)
	if err != nil {
		return "", fmt.Errorf("crypto: invalid base64 payload: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("crypto: failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: failed to create gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("crypto: ciphertext payload too short")
	}

	nonce, actualCiphertext := data[:nonceSize], data[nonceSize:]
	plainBytes, err := gcm.Open(nil, nonce, actualCiphertext, nil)
	if err != nil {
		return "", fmt.Errorf("crypto: failed to decrypt payload (invalid key or tampered data): %w", err)
	}

	return string(plainBytes), nil
}
