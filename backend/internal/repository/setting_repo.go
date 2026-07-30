package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"

	"filebox/internal/entity"
)

// SettingRepository accesses the settings table.
type SettingRepository struct {
	db *sqlx.DB
}

// NewSettingRepository creates a SettingRepository.
func NewSettingRepository(db *sqlx.DB) *SettingRepository {
	return &SettingRepository{db: db}
}

// Get returns a setting by key.
func (r *SettingRepository) Get(ctx context.Context, key string) (*entity.Setting, error) {
	var s entity.Setting
	query := "SELECT * FROM settings WHERE `key` = ?"
	err := r.db.GetContext(ctx, &s, query, key)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// List returns all settings.
func (r *SettingRepository) List(ctx context.Context) ([]entity.Setting, error) {
	var settings []entity.Setting
	err := r.db.SelectContext(ctx, &settings, "SELECT * FROM settings ORDER BY `key`")
	if err != nil {
		return nil, err
	}
	return settings, nil
}

// Set inserts or updates a setting value.
func (r *SettingRepository) Set(ctx context.Context, key, value string) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO settings (`key`, value) VALUES (?, ?) ON DUPLICATE KEY UPDATE value = VALUES(value)",
		key, value)
	if err != nil {
		return fmt.Errorf("upsert setting: %w", err)
	}
	return nil
}
