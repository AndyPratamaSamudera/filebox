package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"filebox/internal/encryption"
	"filebox/internal/entity"
	"filebox/internal/repository"
	"filebox/internal/storage"
	"filebox/internal/utils"
)

// ItemService handles all operations on the unified items table (files + folders).
type ItemService struct {
	repo     *repository.ItemRepository
	userRepo *repository.UserRepository
	storage  *storage.LocalStorage
	enc      *encryption.Service
	maxDirect int64
}

// NewItemService creates an ItemService.
func NewItemService(repo *repository.ItemRepository, userRepo *repository.UserRepository, st *storage.LocalStorage, enc *encryption.Service, maxDirect int64) *ItemService {
	return &ItemService{
		repo:      repo,
		userRepo:  userRepo,
		storage:   st,
		enc:       enc,
		maxDirect: maxDirect,
	}
}

// ItemUploadInput bundles the fields needed to store an uploaded file.
type ItemUploadInput struct {
	UserID          uint64
	Directory       string
	Name            string
	MIME            string
	Content         io.Reader
	DeclaredSize    int64
	Favorite        bool
	ShareRecipients []string
	Password        string
}

// ItemDetail is the response payload for GET /item/detail.
type ItemDetail struct {
	Item   *entity.Item       `json:"item"`
	Shares []entity.ItemShare `json:"shares,omitempty"`
}

var itemNameRe = regexp.MustCompile(`^[a-zA-Z0-9 _\-.,()'&!+]+$`)

// normalizePath trims leading/trailing slashes and removes a trailing slash.
func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.Trim(p, "/")
	p = strings.TrimSuffix(p, "/")
	return p
}

// parentAndName splits a path into parent directory and final segment.
// For a root item, parent is "" and name is the path itself.
func parentAndName(p string) (parent, name string) {
	p = normalizePath(p)
	if p == "" {
		return "", ""
	}
	idx := strings.LastIndex(p, "/")
	if idx == -1 {
		return "", p
	}
	return p[:idx], p[idx+1:]
}

// isFilePath reports whether the last path segment looks like a file (has an extension).
func isFilePath(p string) bool {
	_, name := parentAndName(p)
	if name == "" {
		return false
	}
	ext := filepath.Ext(name)
	return ext != "" && ext != "."
}

// itemPath joins a parent path and a child name into a full item path.
func itemPath(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "/" + name
}

// validateItemName ensures a name contains only letters, digits, spaces, '-' and '_'.
func validateItemName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return NewUserError("name is required")
	}
	if len(name) > 255 {
		return NewUserError("name too long")
	}
	if !itemNameRe.MatchString(name) {
		return NewUserError("name may only contain letters, numbers, spaces, '-', '_', '.', ',', '(', ')', ''', '&', '!', '+'")
	}
	return nil
}

// ValidateFilePassword rejects passwords that are identical to the file name.
func ValidateFilePassword(password, name string) error {
	if password == "" {
		return nil
	}
	if password == name {
		return NewUserError("password cannot be the same as the file name")
	}
	return nil
}

// resolveFolder returns the folder item for the given directory path, or nil for root.
func (s *ItemService) resolveFolder(ctx context.Context, userID uint64, directory string) (*entity.Item, error) {
	directory = normalizePath(directory)
	if directory == "" {
		return nil, nil
	}
	item, err := s.repo.GetByPath(ctx, userID, directory)
	if err != nil {
		return nil, err
	}
	if item.Type != entity.ItemTypeFolder {
		return nil, NewUserError("path is not a folder")
	}
	return item, nil
}

// resolveItem returns the item at the given path, verifying it matches the expected type.
func (s *ItemService) resolveItem(ctx context.Context, userID uint64, directory string) (*entity.Item, error) {
	directory = normalizePath(directory)
	if directory == "" {
		return nil, NewUserError("root is not an item")
	}
	item, err := s.repo.GetByPath(ctx, userID, directory)
	if err != nil {
		return nil, err
	}
	if isFilePath(directory) && item.Type != entity.ItemTypeFile {
		return nil, NewUserError("path is not a file")
	}
	if !isFilePath(directory) && item.Type != entity.ItemTypeFolder {
		return nil, NewUserError("path is not a folder")
	}
	return item, nil
}

// resolveItemOrShared returns an owned item by path, or a shared item if the user is a recipient.
func (s *ItemService) resolveItemOrShared(ctx context.Context, userID uint64, directory string) (*entity.Item, error) {
	directory = normalizePath(directory)
	if directory == "" {
		return nil, NewUserError("root is not an item")
	}
	// Try owned item first.
	item, err := s.repo.GetByPath(ctx, userID, directory)
	if err == nil {
		if isFilePath(directory) && item.Type != entity.ItemTypeFile {
			return nil, NewUserError("path is not a file")
		}
		return item, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	// Fall back to shared items where the path matches the owner's tree.
	shares, err := s.repo.ListSharesWithUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, sh := range shares {
		if sh.ItemID == 0 {
			continue
		}
		ownerItem, err := s.repo.GetByID(ctx, sh.ItemID, sh.OwnerUserID)
		if err != nil {
			continue
		}
		if ownerItem.Path == directory && ownerItem.Type == entity.ItemTypeFile {
			return ownerItem, nil
		}
	}
	return nil, repository.ErrNotFound
}

// nextUniqueName returns a name that does not already exist under the parent.
// For files, name is the full name (including extension).
func (s *ItemService) nextUniqueName(ctx context.Context, userID uint64, parentID *uint64, name string, isFile bool) (string, error) {
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
			_, err = s.repo.GetByParentAndName(ctx, userID, parentID, base+ext, &ext)
		} else {
			_, err = s.repo.GetByParentAndName(ctx, userID, parentID, candidate, nil)
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

// List returns the direct children of a folder (root when directory is empty).
func (s *ItemService) List(ctx context.Context, userID uint64, directory string) ([]entity.Item, error) {
	parent, err := s.resolveFolder(ctx, userID, directory)
	if err != nil {
		return nil, err
	}
	var parentID *uint64
	if parent != nil {
		parentID = &parent.ID
	}
	items, err := s.repo.ListByParent(ctx, userID, parentID)
	if err != nil {
		return nil, err
	}
	return s.attachShareCounts(ctx, userID, items), nil
}

// Detail returns item metadata plus its share list for files.
func (s *ItemService) Detail(ctx context.Context, userID uint64, directory string) (*ItemDetail, error) {
	item, err := s.resolveItem(ctx, userID, directory)
	if err != nil {
		return nil, err
	}
	detail := &ItemDetail{Item: item}
	if item.Type == entity.ItemTypeFile {
		shares, err := s.repo.ListSharesForItem(ctx, item.ID, userID)
		if err != nil {
			return nil, err
		}
		detail.Shares = shares
	}
	return detail, nil
}

// CreateFolder creates a new folder at the given directory path.
func (s *ItemService) CreateFolder(ctx context.Context, userID uint64, directory string) (*entity.Item, error) {
	directory = normalizePath(directory)
	if directory == "" {
		return nil, NewUserError("folder path is required")
	}
	parentPath, name := parentAndName(directory)
	if err := validateItemName(name); err != nil {
		return nil, err
	}
	parent, err := s.resolveFolder(ctx, userID, parentPath)
	if err != nil {
		return nil, err
	}
	var parentID *uint64
	if parent != nil {
		parentID = &parent.ID
	}
	uniqueName, err := s.nextUniqueName(ctx, userID, parentID, name, false)
	if err != nil {
		return nil, err
	}
	folder := &entity.Item{
		UserID:   userID,
		ParentID: parentID,
		Type:     entity.ItemTypeFolder,
		Name:     uniqueName,
		Path:     itemPath(parentPath, uniqueName),
	}
	return s.repo.Create(ctx, folder)
}

// Upload stores a single file under the given directory.
func (s *ItemService) Upload(ctx context.Context, in ItemUploadInput) (*entity.Item, error) {
	if s.maxDirect > 0 && in.DeclaredSize > s.maxDirect {
		return nil, NewUserError("file exceeds direct upload limit; use chunk upload")
	}

	parent, err := s.resolveFolder(ctx, in.UserID, in.Directory)
	if err != nil {
		return nil, err
	}
	var parentID *uint64
	if parent != nil {
		parentID = &parent.ID
	}
	var parentPath string
	if parent != nil {
		parentPath = parent.Path
	}

	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, NewUserError("file name is required")
	}
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		return nil, NewUserError("only files with an extension can be uploaded")
	}
	base := strings.TrimSuffix(name, ext)
	if err := validateItemName(base); err != nil {
		return nil, err
	}
	uniqueName, err := s.nextUniqueName(ctx, in.UserID, parentID, name, true)
	if err != nil {
		return nil, err
	}
	if err := ValidateFilePassword(in.Password, uniqueName); err != nil {
		return nil, err
	}

	var passwordHash *string
	if in.Password != "" {
		h := utils.HashFilePassword(in.Password)
		passwordHash = &h
	}

	plain, err := io.ReadAll(in.Content)
	if err != nil {
		return nil, fmt.Errorf("read upload: %w", err)
	}
	cipher, iv, tag, err := s.enc.EncryptBytes(plain)
	if err != nil {
		return nil, fmt.Errorf("encrypt file: %w", err)
	}

	relPath, size, err := s.storage.SaveUpload(in.UserID, ext, bytes.NewReader(cipher))
	if err != nil {
		return nil, err
	}

	ivB64 := base64.StdEncoding.EncodeToString(iv)
	tagB64 := base64.StdEncoding.EncodeToString(tag)

	mimePtr := &in.MIME
	if in.MIME == "" {
		mimePtr = nil
	}
	item := &entity.Item{
		UserID:        in.UserID,
		ParentID:      parentID,
		Type:          entity.ItemTypeFile,
		Name:          uniqueName,
		Ext:           &ext,
		Path:          itemPath(parentPath, uniqueName),
		MIME:          mimePtr,
		Size:          uint64(size),
		StoragePath:   relPath,
		EncryptionIV:  &ivB64,
		EncryptionTag: &tagB64,
		IsFavorite:    in.Favorite,
		PasswordHash:  passwordHash,
	}

	created, err := s.repo.Create(ctx, item)
	if err != nil {
		_ = s.storage.Delete(relPath)
		return nil, err
	}

	for _, email := range in.ShareRecipients {
		email = strings.TrimSpace(strings.ToLower(email))
		if email == "" {
			continue
		}
		if _, err := s.shareByEmail(ctx, in.UserID, created.ID, email); err != nil {
			utils.Log.Warn().Err(err).Uint64("item_id", created.ID).Str("email", email).Msg("failed to auto-share upload")
		}
	}

	_ = s.userRepo.AddStorageUsed(ctx, in.UserID, uint64(size), false)
	utils.Log.Info().Uint64("user_id", in.UserID).Uint64("item_id", created.ID).Int64("size", size).Msg("file uploaded")
	return created, nil
}

// Update modifies an item. For folders only the name can be changed; for files,
// name, favorite, shares, and password can all be updated.
// When shares is non-nil, the existing share list is fully replaced with the
// provided list (overwrite), not appended to.
func (s *ItemService) Update(ctx context.Context, userID uint64, directory string, name string, isFavorite *bool, shares []string, password string) (*entity.Item, error) {
	item, err := s.resolveItem(ctx, userID, directory)
	if err != nil {
		return nil, err
	}
	parentPath, _ := parentAndName(item.Path)

	if item.Type == entity.ItemTypeFolder {
		name = strings.TrimSpace(name)
		if err := validateItemName(name); err != nil {
			return nil, err
		}
		newName, err := s.nextUniqueName(ctx, userID, item.ParentID, name, false)
		if err != nil {
			return nil, err
		}
		oldPath := item.Path
		newPath := itemPath(parentPath, newName)
		if err := s.repo.UpdateNameAndPath(ctx, item.ID, userID, newName, newPath); err != nil {
			return nil, err
		}
		if oldPath != newPath {
			if err := s.repo.RepathSubtree(ctx, userID, oldPath, newPath); err != nil {
				return nil, err
			}
		}
		return s.repo.GetByID(ctx, item.ID, userID)
	}

	// File update.
	if name != "" {
		name = strings.TrimSpace(name)
		if err := validateItemName(name); err != nil {
			return nil, err
		}
		ext := filepath.Ext(item.Name)
		newName := name + ext
		newName, err = s.nextUniqueName(ctx, userID, item.ParentID, newName, true)
		if err != nil {
			return nil, err
		}
		newPath := itemPath(parentPath, newName)
		if err := s.repo.UpdateNameAndPath(ctx, item.ID, userID, newName, newPath); err != nil {
			return nil, err
		}
	}

	if isFavorite != nil {
		if err := s.repo.SetFavorite(ctx, item.ID, userID, *isFavorite); err != nil {
			return nil, err
		}
	}

	if password != "" || name != "" || isFavorite != nil || shares != nil {
		// Re-fetch the row so the latest name/path is used.
		item, err = s.repo.GetByID(ctx, item.ID, userID)
		if err != nil {
			return nil, err
		}
	}

	if password != "" {
		if err := ValidateFilePassword(password, item.Name); err != nil {
			return nil, err
		}
		h := utils.HashFilePassword(password)
		if err := s.repo.SetPassword(ctx, item.ID, userID, &h); err != nil {
			return nil, err
		}
	} else if shares != nil && len(shares) == 0 {
		// Empty password with no explicit shares request does not clear password.
	}
	if password == "" && shares == nil && name == "" && isFavorite == nil {
		// clear password when password explicitly empty? The user sends empty string to remove.
		if err := s.repo.SetPassword(ctx, item.ID, userID, nil); err != nil {
			return nil, err
		}
	}

	if shares != nil {
		if err := s.replaceShares(ctx, userID, item.ID, shares); err != nil {
			return nil, err
		}
	}

	return s.repo.GetByID(ctx, item.ID, userID)
}

// Delete removes a file or folder (recursively) from disk and database.
func (s *ItemService) Delete(ctx context.Context, userID uint64, directory string) error {
	item, err := s.resolveItem(ctx, userID, directory)
	if err != nil {
		return err
	}
	if item.Type == entity.ItemTypeFile {
		if err := s.repo.Delete(ctx, item.ID, userID); err != nil {
			return err
		}
		_ = s.storage.Delete(item.StoragePath)
		_ = s.userRepo.AddStorageUsed(ctx, userID, item.Size, true)
		return nil
	}

	// Folder: delete all descendants in a transaction.
	tx, err := s.repo.DB().BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	ids, err := s.repo.SubtreeIDsTx(ctx, tx, userID, item.ID)
	if err != nil {
		return err
	}
	files, err := s.repo.FilesByIDsTx(ctx, tx, userID, ids)
	if err != nil {
		return err
	}
	var totalSize uint64
	for _, f := range files {
		totalSize += f.Size
	}
	if err := s.repo.DeleteByIDsTx(ctx, tx, userID, ids); err != nil {
		return err
	}
	if totalSize > 0 {
		if err := s.userRepo.AddStorageUsedTx(ctx, tx, userID, totalSize, true); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	for _, f := range files {
		_ = s.storage.Delete(f.StoragePath)
	}
	return nil
}

// Search returns items matching the query.
func (s *ItemService) Search(ctx context.Context, userID uint64, query string) ([]entity.Item, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return []entity.Item{}, nil
	}
	items, err := s.repo.Search(ctx, userID, q, 50)
	if err != nil {
		return nil, err
	}
	return s.attachShareCounts(ctx, userID, items), nil
}

// ListFavorites returns the user's favorited files.
func (s *ItemService) ListFavorites(ctx context.Context, userID uint64) ([]entity.Item, error) {
	items, err := s.repo.ListFavorites(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.attachShareCounts(ctx, userID, items), nil
}

// ListShared returns files shared with the user.
func (s *ItemService) ListShared(ctx context.Context, userID uint64) ([]entity.SharedItem, error) {
	return s.repo.ListSharesWithUser(ctx, userID)
}

// GetByPathOrShared returns an item by path for the owner or a recipient.
func (s *ItemService) GetByPathOrShared(ctx context.Context, userID uint64, directory string) (*entity.Item, error) {
	item, err := s.resolveItemOrShared(ctx, userID, directory)
	if err != nil {
		return nil, err
	}
	if item.Type != entity.ItemTypeFile {
		return nil, NewUserError("only files can be downloaded or previewed")
	}
	return item, nil
}

// VerifyPassword checks a supplied password against a file's stored hash.
func (s *ItemService) VerifyPassword(item *entity.Item, password string) error {
	if item.PasswordHash == nil || *item.PasswordHash == "" {
		return nil
	}
	if password == "" {
		return NewUserError("file is locked")
	}
	if !utils.CheckFilePassword(password, *item.PasswordHash) {
		return NewUserError("incorrect file password")
	}
	return nil
}

// StoragePath returns the absolute on-disk path for an item.
func (s *ItemService) StoragePath(item *entity.Item) string {
	return s.storage.FullPath(item.StoragePath)
}

// DecryptToTempPath decrypts a file into a temporary file and returns its path.
func (s *ItemService) DecryptToTempPath(ctx context.Context, item *entity.Item) (string, error) {
	srcPath := s.storage.FullPath(item.StoragePath)
	src, err := os.Open(srcPath)
	if err != nil {
		return "", fmt.Errorf("open encrypted file: %w", err)
	}
	defer src.Close()

	tempFile, err := os.CreateTemp(filepath.Join(s.storage.Base(), storage.DirTemp), "decrypt-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer tempFile.Close()

	if item.IsChunked {
		if err := s.decryptChunkedFile(tempFile, src, item); err != nil {
			_ = os.Remove(tempPath)
			return "", err
		}
	} else {
		if err := s.decryptDirectFile(tempFile, src, item); err != nil {
			_ = os.Remove(tempPath)
			return "", err
		}
	}
	return tempPath, nil
}

func (s *ItemService) decryptDirectFile(out io.Writer, in io.Reader, item *entity.Item) error {
	if item.EncryptionIV == nil || item.EncryptionTag == nil {
		return errors.New("missing encryption metadata")
	}
	cipher, err := io.ReadAll(in)
	if err != nil {
		return fmt.Errorf("read encrypted file: %w", err)
	}
	plain, err := s.enc.DecryptBytes(cipher)
	if err != nil {
		return fmt.Errorf("decrypt file: %w", err)
	}
	if _, err := out.Write(plain); err != nil {
		return fmt.Errorf("write plaintext: %w", err)
	}
	return nil
}

func (s *ItemService) decryptChunkedFile(out io.Writer, in io.Reader, item *entity.Item) error {
	if item.ChunkSize == nil || item.ChunkCount == nil || *item.ChunkCount <= 0 {
		return errors.New("missing chunk metadata")
	}
	plainChunkSize := *item.ChunkSize
	chunkCount := *item.ChunkCount
	encChunkSize := plainChunkSize + 12 + 16

	for i := 0; i < chunkCount; i++ {
		expected := encChunkSize
		if i == chunkCount-1 {
			expected = plainChunkSize + 12 + 16
		}
		buf := make([]byte, expected)
		n, err := io.ReadFull(in, buf)
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			return fmt.Errorf("read chunk %d: %w", i, err)
		}
		if n == 0 {
			return fmt.Errorf("chunk %d is empty", i)
		}
		plain, err := s.enc.DecryptChunk(buf[:n])
		if err != nil {
			return fmt.Errorf("decrypt chunk %d: %w", i, err)
		}
		if _, err := out.Write(plain); err != nil {
			return fmt.Errorf("write chunk %d: %w", i, err)
		}
	}
	return nil
}

// attachShareCounts enriches file items with the number of recipients.
func (s *ItemService) attachShareCounts(ctx context.Context, userID uint64, items []entity.Item) []entity.Item {
	if len(items) == 0 {
		return items
	}
	ids := make([]uint64, 0, len(items))
	for _, it := range items {
		if it.Type == entity.ItemTypeFile {
			ids = append(ids, it.ID)
		}
	}
	if len(ids) == 0 {
		return items
	}
	counts, err := s.repo.CountShares(ctx, userID, ids)
	if err != nil {
		return items
	}
	for i := range items {
		if items[i].Type == entity.ItemTypeFile {
			items[i].ShareCount = counts[items[i].ID]
		}
	}
	return items
}

// shareByEmail creates a share record for a file item.
func (s *ItemService) shareByEmail(ctx context.Context, ownerID, itemID uint64, email string) (*entity.ItemShare, error) {
	item, err := s.repo.GetByID(ctx, itemID, ownerID)
	if err != nil {
		return nil, err
	}
	if item.Type != entity.ItemTypeFile {
		return nil, NewUserError("only files can be shared")
	}
	recipient, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, NewUserError("recipient not found")
		}
		return nil, err
	}
	if recipient.ID == ownerID {
		return nil, NewUserError("cannot share with yourself")
	}
	if existing, err := s.repo.GetShareForRecipient(ctx, itemID, recipient.ID); err == nil && existing != nil {
		return existing, nil
	}
	share := &entity.ItemShare{
		ItemID:           itemID,
		OwnerUserID:      ownerID,
		SharedWithUserID: recipient.ID,
	}
	if err := s.repo.CreateShare(ctx, share); err != nil {
		return nil, err
	}
	return s.repo.GetShareForRecipient(ctx, itemID, recipient.ID)
}

// replaceShares replaces the entire share list for an item. Existing shares whose
// recipients are not in the new list are removed; new recipients are added.
func (s *ItemService) replaceShares(ctx context.Context, ownerID, itemID uint64, emails []string) error {
	item, err := s.repo.GetByID(ctx, itemID, ownerID)
	if err != nil {
		return err
	}
	if item.Type != entity.ItemTypeFile {
		return NewUserError("only files can be shared")
	}
	existing, err := s.repo.ListSharesForItem(ctx, itemID, ownerID)
	if err != nil {
		return err
	}
	wanted := map[string]bool{}
	for _, email := range emails {
		email = strings.TrimSpace(strings.ToLower(email))
		if email != "" {
			wanted[email] = true
		}
	}
	existingByEmail := map[string]uint64{}
	for _, sh := range existing {
		existingByEmail[strings.ToLower(sh.SharedEmail)] = sh.ID
	}
	for email := range wanted {
		if _, ok := existingByEmail[email]; ok {
			continue
		}
		if _, err := s.shareByEmail(ctx, ownerID, itemID, email); err != nil {
			return err
		}
	}
	for email, id := range existingByEmail {
		if wanted[email] {
			continue
		}
		if err := s.repo.DeleteShare(ctx, id, ownerID); err != nil {
			return err
		}
	}
	return nil
}
