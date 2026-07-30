package entity

import "time"

// SharedItem is an enriched view of a file shared with the current user.
type SharedItem struct {
	ID               uint64    `db:"id" json:"id"`
	ItemID           uint64    `db:"item_id" json:"item_id"`
	OwnerUserID      uint64    `db:"owner_user_id" json:"owner_user_id"`
	CreatedAt        time.Time `db:"created_at" json:"created_at"`
	ItemName         string    `db:"item_name" json:"item_name"`
	ItemPath         string    `db:"item_path" json:"item_path"`
	ItemExt          *string   `db:"item_ext" json:"item_ext,omitempty"`
	ItemMIME         *string   `db:"item_mime" json:"item_mime,omitempty"`
	ItemSize         uint64    `db:"item_size" json:"item_size"`
	ItemPasswordHash *string   `db:"item_password_hash" json:"-"`
	ItemIsLocked     bool      `db:"item_is_locked" json:"item_is_locked"`
	ItemStoragePath  string    `db:"item_storage_path" json:"-"`
	OwnerEmail       string    `db:"owner_email" json:"owner_email"`
	OwnerName        string    `db:"owner_name" json:"owner_name"`
}

// SetLocked sets ItemIsLocked based on the stored file password hash.
func (s *SharedItem) SetLocked() {
	s.ItemIsLocked = s.ItemPasswordHash != nil && *s.ItemPasswordHash != ""
}
