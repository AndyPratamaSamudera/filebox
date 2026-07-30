package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/google/uuid"
)

// subdirectories under the storage root, per the FileBox spec.
const (
	DirUsers      = "users"
	DirTrash      = "trash"
	DirTemp       = "temp"
	DirChunks     = "chunks"
	DirThumbnails = "thumbnails"
)

// LocalStorage manages file I/O on the local filesystem. Only metadata lives
// in MariaDB; file bytes live here under <base>/users/<userID>/<uuid>.item.
type LocalStorage struct {
	base string
}

// NewLocalStorage resolves the storage root (relative to CWD when not absolute)
// and ensures the expected subdirectories exist.
func NewLocalStorage(path string) (*LocalStorage, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve storage path: %w", err)
	}
	for _, sub := range []string{DirUsers, DirTrash, DirTemp, DirChunks, DirThumbnails} {
		if err := os.MkdirAll(filepath.Join(abs, sub), 0o755); err != nil {
			return nil, fmt.Errorf("create storage dir %s: %w", sub, err)
		}
	}
	return &LocalStorage{base: abs}, nil
}

// Base returns the absolute storage root.
func (s *LocalStorage) Base() string { return s.base }

// SaveUpload writes content to a unique file under users/<userID>/ and returns
// the relative storage path (e.g. "users/3/<uuid>.item") plus the byte count.
// The original extension is kept in the database; the physical file is always
// stored with the .item suffix.
func (s *LocalStorage) SaveUpload(userID uint64, ext string, content io.Reader) (string, int64, error) {
	dir := filepath.Join(s.base, DirUsers, strconv.FormatUint(userID, 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", 0, fmt.Errorf("create user dir: %w", err)
	}

	name := uuid.NewString() + ".item"
	full := filepath.Join(dir, name)

	f, err := os.Create(full)
	if err != nil {
		return "", 0, fmt.Errorf("create storage file: %w", err)
	}
	defer f.Close()

	n, err := io.Copy(f, content)
	if err != nil {
		_ = os.Remove(full)
		return "", 0, fmt.Errorf("write storage file: %w", err)
	}

	rel := filepath.Join(DirUsers, strconv.FormatUint(userID, 10), name)
	return rel, n, nil
}

// chunkDir returns the absolute directory for a chunked upload session.
func (s *LocalStorage) chunkDir(uploadID string) string {
	return filepath.Join(s.base, DirChunks, uploadID)
}

// chunkPath returns the absolute path for a single chunk.
func (s *LocalStorage) chunkPath(uploadID string, index int) string {
	return filepath.Join(s.chunkDir(uploadID), strconv.Itoa(index))
}

// SaveChunk writes a single chunk for a chunked upload session. It returns
// the number of bytes written.
func (s *LocalStorage) SaveChunk(uploadID string, index int, content io.Reader) (int64, error) {
	dir := s.chunkDir(uploadID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, fmt.Errorf("create chunk dir: %w", err)
	}
	full := s.chunkPath(uploadID, index)
	f, err := os.Create(full)
	if err != nil {
		return 0, fmt.Errorf("create chunk file: %w", err)
	}
	defer f.Close()

	n, err := io.Copy(f, content)
	if err != nil {
		_ = os.Remove(full)
		return 0, fmt.Errorf("write chunk file: %w", err)
	}
	return n, nil
}

// ListChunkSizes returns the size of each chunk that has been written, in
// order from 0 up to the highest index present.
func (s *LocalStorage) ListChunkSizes(uploadID string) ([]int64, error) {
	dir := s.chunkDir(uploadID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read chunk dir: %w", err)
	}

	sizes := make(map[int]int64)
	maxIdx := -1
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		idx, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		sizes[idx] = info.Size()
		if idx > maxIdx {
			maxIdx = idx
		}
	}

	result := make([]int64, 0, len(sizes))
	for i := 0; i <= maxIdx; i++ {
		result = append(result, sizes[i])
	}
	return result, nil
}

// AssembleChunks concatenates all chunk files in order into the given final
// file path. It returns the total assembled size. The caller is responsible
// for deleting the chunk directory after success.
func (s *LocalStorage) AssembleChunks(uploadID string, chunkCount int, finalPath string) (int64, error) {
	dir := s.chunkDir(uploadID)
	outDir := filepath.Dir(finalPath)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return 0, fmt.Errorf("create final dir: %w", err)
	}

	out, err := os.Create(finalPath)
	if err != nil {
		return 0, fmt.Errorf("create final file: %w", err)
	}
	defer out.Close()

	var total int64
	for i := 0; i < chunkCount; i++ {
		chunkFile := filepath.Join(dir, strconv.Itoa(i))
		in, err := os.Open(chunkFile)
		if err != nil {
			return 0, fmt.Errorf("open chunk %d: %w", i, err)
		}
		n, err := io.Copy(out, in)
		_ = in.Close()
		if err != nil {
			return 0, fmt.Errorf("copy chunk %d: %w", i, err)
		}
		total += n
	}
	return total, nil
}

// DeleteChunkDir removes the temporary chunk directory for an upload session.
func (s *LocalStorage) DeleteChunkDir(uploadID string) error {
	dir := s.chunkDir(uploadID)
	if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// FullPath joins the storage root with a relative storage path.
func (s *LocalStorage) FullPath(rel string) string {
	return filepath.Join(s.base, rel)
}

// Delete removes a file from the filesystem.
func (s *LocalStorage) Delete(rel string) error {
	if rel == "" {
		return nil
	}
	if err := os.Remove(s.FullPath(rel)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
