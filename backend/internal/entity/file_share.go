package entity

import "time"

// ItemShare records that a file has been shared with a specific user. In the
// server-side disk-encryption model the server can decrypt the file for any
// authorized recipient, so no wrapped key is needed.
type ItemShare struct {
	ID               uint64    `db:"id" json:"id"`
	ItemID           uint64    `db:"item_id" json:"item_id"`
	OwnerUserID      uint64    `db:"owner_user_id" json:"owner_user_id"`
	SharedWithUserID uint64    `db:"shared_with_user_id" json:"shared_with_user_id"`
	CreatedAt        time.Time `db:"created_at" json:"created_at"`

	// Recipient identity is populated by queries via JOIN to the users table.
	SharedEmail string `db:"shared_email" json:"shared_email,omitempty"`
	SharedName  string `db:"shared_name" json:"shared_name,omitempty"`
}
