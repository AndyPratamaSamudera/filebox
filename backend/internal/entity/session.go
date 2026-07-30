package entity

import "time"

// Session maps to the sessions table. It stores a SHA-256 hash of the refresh
// token so that raw refresh tokens never live in the database.
type Session struct {
	ID        uint64    `db:"id" json:"-"`
	UserID    uint64    `db:"user_id" json:"user_id"`
	TokenHash string    `db:"token_hash" json:"-"`
	UserAgent *string   `db:"user_agent" json:"-"`
	IP        *string   `db:"ip" json:"-"`
	ExpiresAt time.Time `db:"expires_at" json:"expires_at"`
	Revoked   bool      `db:"revoked" json:"revoked"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}
