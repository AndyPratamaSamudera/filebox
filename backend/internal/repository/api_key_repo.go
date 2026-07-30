package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"

	"github.com/jmoiron/sqlx"

	"filebox/internal/entity"
)

// APIKeyRepository accesses the api_keys table.
type APIKeyRepository struct {
	db *sqlx.DB
}

// NewAPIKeyRepository creates an APIKeyRepository.
func NewAPIKeyRepository(db *sqlx.DB) *APIKeyRepository {
	return &APIKeyRepository{db: db}
}

func hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// Create inserts an API key row. The caller must supply the hashed key; the
// plaintext key is never stored.
func (r *APIKeyRepository) Create(ctx context.Context, k *entity.APIKey) (*entity.APIKey, error) {
	res, err := r.db.ExecContext(ctx,
		"INSERT INTO api_keys (user_id, name, key_hash, `key`, permissions, expires_at) VALUES (?, ?, ?, ?, ?, ?)",
		k.UserID, k.Name, k.KeyHash, k.Key, k.Permissions, k.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("insert api key: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, uint64(id), k.UserID)
}

// GetByID returns an API key owned by the user.
func (r *APIKeyRepository) GetByID(ctx context.Context, id, userID uint64) (*entity.APIKey, error) {
	var k entity.APIKey
	err := r.db.GetContext(ctx, &k,
		`SELECT * FROM api_keys WHERE id = ? AND user_id = ?`, id, userID)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// List returns the user's non-revoked API keys.
func (r *APIKeyRepository) List(ctx context.Context, userID uint64) ([]entity.APIKey, error) {
	var keys []entity.APIKey
	err := r.db.SelectContext(ctx, &keys,
		`SELECT id, user_id, name, permissions, expires_at, last_used_at, revoked, created_at, updated_at
		 FROM api_keys WHERE user_id = ? AND revoked = 0 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	return keys, nil
}

// FindByKeyHash looks up an API key by its hash. Used by auth middleware.
func (r *APIKeyRepository) FindByKeyHash(ctx context.Context, hash string) (*entity.APIKey, error) {
	var k entity.APIKey
	err := r.db.GetContext(ctx, &k,
		`SELECT * FROM api_keys WHERE key_hash = ? AND revoked = 0 LIMIT 1`, hash)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// UpdateLastUsedAt refreshes the last_used_at timestamp.
func (r *APIKeyRepository) UpdateLastUsedAt(ctx context.Context, id uint64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE api_keys SET last_used_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

// Revoke marks an API key as revoked.
func (r *APIKeyRepository) Revoke(ctx context.Context, id, userID uint64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE api_keys SET revoked = 1 WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// HashKey returns the SHA-256 hash of a raw API key.
func (r *APIKeyRepository) HashKey(key string) string {
	return hashKey(key)
}
