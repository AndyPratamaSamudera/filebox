package entity

import "time"

// APIKey maps to the api_keys table and stores a long-lived access credential.
type APIKey struct {
	ID          uint64     `db:"id" json:"id"`
	UserID      uint64     `db:"user_id" json:"-"`
	Name        string     `db:"name" json:"name"`
	KeyHash     string     `db:"key_hash" json:"-"`
	Key         string     `db:"key" json:"-"`
	Permissions []byte     `db:"permissions" json:"-"`
	ExpiresAt   *time.Time `db:"expires_at" json:"-"`
	LastUsedAt  *time.Time `db:"last_used_at" json:"last_used_at,omitempty"`
	Revoked     bool       `db:"revoked" json:"-"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at" json:"-"`
}
