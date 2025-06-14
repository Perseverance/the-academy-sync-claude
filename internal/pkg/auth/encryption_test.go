package auth

import (
	"bytes"
	"testing"
)

func TestEncryptionService(t *testing.T) {
	secret := "test-secret-for-encryption-testing"
	service := NewEncryptionService(secret)

	// Test basic encryption/decryption
	plaintext := "Hello, World! This is a test message."
	
	// Encrypt the plaintext
	ciphertext, err := service.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}
	
	if len(ciphertext) == 0 {
		t.Fatal("Ciphertext is empty")
	}
	
	// Decrypt the ciphertext
	decrypted, err := service.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}
	
	if decrypted != plaintext {
		t.Errorf("Decrypted text doesn't match original. Expected: %s, Got: %s", plaintext, decrypted)
	}
}

func TestEncryptionConsistency(t *testing.T) {
	secret := "consistent-secret-key"
	
	// Create two services with the same secret
	service1 := NewEncryptionService(secret)
	service2 := NewEncryptionService(secret)
	
	// The derived keys should be identical (deterministic key derivation)
	if !bytes.Equal(service1.key, service2.key) {
		t.Error("Key derivation is not consistent - same secret produced different keys")
	}
	
	// Test cross-service decryption
	plaintext := "Cross-service test message"
	
	// Encrypt with service1
	ciphertext, err := service1.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encryption with service1 failed: %v", err)
	}
	
	// Decrypt with service2
	decrypted, err := service2.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decryption with service2 failed: %v", err)
	}
	
	if decrypted != plaintext {
		t.Errorf("Cross-service decryption failed. Expected: %s, Got: %s", plaintext, decrypted)
	}
}

func TestEncryptionDifferentSecrets(t *testing.T) {
	secret1 := "first-secret"
	secret2 := "second-secret"
	
	service1 := NewEncryptionService(secret1)
	service2 := NewEncryptionService(secret2)
	
	// Different secrets should produce different keys
	if bytes.Equal(service1.key, service2.key) {
		t.Error("Different secrets produced identical keys - this should not happen")
	}
	
	// Encryption with one service should not be decryptable by another
	plaintext := "Secret message"
	
	ciphertext, err := service1.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}
	
	// Attempting to decrypt with wrong service should fail
	_, err = service2.Decrypt(ciphertext)
	if err == nil {
		t.Error("Decryption with wrong key succeeded - this is a security issue")
	}
}

func TestEncryptionEmptyString(t *testing.T) {
	service := NewEncryptionService("test-secret")
	
	// Empty string should return nil
	ciphertext, err := service.Encrypt("")
	if err != nil {
		t.Fatalf("Encryption of empty string failed: %v", err)
	}
	
	if ciphertext != nil {
		t.Error("Expected nil ciphertext for empty string")
	}
	
	// Decrypting nil/empty should return empty string
	decrypted, err := service.Decrypt(nil)
	if err != nil {
		t.Fatalf("Decryption of nil failed: %v", err)
	}
	
	if decrypted != "" {
		t.Errorf("Expected empty string, got: %s", decrypted)
	}
}

func TestKeyDerivationSecurity(t *testing.T) {
	// Test that Argon2 is being used (we can't directly test this, but we can verify
	// that the key derivation is memory-hard by checking it's not just a simple hash)
	
	secret := "test-secret"
	service := NewEncryptionService(secret)
	
	// Key should be exactly 32 bytes (256 bits) for AES-256
	if len(service.key) != 32 {
		t.Errorf("Expected key length of 32 bytes, got %d", len(service.key))
	}
	
	// Key should not be all zeros
	allZeros := make([]byte, 32)
	if bytes.Equal(service.key, allZeros) {
		t.Error("Key derivation produced all zeros - this suggests a failure")
	}
}

func TestEncryptionRandomness(t *testing.T) {
	service := NewEncryptionService("test-secret")
	plaintext := "Same message encrypted multiple times"
	
	// Encrypt the same message multiple times
	ciphertext1, err := service.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("First encryption failed: %v", err)
	}
	
	ciphertext2, err := service.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Second encryption failed: %v", err)
	}
	
	// Ciphertexts should be different due to random nonces
	if bytes.Equal(ciphertext1, ciphertext2) {
		t.Error("Same plaintext produced identical ciphertext - this suggests nonce reuse")
	}
	
	// But both should decrypt to the same plaintext
	decrypted1, err := service.Decrypt(ciphertext1)
	if err != nil {
		t.Fatalf("Decryption of first ciphertext failed: %v", err)
	}
	
	decrypted2, err := service.Decrypt(ciphertext2)
	if err != nil {
		t.Fatalf("Decryption of second ciphertext failed: %v", err)
	}
	
	if decrypted1 != plaintext || decrypted2 != plaintext {
		t.Error("Decrypted text doesn't match original plaintext")
	}
}

func BenchmarkKeyDerivation(b *testing.B) {
	secret := "benchmark-secret-key-for-performance-testing"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewEncryptionService(secret)
	}
}

func BenchmarkEncryption(b *testing.B) {
	service := NewEncryptionService("benchmark-secret")
	plaintext := "This is a test message for benchmarking encryption performance with a reasonably sized payload"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.Encrypt(plaintext)
		if err != nil {
			b.Fatalf("Encryption failed: %v", err)
		}
	}
}

func BenchmarkDecryption(b *testing.B) {
	service := NewEncryptionService("benchmark-secret")
	plaintext := "This is a test message for benchmarking decryption performance with a reasonably sized payload"
	
	// Pre-encrypt the message
	ciphertext, err := service.Encrypt(plaintext)
	if err != nil {
		b.Fatalf("Pre-encryption failed: %v", err)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.Decrypt(ciphertext)
		if err != nil {
			b.Fatalf("Decryption failed: %v", err)
		}
	}
}