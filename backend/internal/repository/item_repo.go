package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"filebox/internal/entity"
)

// ItemRepository accesses the unified items table.
type ItemRepository struct {
	db *sqlx.DB
}

// NewItemRepository creates an ItemRepository.
func NewItemRepository(db *sqlx.DB) *ItemRepository {
	return &ItemRepository{db: db}
}

// DB exposes the underlying sqlx.DB for transaction handling.
func (r *ItemRepository) DB() *sqlx.DB {
	return r.db
}

// Create inserts a new item and returns the created row.
func (r *ItemRepository) Create(ctx context.Context, item *entity.Item) (*entity.Item, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO items (user_id, parent_id, type, name, ext, path, mime, size, storage_path, checksum,
		                    is_chunked, chunk_count, chunk_size, is_favorite, password_hash, encryption_iv, encryption_tag)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.UserID, item.ParentID, item.Type, item.Name, item.Ext, item.Path, item.MIME, item.Size, item.StoragePath,
		item.Checksum, item.IsChunked, item.ChunkCount, item.ChunkSize, item.IsFavorite, item.PasswordHash, item.EncryptionIV, item.EncryptionTag)
	if err != nil {
		return nil, fmt.Errorf("insert item: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, uint64(id), item.UserID)
}

// GetByID returns an item owned by the given user.
func (r *ItemRepository) GetByID(ctx context.Context, id, userID uint64) (*entity.Item, error) {
	var item entity.Item
	err := r.db.GetContext(ctx, &item,
		`SELECT * FROM items WHERE id = ? AND user_id = ?`, id, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	item.SetLocked()
	return &item, nil
}

// GetByPath returns an item by its full path.
func (r *ItemRepository) GetByPath(ctx context.Context, userID uint64, path string) (*entity.Item, error) {
	var item entity.Item
	err := r.db.GetContext(ctx, &item,
		`SELECT * FROM items WHERE user_id = ? AND path = ? LIMIT 1`,
		userID, path)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	item.SetLocked()
	return &item, nil
}

// GetByParentAndName returns an item by its parent and name/ext. ext may be nil for folders.
func (r *ItemRepository) GetByParentAndName(ctx context.Context, userID uint64, parentID *uint64, name string, ext *string) (*entity.Item, error) {
	var item entity.Item
	var err error
	if ext != nil {
		err = r.db.GetContext(ctx, &item,
			`SELECT * FROM items WHERE user_id = ? AND parent_id <=> ? AND name = ? AND ext <=> ? LIMIT 1`,
			userID, parentID, name, ext)
	} else {
		err = r.db.GetContext(ctx, &item,
			`SELECT * FROM items WHERE user_id = ? AND parent_id <=> ? AND name = ? AND ext IS NULL LIMIT 1`,
			userID, parentID, name)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	item.SetLocked()
	return &item, nil
}

// ListByParent returns direct children of a folder. A nil parentID lists root items.
func (r *ItemRepository) ListByParent(ctx context.Context, userID uint64, parentID *uint64) ([]entity.Item, error) {
	var items []entity.Item
	err := r.db.SelectContext(ctx, &items,
		`SELECT * FROM items WHERE user_id = ? AND parent_id <=> ? ORDER BY type DESC, name ASC`,
		userID, parentID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].SetLocked()
	}
	return items, nil
}

// ListFavorites returns the user's favorited files.
func (r *ItemRepository) ListFavorites(ctx context.Context, userID uint64) ([]entity.Item, error) {
	var items []entity.Item
	err := r.db.SelectContext(ctx, &items,
		`SELECT * FROM items WHERE user_id = ? AND type = 'file' AND is_favorite = 1 ORDER BY name ASC`,
		userID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].SetLocked()
	}
	return items, nil
}

// Search returns items whose name matches the query (both files and folders).
func (r *ItemRepository) Search(ctx context.Context, userID uint64, query string, limit int) ([]entity.Item, error) {
	if limit <= 0 {
		limit = 50
	}
	pattern := "%" + query + "%"
	var items []entity.Item
	err := r.db.SelectContext(ctx, &items,
		`SELECT * FROM items
		 WHERE user_id = ? AND (name LIKE ? OR ext LIKE ? OR mime LIKE ? OR path LIKE ?)
		 ORDER BY type DESC, name ASC LIMIT ?`,
		userID, pattern, pattern, pattern, pattern, limit)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].SetLocked()
	}
	return items, nil
}

// Update renames an item. ext and parent_id are intentionally not changed here.
func (r *ItemRepository) Update(ctx context.Context, id, userID uint64, name string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE items SET name = ? WHERE id = ? AND user_id = ?`,
		name, id, userID)
	return err
}

// UpdateNameAndPath renames an item and updates its stored path.
func (r *ItemRepository) UpdateNameAndPath(ctx context.Context, id, userID uint64, name, path string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE items SET name = ?, path = ? WHERE id = ? AND user_id = ?`,
		name, path, id, userID)
	return err
}

// RepathSubtree updates the path of a folder and all of its descendants by
// replacing the old path prefix with the new one.
func (r *ItemRepository) RepathSubtree(ctx context.Context, userID uint64, oldPath, newPath string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE items
		    SET path = CONCAT(?, SUBSTRING(path, CHAR_LENGTH(?) + 1))
		  WHERE user_id = ? AND (path = ? OR path LIKE CONCAT(?, '/%'))`,
		newPath, oldPath, userID, oldPath, oldPath)
	return err
}

// SetFavorite toggles the favorite flag on a file.
func (r *ItemRepository) SetFavorite(ctx context.Context, id, userID uint64, favorite bool) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE items SET is_favorite = ? WHERE id = ? AND user_id = ? AND type = 'file'`,
		favorite, id, userID)
	return err
}

// SetPassword stores or clears the hashed file password.
func (r *ItemRepository) SetPassword(ctx context.Context, id, userID uint64, hash *string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE items SET password_hash = ? WHERE id = ? AND user_id = ? AND type = 'file'`,
		hash, id, userID)
	return err
}

// Delete removes an item row permanently.
func (r *ItemRepository) Delete(ctx context.Context, id, userID uint64) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM items WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// DeleteByIDsTx permanently deletes items by ID within a transaction.
func (r *ItemRepository) DeleteByIDsTx(ctx context.Context, tx *sqlx.Tx, userID uint64, ids []uint64) error {
	if len(ids) == 0 {
		return nil
	}
	query, args, err := sqlx.In(
		`DELETE FROM items WHERE user_id = ? AND id IN (?)`,
		userID, ids)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, tx.Rebind(query), args...)
	return err
}

// SubtreeIDsTx returns IDs of the item and all descendants, deepest-first.
func (r *ItemRepository) SubtreeIDsTx(ctx context.Context, tx *sqlx.Tx, userID, id uint64) ([]uint64, error) {
	var ids []uint64
	err := tx.SelectContext(ctx, &ids,
		`WITH RECURSIVE descendants AS (
			SELECT id, type FROM items WHERE id = ? AND user_id = ?
			UNION ALL
			SELECT i.id, i.type FROM items i
			INNER JOIN descendants d ON i.parent_id = d.id
			WHERE i.user_id = ?
		)
		SELECT id FROM descendants ORDER BY type = 'folder' DESC, id DESC`,
		id, userID, userID)
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// FilesByIDsTx returns file items whose ID is in ids within a transaction.
func (r *ItemRepository) FilesByIDsTx(ctx context.Context, tx *sqlx.Tx, userID uint64, ids []uint64) ([]entity.Item, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	query, args, err := sqlx.In(
		`SELECT * FROM items WHERE user_id = ? AND id IN (?) AND type = 'file'`,
		userID, ids)
	if err != nil {
		return nil, err
	}
	var items []entity.Item
	err = tx.SelectContext(ctx, &items, tx.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].SetLocked()
	}
	return items, nil
}

// CreateShare stores a share record.
func (r *ItemRepository) CreateShare(ctx context.Context, s *entity.ItemShare) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO item_shares (item_id, owner_user_id, shared_with_user_id) VALUES (?, ?, ?)`,
		s.ItemID, s.OwnerUserID, s.SharedWithUserID)
	if err != nil {
		return fmt.Errorf("insert share: %w", err)
	}
	return nil
}

// GetShareForRecipient returns the share record for a specific item and recipient.
func (r *ItemRepository) GetShareForRecipient(ctx context.Context, itemID, recipientID uint64) (*entity.ItemShare, error) {
	var s entity.ItemShare
	err := r.db.GetContext(ctx, &s,
		`SELECT * FROM item_shares WHERE item_id = ? AND shared_with_user_id = ?`,
		itemID, recipientID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// ListSharesForItem returns all recipients an item has been shared with.
func (r *ItemRepository) ListSharesForItem(ctx context.Context, itemID, ownerID uint64) ([]entity.ItemShare, error) {
	var shares []entity.ItemShare
	err := r.db.SelectContext(ctx, &shares,
		`SELECT s.*, u.email AS shared_email, u.username AS shared_name
		   FROM item_shares s
		   JOIN users u ON u.id = s.shared_with_user_id
		  WHERE s.item_id = ? AND s.owner_user_id = ?
		  ORDER BY s.created_at DESC`,
		itemID, ownerID)
	if err != nil {
		return nil, err
	}
	return shares, nil
}

// ListSharesWithUser returns all shares where the given user is the recipient,
// enriched with item and owner metadata.
func (r *ItemRepository) ListSharesWithUser(ctx context.Context, userID uint64) ([]entity.SharedItem, error) {
	var shares []entity.SharedItem
		err := r.db.SelectContext(ctx, &shares,
		`SELECT s.id, s.item_id, s.owner_user_id, s.created_at,
			        i.name AS item_name, i.path AS item_path, i.ext AS item_ext, i.mime AS item_mime, i.size AS item_size,
			        i.password_hash AS item_password_hash, i.storage_path AS item_storage_path,
			        u.email AS owner_email, u.username AS owner_name
		   FROM item_shares s
		   JOIN items i ON i.id = s.item_id
		   JOIN users u ON u.id = s.owner_user_id
		  WHERE s.shared_with_user_id = ?
		  ORDER BY s.created_at DESC`,
		userID)
	if err != nil {
		return nil, err
	}
	for i := range shares {
		shares[i].SetLocked()
	}
	return shares, nil
}

// CountShares returns how many recipients each item has been shared with.
func (r *ItemRepository) CountShares(ctx context.Context, ownerID uint64, itemIDs []uint64) (map[uint64]int, error) {
	counts := map[uint64]int{}
	if len(itemIDs) == 0 {
		return counts, nil
	}
	query := `SELECT item_id, COUNT(*) AS cnt FROM item_shares WHERE owner_user_id = ? AND item_id IN (?) GROUP BY item_id`
	query, args, err := sqlx.In(query, ownerID, itemIDs)
	if err != nil {
		return nil, err
	}
	query = r.db.Rebind(query)
	var rows []struct {
		ItemID uint64 `db:"item_id"`
		Count  int    `db:"cnt"`
	}
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	for _, row := range rows {
		counts[row.ItemID] = row.Count
	}
	return counts, nil
}

// DeleteShare removes a single share record.
func (r *ItemRepository) DeleteShare(ctx context.Context, id, ownerID uint64) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM item_shares WHERE id = ? AND owner_user_id = ?`, id, ownerID)
	return err
}

// DeleteSharesByItemIDs removes all shares for the given items.
func (r *ItemRepository) DeleteSharesByItemIDs(ctx context.Context, ownerID uint64, itemIDs []uint64) error {
	if len(itemIDs) == 0 {
		return nil
	}
	query, args, err := sqlx.In(
		`DELETE FROM item_shares WHERE owner_user_id = ? AND item_id IN (?)`,
		ownerID, itemIDs)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, r.db.Rebind(query), args...)
	return err
}
