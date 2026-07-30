package utils

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword generates a bcrypt hash of a password.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword compares a plaintext password against a bcrypt hash.
func CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// HashToken returns the SHA-256 hex digest of a token. Used to store refresh
// tokens safely: the DB only keeps the hash, never the raw token.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// HashFilePassword returns the SHA-512 hex digest of a file password.
// Per the FileBox spec, file passwords are stored hashed and never plaintext.
func HashFilePassword(password string) string {
	sum := sha512.Sum512([]byte(password))
	return hex.EncodeToString(sum[:])
}

// CheckFilePassword reports whether the plaintext matches the stored SHA-512 hash.
func CheckFilePassword(password, hash string) bool {
	return HashFilePassword(password) == hash
}
