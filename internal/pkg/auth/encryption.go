package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"

	"golang.org/x/crypto/argon2"
)

// EncryptionService handles encryption and decryption of sensitive data
type EncryptionService struct {
	key []byte
}

// Key derivation parameters for Argon2id
const (
	// Argon2id parameters - these provide strong security while remaining performant
	// Time parameter (iterations): Number of passes over memory
	argon2Time = 3
	// Memory parameter: Amount of memory to use in KiB (64 MB)
	argon2Memory = 64 * 1024
	// Threads parameter: Number of parallel threads
	argon2Threads = 4
	// Key length: 32 bytes for AES-256
	argon2KeyLen = 32
	
	// Static versioned salt for consistent key derivation
	// In production, this could be environment-specific or derived from app version
	// Using a versioned approach allows for future salt rotation if needed
	saltVersion = "v1"
	staticSalt = "academy-sync-encryption-salt-" + saltVersion + "-2025"
)

// NewEncryptionService creates a new encryption service with a securely derived key
func NewEncryptionService(secret string) *EncryptionService {
	// Use Argon2id to derive a strong key from the secret
	// This provides resistance against brute-force attacks even with low-entropy inputs
	key := argon2.IDKey(
		[]byte(secret),
		[]byte(staticSalt),
		argon2Time,
		argon2Memory,
		argon2Threads,
		argon2KeyLen,
	)
	
	return &EncryptionService{
		key: key,
	}
}

// Encrypt encrypts plaintext data and returns the encrypted bytes
func (e *EncryptionService) Encrypt(plaintext string) ([]byte, error) {
	if plaintext == "" {
		return nil, nil // Return nil for empty strings
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, err
	}

	// Create a GCM cipher mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Create a nonce (number used once)
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Encrypt the data
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return ciphertext, nil
}

// Decrypt decrypts encrypted data and returns the plaintext
func (e *EncryptionService) Decrypt(ciphertext []byte) (string, error) {
	if len(ciphertext) == 0 {
		return "", nil // Return empty string for nil/empty bytes
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}

	// Extract nonce and ciphertext
	nonce := ciphertext[:gcm.NonceSize()]
	ciphertext = ciphertext[gcm.NonceSize():]

	// Decrypt the data
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}