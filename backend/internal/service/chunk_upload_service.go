package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"filebox/internal/encryption"
	"filebox/internal/entity"
	"filebox/internal/repository"
	"filebox/internal/storage"
	"filebox/internal/utils"
)

// ChunkFileMeta bundles the metadata for a chunked upload. It is stored as JSON
// in the chunk_uploads table so the finalize step can create the item row.
type ChunkFileMeta struct {
	Directory string `json:"directory"`
	Name      string `json:"name"`
	Ext       string `json:"ext"`
	MIME      string `json:"mime"`
	Password  string `json:"password,omitempty"`
}

// ChunkUploadService manages chunked upload sessions: create, receive chunks,
// and assemble them into a final file.
type ChunkUploadService struct {
	chunkRepo *repository.ChunkUploadRepository
	itemRepo  *repository.ItemRepository
	userRepo  *repository.UserRepository
	storage   *storage.LocalStorage
	enc       *encryption.Service
}

// NewChunkUploadService creates a ChunkUploadService.
func NewChunkUploadService(chunkRepo *repository.ChunkUploadRepository, itemRepo *repository.ItemRepository, userRepo *repository.UserRepository, st *storage.LocalStorage, enc *encryption.Service) *ChunkUploadService {
	return &ChunkUploadService{
		chunkRepo: chunkRepo,
		itemRepo:  itemRepo,
		userRepo:  userRepo,
		storage:   st,
		enc:       enc,
	}
}

// Init creates a new chunked upload session and returns its ID.
func (s *ChunkUploadService) Init(ctx context.Context, userID uint64, chunkSize, totalChunks int, totalSize int64, meta ChunkFileMeta) (string, error) {
	if chunkSize <= 0 {
		return "", NewUserError("chunk size must be positive")
	}
	if totalChunks <= 0 {
		return "", NewUserError("total chunks must be positive")
	}
	if totalSize <= 0 {
		return "", NewUserError("total size must be positive")
	}

	ext := meta.Ext
	if ext == "" {
		ext = filepath.Ext(meta.Name)
	}
	if ext == "" {
		return "", NewUserError("only files with an extension can be uploaded")
	}

	if _, err := resolveFolderForService(ctx, s.itemRepo, userID, meta.Directory); err != nil {
		return "", err
	}

	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return "", fmt.Errorf("marshal chunk metadata: %w", err)
	}

	uploadID := uuid.NewString()
	cu := &entity.ChunkUpload{
		ID:          uploadID,
		UserID:      userID,
		TotalChunks: totalChunks,
		ChunkSize:   chunkSize,
		Metadata:    string(metaJSON),
	}
	if err := s.chunkRepo.Create(ctx, cu); err != nil {
		return "", err
	}
	return uploadID, nil
}

// UploadChunk receives one plaintext chunk for a session, encrypts it, and
// stores the ciphertext on disk.
func (s *ChunkUploadService) UploadChunk(ctx context.Context, userID uint64, uploadID string, index int, content io.Reader) error {
	if index < 0 {
		return NewUserError("chunk index must be non-negative")
	}

	cu, err := s.chunkRepo.GetByIDAndUser(ctx, uploadID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return NewUserError("upload session not found")
		}
		return err
	}
	if cu.CompletedAt != nil {
		return NewUserError("upload session already completed")
	}
	if index >= cu.TotalChunks {
		return NewUserError("chunk index out of range")
	}

	plain, err := io.ReadAll(content)
	if err != nil {
		return fmt.Errorf("read chunk: %w", err)
	}
	if len(plain) == 0 {
		return NewUserError("chunk is empty")
	}

	cipher, err := s.enc.EncryptChunk(plain)
	if err != nil {
		return fmt.Errorf("encrypt chunk: %w", err)
	}

	if _, err := s.storage.SaveChunk(uploadID, index, bytes.NewReader(cipher)); err != nil {
		return err
	}
	return nil
}

// Status returns the list of chunk indexes that have been received.
func (s *ChunkUploadService) Status(ctx context.Context, userID uint64, uploadID string) (int, []int, error) {
	cu, err := s.chunkRepo.GetByIDAndUser(ctx, uploadID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return 0, nil, NewUserError("upload session not found")
		}
		return 0, nil, err
	}

	sizes, err := s.storage.ListChunkSizes(uploadID)
	if err != nil {
		return 0, nil, err
	}

	received := make([]int, 0, len(sizes))
	for i, size := range sizes {
		if size > 0 {
			received = append(received, i)
		}
	}
	return cu.TotalChunks, received, nil
}

// Complete verifies all chunks are present, assembles them into the final file,
// creates the item row, and cleans up the temporary chunk directory.
func (s *ChunkUploadService) Complete(ctx context.Context, userID uint64, uploadID string) (*entity.Item, error) {
	cu, err := s.chunkRepo.GetByIDAndUser(ctx, uploadID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, NewUserError("upload session not found")
		}
		return nil, err
	}
	if cu.CompletedAt != nil {
		return nil, NewUserError("upload session already completed")
	}

	var meta ChunkFileMeta
	if err := json.Unmarshal([]byte(cu.Metadata), &meta); err != nil {
		return nil, fmt.Errorf("decode chunk metadata: %w", err)
	}

	parent, err := resolveFolderForService(ctx, s.itemRepo, userID, meta.Directory)
	if err != nil {
		return nil, err
	}
	var parentID *uint64
	var parentPath string
	if parent != nil {
		parentID = &parent.ID
		parentPath = parent.Path
	}

	sizes, err := s.storage.ListChunkSizes(uploadID)
	if err != nil {
		return nil, err
	}
	if len(sizes) != cu.TotalChunks {
		return nil, NewUserError(fmt.Sprintf("missing chunks: expected %d, got %d", cu.TotalChunks, len(sizes)))
	}
	for i, size := range sizes {
		if size == 0 {
			return nil, NewUserError(fmt.Sprintf("chunk %d is empty", i))
		}
	}

	ext := meta.Ext
	if ext == "" {
		ext = filepath.Ext(meta.Name)
	}
	if ext == "" {
		return nil, NewUserError("only files with an extension can be uploaded")
	}
	base := strings.TrimSuffix(meta.Name, ext)
	if err := validateItemName(base); err != nil {
		return nil, err
	}
	uniqueName, err := nextUniqueNameForService(ctx, s.itemRepo, userID, parentID, meta.Name, true)
	if err != nil {
		return nil, err
	}
	if err := ValidateFilePassword(meta.Password, uniqueName); err != nil {
		return nil, err
	}

	var passwordHash *string
	if meta.Password != "" {
		h := utils.HashFilePassword(meta.Password)
		passwordHash = &h
	}

	relPath := filepath.Join(storage.DirUsers, strconv.FormatUint(userID, 10), uuid.NewString()+".item")
	finalPath := s.storage.FullPath(relPath)

	assembledSize, err := s.storage.AssembleChunks(uploadID, cu.TotalChunks, finalPath)
	if err != nil {
		return nil, err
	}

	chunkCount := cu.TotalChunks
	chunkSize := cu.ChunkSize

	item := &entity.Item{
		UserID:       userID,
		ParentID:     parentID,
		Type:         entity.ItemTypeFile,
		Name:         uniqueName,
		Ext:          &ext,
		Path:         itemPath(parentPath, uniqueName),
		Size:         uint64(assembledSize),
		StoragePath:  relPath,
		IsChunked:    true,
		ChunkCount:   &chunkCount,
		ChunkSize:    &chunkSize,
		PasswordHash: passwordHash,
	}

	created, err := s.itemRepo.Create(ctx, item)
	if err != nil {
		_ = s.storage.Delete(relPath)
		return nil, err
	}

	_ = s.userRepo.AddStorageUsed(ctx, userID, uint64(assembledSize), false)
	_ = s.chunkRepo.MarkCompleted(ctx, uploadID)
	_ = s.storage.DeleteChunkDir(uploadID)

	utils.Log.Info().Uint64("user_id", userID).Uint64("item_id", created.ID).Int64("size", assembledSize).Int("chunks", cu.TotalChunks).Msg("chunked file uploaded")
	return created, nil
}

// resolveFolderForService is a helper to resolve a directory path to a folder item.
func resolveFolderForService(ctx context.Context, repo *repository.ItemRepository, userID uint64, directory string) (*entity.Item, error) {
	directory = normalizePath(directory)
	if directory == "" {
		return nil, nil
	}
	item, err := repo.GetByPath(ctx, userID, directory)
	if err != nil {
		return nil, err
	}
	if item.Type != entity.ItemTypeFolder {
		return nil, NewUserError("path is not a folder")
	}
	return item, nil
}

// nextUniqueNameForService is a helper to find a non-conflicting name.
func nextUniqueNameForService(ctx context.Context, repo *repository.ItemRepository, userID uint64, parentID *uint64, name string, isFile bool) (string, error) {
	candidate := name
	ext := ""
	base := name
	if isFile {
		ext = filepath.Ext(name)
		base = strings.TrimSuffix(name, ext)
	}
	for i := 1; ; i++ {
		var err error
		if isFile {
			_, err = repo.GetByParentAndName(ctx, userID, parentID, base+ext, &ext)
		} else {
			_, err = repo.GetByParentAndName(ctx, userID, parentID, candidate, nil)
		}
		if errors.Is(err, repository.ErrNotFound) {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
		if isFile {
			candidate = fmt.Sprintf("%s_%d%s", base, i, ext)
		} else {
			candidate = fmt.Sprintf("%s_%d", name, i)
		}
	}
}
