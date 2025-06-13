package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
)

// EncryptionService handles encryption and decryption of sensitive data
type EncryptionService struct {
	key []byte
}

// NewEncryptionService creates a new encryption service with a derived key
func NewEncryptionService(secret string) *EncryptionService {
	// Use SHA256 to create a consistent 32-byte key from the secret
	hash := sha256.Sum256([]byte(secret))
	return &EncryptionService{
		key: hash[:],
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