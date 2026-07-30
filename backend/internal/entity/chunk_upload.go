package entity

import "time"

// ChunkUpload tracks an in-progress chunked upload session. The actual chunk
// bytes live on disk under storage/chunks/<id>/<index>; this table stores the
// session metadata needed to validate and assemble them.
type ChunkUpload struct {
	ID          string     `db:"id" json:"id"`
	UserID      uint64     `db:"user_id" json:"user_id"`
	TotalChunks int        `db:"total_chunks" json:"total_chunks"`
	ChunkSize   int        `db:"chunk_size" json:"chunk_size"`
	Metadata    string     `db:"metadata" json:"metadata"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
	CompletedAt *time.Time `db:"completed_at" json:"completed_at,omitempty"`
}
