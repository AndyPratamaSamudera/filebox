package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"filebox/internal/entity"
	"filebox/internal/repository"
	"filebox/internal/utils"
)

// APIKeyService manages API key lifecycle.
type APIKeyService struct {
	repo     *repository.APIKeyRepository
	userRepo *repository.UserRepository
}

// NewAPIKeyService creates an APIKeyService.
func NewAPIKeyService(repo *repository.APIKeyRepository, userRepo *repository.UserRepository) *APIKeyService {
	return &APIKeyService{repo: repo, userRepo: userRepo}
}

// Create generates a new API key, stores its hash and plaintext, and returns
// the plaintext key. The plaintext is stored so the user can reveal it again
// later after verifying their account password.
func (s *APIKeyService) Create(ctx context.Context, userID uint64, name string) (*entity.APIKey, string, error) {
	name = cleanName(name)
	if name == "" {
		return nil, "", NewUserError("name is required")
	}

	existing, err := s.repo.List(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	if len(existing) > 0 {
		return nil, "", NewUserError("only one active API key is allowed per user; revoke the existing key first")
	}

	plaintext, err := generateKey()
	if err != nil {
		return nil, "", fmt.Errorf("generate api key: %w", err)
	}

	k := &entity.APIKey{
		UserID:  userID,
		Name:    name,
		KeyHash: s.repo.HashKey(plaintext),
		Key:     plaintext,
	}
	created, err := s.repo.Create(ctx, k)
	if err != nil {
		return nil, "", err
	}
	return created, plaintext, nil
}

// List returns the user's active API keys.
func (s *APIKeyService) List(ctx context.Context, userID uint64) ([]entity.APIKey, error) {
	return s.repo.List(ctx, userID)
}

// Revoke disables an API key.
func (s *APIKeyService) Revoke(ctx context.Context, userID, id uint64) error {
	if _, err := s.repo.GetByID(ctx, id, userID); err != nil {
		return err
	}
	return s.repo.Revoke(ctx, id, userID)
}

// Reveal returns the plaintext API key after the user verifies their account
// password. This lets users view the same key multiple times without revoking
// and recreating it.
func (s *APIKeyService) Reveal(ctx context.Context, userID, id uint64, password string) (string, error) {
	if password == "" {
		return "", NewUserError("password is required")
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return "", err
	}
	if !utils.CheckPassword(password, user.PasswordHash) {
		return "", ErrInvalidCredentials
	}

	key, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		return "", err
	}
	if key.Revoked {
		return "", NewUserError("api key has been revoked")
	}
	return key.Key, nil
}

// Validate checks a raw API key and returns the owning user ID when valid.
func (s *APIKeyService) Validate(ctx context.Context, key string) (uint64, error) {
	k, err := s.repo.FindByKeyHash(ctx, s.repo.HashKey(key))
	if err != nil {
		return 0, err
	}
	if k.ExpiresAt != nil && k.ExpiresAt.Before(time.Now()) {
		return 0, repository.ErrNotFound
	}
	_ = s.repo.UpdateLastUsedAt(ctx, k.ID)
	return k.UserID, nil
}

func generateKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "fbx_" + hex.EncodeToString(b), nil
}

func cleanName(s string) string {
	out := []rune(s)
	if len(out) > 255 {
		out = out[:255]
	}
	return string(out)
}
