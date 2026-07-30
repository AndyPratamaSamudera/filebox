package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"filebox/internal/entity"
)

// ChunkUploadRepository accesses the chunk_uploads table.
type ChunkUploadRepository struct {
	db *sqlx.DB
}

// NewChunkUploadRepository creates a ChunkUploadRepository.
func NewChunkUploadRepository(db *sqlx.DB) *ChunkUploadRepository {
	return &ChunkUploadRepository{db: db}
}

// Create inserts a new chunked upload session.
func (r *ChunkUploadRepository) Create(ctx context.Context, cu *entity.ChunkUpload) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO chunk_uploads (id, user_id, total_chunks, chunk_size, metadata)
		 VALUES (?, ?, ?, ?, ?)`,
		cu.ID, cu.UserID, cu.TotalChunks, cu.ChunkSize, cu.Metadata)
	if err != nil {
		return fmt.Errorf("insert chunk upload: %w", err)
	}
	return nil
}

// GetByID returns a session by ID.
func (r *ChunkUploadRepository) GetByID(ctx context.Context, id string) (*entity.ChunkUpload, error) {
	var cu entity.ChunkUpload
	err := r.db.GetContext(ctx, &cu, `SELECT * FROM chunk_uploads WHERE id = ?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &cu, nil
}

// GetByIDAndUser returns a session only if it belongs to the given user.
func (r *ChunkUploadRepository) GetByIDAndUser(ctx context.Context, id string, userID uint64) (*entity.ChunkUpload, error) {
	var cu entity.ChunkUpload
	err := r.db.GetContext(ctx, &cu,
		`SELECT * FROM chunk_uploads WHERE id = ? AND user_id = ?`, id, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &cu, nil
}

// MarkCompleted sets completed_at for the session.
func (r *ChunkUploadRepository) MarkCompleted(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE chunk_uploads SET completed_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

// Delete removes a session.
func (r *ChunkUploadRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM chunk_uploads WHERE id = ?`, id)
	return err
}
