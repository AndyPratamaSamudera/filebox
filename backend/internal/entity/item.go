package entity

import "time"

// ItemType distinguishes files from folders in the unified items table.
type ItemType string

const (
	ItemTypeFile   ItemType = "file"
	ItemTypeFolder ItemType = "folder"
)

// Item maps to the unified items table. A single table stores both files and
// folders; nullable columns are only meaningful for files.
type Item struct {
	ID            uint64    `db:"id" json:"id"`
	UserID        uint64    `db:"user_id" json:"-"`
	ParentID      *uint64   `db:"parent_id" json:"parent_id,omitempty"`
	Type          ItemType  `db:"type" json:"type"`
	Name          string    `db:"name" json:"name"`
	Ext           *string   `db:"ext" json:"ext,omitempty"`
	Path          string    `db:"path" json:"path"`
	MIME          *string   `db:"mime" json:"mime,omitempty"`
	Size          uint64    `db:"size" json:"size,omitempty"`
	StoragePath   string    `db:"storage_path" json:"-"`
	Checksum      *string   `db:"checksum" json:"checksum,omitempty"`
	IsChunked     bool      `db:"is_chunked" json:"is_chunked,omitempty"`
	ChunkCount    *int      `db:"chunk_count" json:"chunk_count,omitempty"`
	ChunkSize     *int      `db:"chunk_size" json:"chunk_size,omitempty"`
	IsFavorite    bool      `db:"is_favorite" json:"is_favorite"`
	PasswordHash  *string   `db:"password_hash" json:"-"`
	IsLocked      bool      `db:"-" json:"is_locked"`
	ShareCount    int       `db:"-" json:"share_count,omitempty"`
	EncryptionIV  *string   `db:"encryption_iv" json:"encryption_iv,omitempty"`
	EncryptionTag *string   `db:"encryption_tag" json:"encryption_tag,omitempty"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time `db:"updated_at" json:"-"`
}

// SetLocked sets IsLocked based on the stored password hash.
func (i *Item) SetLocked() {
	i.IsLocked = i.PasswordHash != nil && *i.PasswordHash != ""
}

// FullName returns name + ext for files; for folders it returns just name.
func (i *Item) FullName() string {
	if i.Type == ItemTypeFile && i.Ext != nil && *i.Ext != "" {
		return i.Name + *i.Ext
	}
	return i.Name
}
