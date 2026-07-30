package entity

import "time"

// UserStatus enumerates account states.
type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusSuspended UserStatus = "suspended"
)

// UserRole enumerates account roles.
type UserRole string

const (
	UserRoleUser  UserRole = "user"
	UserRoleAdmin UserRole = "admin"
)

// User maps to the users table.
type User struct {
	ID           uint64     `db:"id" json:"id"`
	Email        string     `db:"email" json:"email"`
	Username     string     `db:"username" json:"username"`
	PasswordHash string     `db:"password_hash" json:"-"`
	PinHash      *string    `db:"pin_hash" json:"-"`
	Status       UserStatus `db:"status" json:"status"`
	Role         UserRole   `db:"role" json:"role"`
	StorageQuota uint64     `db:"storage_quota" json:"storage_quota"`
	TotalStorage uint64     `db:"total_storage" json:"total_storage"`
	StorageUsed  uint64     `db:"storage_used" json:"storage_used"`
	LastLoginAt  *time.Time `db:"last_login_at" json:"last_login_at,omitempty"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt    *time.Time `db:"deleted_at" json:"-"`
}
