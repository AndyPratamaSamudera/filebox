// Package encryption provides server-side AES-GCM-256 encryption for file bytes
// before they are written to disk. The key is supplied at startup via the
// FILEBOX_ENCRYPTION_KEY environment variable and is never stored in the database.
package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

const (
	nonceSize = 12 // AES-GCM standard nonce length
	tagSize   = 16 // AES-GCM authentication tag length
)

// Service wraps an AES-GCM cipher instantiated from the master key.
type Service struct {
	aead cipher.AEAD
}

// NewService creates an encryption service from a 32-byte AES-256 key.
func NewService(key []byte) (*Service, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}
	return &Service{aead: aead}, nil
}

// NewServiceFromEnv decodes FILEBOX_ENCRYPTION_KEY as base64, hex, or raw 32-byte string.
func NewServiceFromEnv(val string) (*Service, error) {
	if val == "" {
		return nil, errors.New("FILEBOX_ENCRYPTION_KEY is required")
	}

	// Try base64 first.
	key, err := base64.StdEncoding.DecodeString(val)
	if err == nil && len(key) == 32 {
		return NewService(key)
	}

	// Try hex.
	key, err = hex.DecodeString(val)
	if err == nil && len(key) == 32 {
		return NewService(key)
	}

	// Fall back to a raw 32-byte string.
	if len(val) == 32 {
		return NewService([]byte(val))
	}

	return nil, fmt.Errorf("FILEBOX_ENCRYPTION_KEY must decode to 32 bytes (base64/hex/raw); got %d bytes", len(val))
}

// EncryptBytes encrypts plaintext in one shot and returns
// combined = nonce || ciphertext || tag, plus the nonce and tag separately.
func (s *Service) EncryptBytes(plain []byte) (combined, nonce, tag []byte, err error) {
	nonce = make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, nil, fmt.Errorf("generate nonce: %w", err)
	}

	cipherText := s.aead.Seal(nil, nonce, plain, nil)
	// cipherText already includes the tag at the end.
	combined = make([]byte, nonceSize+len(cipherText))
	copy(combined, nonce)
	copy(combined[nonceSize:], cipherText)
	tag = cipherText[len(cipherText)-tagSize:]
	return combined, nonce, tag, nil
}

// DecryptBytes decrypts combined (nonce || ciphertext || tag) back to plaintext.
func (s *Service) DecryptBytes(combined []byte) ([]byte, error) {
	if len(combined) < nonceSize+tagSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce := combined[:nonceSize]
	cipherText := combined[nonceSize:]
	return s.aead.Open(nil, nonce, cipherText, nil)
}

// EncryptChunk is a convenience wrapper for chunk encryption.
func (s *Service) EncryptChunk(plain []byte) ([]byte, error) {
	combined, _, _, err := s.EncryptBytes(plain)
	return combined, err
}

// DecryptChunk decrypts a single chunk produced by EncryptChunk.
func (s *Service) DecryptChunk(combined []byte) ([]byte, error) {
	return s.DecryptBytes(combined)
}
