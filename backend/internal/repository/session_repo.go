package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"filebox/internal/entity"
)

// SessionRepository accesses the sessions table (refresh token storage).
type SessionRepository struct {
	db *sqlx.DB
}

// NewSessionRepository creates a SessionRepository.
func NewSessionRepository(db *sqlx.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// Create inserts a new session record and returns its ID.
func (r *SessionRepository) Create(ctx context.Context, s *entity.Session) (uint64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO sessions (user_id, token_hash, user_agent, ip, expires_at)
		 VALUES (?, ?, ?, ?, ?)`,
		s.UserID, s.TokenHash, s.UserAgent, s.IP, s.ExpiresAt)
	if err != nil {
		return 0, fmt.Errorf("insert session: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return uint64(id), nil
}

// GetByTokenHash returns the session matching a refresh token hash.
func (r *SessionRepository) GetByTokenHash(ctx context.Context, hash string) (*entity.Session, error) {
	var s entity.Session
	err := r.db.GetContext(ctx, &s,
		`SELECT * FROM sessions WHERE token_hash = ?`, hash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// Revoke marks a session as revoked by its token hash.
func (r *SessionRepository) Revoke(ctx context.Context, hash string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE sessions SET revoked = 1 WHERE token_hash = ?`, hash)
	return err
}

// IsExpired reports whether the session is past its expiry.
func (r *SessionRepository) IsExpired(s *entity.Session) bool {
	return time.Now().After(s.ExpiresAt) || s.Revoked
}
