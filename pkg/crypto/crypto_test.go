package crypto

import (
	"strings"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	key := []byte("01234567890123456789012345678901") // 32 bytes
	SetEncryptionKey(key)

	original := "sk-ant-api03-secret-key-123456789!@#$%"
	encrypted, err := Encrypt(original)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if !strings.HasPrefix(encrypted, Prefix) {
		t.Fatalf("Encrypted text missing prefix %s: %s", Prefix, encrypted)
	}
	if encrypted == original {
		t.Fatalf("Encrypted text is equal to plaintext")
	}

	decrypted, err := Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted != original {
		t.Fatalf("Expected '%s', got '%s'", original, decrypted)
	}
}

func TestBackwardCompatibilityPlaintext(t *testing.T) {
	plain := "legacy-unencrypted-api-key"
	decrypted, err := Decrypt(plain)
	if err != nil {
		t.Fatalf("Decrypt of plaintext failed: %v", err)
	}
	if decrypted != plain {
		t.Fatalf("Expected '%s', got '%s'", plain, decrypted)
	}
}

func TestEmptyString(t *testing.T) {
	enc, err := Encrypt("")
	if err != nil || enc != "" {
		t.Fatalf("Expected empty string, got '%s', err: %v", enc, err)
	}

	dec, err := Decrypt("")
	if err != nil || dec != "" {
		t.Fatalf("Expected empty string, got '%s', err: %v", dec, err)
	}
}

func TestInvalidCiphertext(t *testing.T) {
	_, err := Decrypt(Prefix + "invalid-base64-content!@#$")
	if err == nil {
		t.Fatalf("Expected error for invalid base64")
	}

	_, err = Decrypt(Prefix + "YWJj") // too short
	if err == nil {
		t.Fatalf("Expected error for short payload")
	}
}

func TestCustomKey(t *testing.T) {
	key1 := []byte("key1-12345678901234567890123456")
	key2 := []byte("key2-12345678901234567890123456")

	secret := "super-confidential-token"
	enc, err := EncryptWithKey(secret, key1)
	if err != nil {
		t.Fatalf("EncryptWithKey failed: %v", err)
	}

	// Decrypting with wrong key must fail
	_, err = DecryptWithKey(enc, key2)
	if err == nil {
		t.Fatalf("Expected decryption failure with wrong key")
	}

	// Decrypting with correct key must succeed
	dec, err := DecryptWithKey(enc, key1)
	if err != nil {
		t.Fatalf("DecryptWithKey failed: %v", err)
	}
	if dec != secret {
		t.Fatalf("Expected '%s', got '%s'", secret, dec)
	}
}
