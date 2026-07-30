package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"filebox/internal/entity"
)

// ErrNotFound is returned when a single-row query matches nothing.
var ErrNotFound = errors.New("record not found")

// UserRepository accesses the users table.
type UserRepository struct {
	db *sqlx.DB
}

// NewUserRepository creates a UserRepository.
func NewUserRepository(db *sqlx.DB) *UserRepository {
	return &UserRepository{db: db}
}

// DB returns the underlying database handle.
func (r *UserRepository) DB() *sqlx.DB { return r.db }

// Create inserts a new user and returns the created row.
func (r *UserRepository) Create(ctx context.Context, email, username, passwordHash string, totalStorage uint64) (*entity.User, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO users (email, username, password_hash, total_storage) VALUES (?, ?, ?, ?)`,
		email, username, passwordHash, totalStorage)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("user last insert id: %w", err)
	}
	return r.GetByID(ctx, uint64(id))
}

// GetByEmail returns the active user with the given email.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*entity.User, error) {
	var u entity.User
	err := r.db.GetContext(ctx, &u,
		`SELECT * FROM users WHERE email = ? AND deleted_at IS NULL`, email)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetByUsername returns the active user with the given username.
func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*entity.User, error) {
	var u entity.User
	err := r.db.GetContext(ctx, &u,
		`SELECT * FROM users WHERE username = ? AND deleted_at IS NULL`, username)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetByID returns the active user by ID.
func (r *UserRepository) GetByID(ctx context.Context, id uint64) (*entity.User, error) {
	var u entity.User
	err := r.db.GetContext(ctx, &u,
		`SELECT * FROM users WHERE id = ? AND deleted_at IS NULL`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// UpdateLastLogin stamps the last login time for a user.
func (r *UserRepository) UpdateLastLogin(ctx context.Context, id uint64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET last_login_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

// AddStorageUsed adjusts a user's storage_used counter by delta (may be negative).
func (r *UserRepository) AddStorageUsed(ctx context.Context, userID, delta uint64, subtract bool) error {
	if subtract {
		_, err := r.db.ExecContext(ctx,
			`UPDATE users SET storage_used = GREATEST(0, CAST(storage_used AS SIGNED) - ?) WHERE id = ?`,
			delta, userID)
		return err
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET storage_used = storage_used + ? WHERE id = ?`, delta, userID)
	return err
}

// AddStorageUsedTx is the transaction-scoped variant of AddStorageUsed.
func (r *UserRepository) AddStorageUsedTx(ctx context.Context, tx *sqlx.Tx, userID, delta uint64, subtract bool) error {
	if subtract {
		_, err := tx.ExecContext(ctx,
			`UPDATE users SET storage_used = GREATEST(0, CAST(storage_used AS SIGNED) - ?) WHERE id = ?`,
			delta, userID)
		return err
	}
	_, err := tx.ExecContext(ctx,
		`UPDATE users SET storage_used = storage_used + ? WHERE id = ?`, delta, userID)
	return err
}
