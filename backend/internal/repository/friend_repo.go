package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"filebox/internal/entity"
)

// FriendRepository accesses the friends and friend_requests tables.
type FriendRepository struct {
	db *sqlx.DB
}

// NewFriendRepository creates a FriendRepository.
func NewFriendRepository(db *sqlx.DB) *FriendRepository {
	return &FriendRepository{db: db}
}

// CreateRequest inserts a pending friend request.
func (r *FriendRepository) CreateRequest(ctx context.Context, requesterUserID, recipientUserID uint64) (*entity.FriendRequest, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO friend_requests (requester_user_id, recipient_user_id, status) VALUES (?, ?, 'pending')`,
		requesterUserID, recipientUserID)
	if err != nil {
		return nil, fmt.Errorf("insert friend request: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return r.GetRequestByID(ctx, uint64(id))
}

// GetRequestByID returns a request enriched with both users' metadata.
func (r *FriendRepository) GetRequestByID(ctx context.Context, id uint64) (*entity.FriendRequest, error) {
	var req entity.FriendRequest
	err := r.db.GetContext(ctx, &req,
		`SELECT fr.*,
				ru.email AS requester_email, ru.username AS requester_name,
				uu.email AS recipient_email, uu.username AS recipient_name
		   FROM friend_requests fr
		   JOIN users ru ON ru.id = fr.requester_user_id
		   JOIN users uu ON uu.id = fr.recipient_user_id
		  WHERE fr.id = ?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &req, nil
}

// FindRequest returns an existing request between two users in a specific direction.
func (r *FriendRepository) FindRequest(ctx context.Context, requesterUserID, recipientUserID uint64) (*entity.FriendRequest, error) {
	var req entity.FriendRequest
	err := r.db.GetContext(ctx, &req,
		`SELECT fr.*,
				ru.email AS requester_email, ru.username AS requester_name,
				uu.email AS recipient_email, uu.username AS recipient_name
		   FROM friend_requests fr
		   JOIN users ru ON ru.id = fr.requester_user_id
		   JOIN users uu ON uu.id = fr.recipient_user_id
		  WHERE fr.requester_user_id = ? AND fr.recipient_user_id = ?`,
		requesterUserID, recipientUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &req, nil
}

// ListRequests returns pending requests for a user, optionally filtered by direction.
func (r *FriendRepository) ListRequests(ctx context.Context, userID uint64, direction string) ([]entity.FriendRequest, error) {
	var reqs []entity.FriendRequest
	var query string
	var args []interface{}
	switch direction {
	case "incoming":
		query = `SELECT fr.*,
					 ru.email AS requester_email, ru.username AS requester_name,
					 uu.email AS recipient_email, uu.username AS recipient_name
			   FROM friend_requests fr
			   JOIN users ru ON ru.id = fr.requester_user_id
			   JOIN users uu ON uu.id = fr.recipient_user_id
			  WHERE fr.recipient_user_id = ? AND fr.status = 'pending'
			  ORDER BY fr.created_at DESC`
		args = append(args, userID)
	case "outgoing":
		query = `SELECT fr.*,
					 ru.email AS requester_email, ru.username AS requester_name,
					 uu.email AS recipient_email, uu.username AS recipient_name
			   FROM friend_requests fr
			   JOIN users ru ON ru.id = fr.requester_user_id
			   JOIN users uu ON uu.id = fr.recipient_user_id
			  WHERE fr.requester_user_id = ? AND fr.status = 'pending'
			  ORDER BY fr.created_at DESC`
		args = append(args, userID)
	default:
		query = `SELECT fr.*,
					 ru.email AS requester_email, ru.username AS requester_name,
					 uu.email AS recipient_email, uu.username AS recipient_name
			   FROM friend_requests fr
			   JOIN users ru ON ru.id = fr.requester_user_id
			   JOIN users uu ON uu.id = fr.recipient_user_id
			  WHERE (fr.requester_user_id = ? OR fr.recipient_user_id = ?) AND fr.status = 'pending'
			  ORDER BY fr.created_at DESC`
		args = append(args, userID, userID)
	}
	err := r.db.SelectContext(ctx, &reqs, query, args...)
	if err != nil {
		return nil, err
	}
	return reqs, nil
}

// UpdateRequestStatus updates the status of a friend request.
func (r *FriendRepository) UpdateRequestStatus(ctx context.Context, id uint64, status entity.FriendRequestStatus) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE friend_requests SET status = ? WHERE id = ?`, string(status), id)
	return err
}

// DeleteRequest removes a friend request.
func (r *FriendRepository) DeleteRequest(ctx context.Context, id uint64) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM friend_requests WHERE id = ?`, id)
	return err
}

// Create adds a friend relationship. It returns the enriched row joined with the
// friend's user metadata.
func (r *FriendRepository) Create(ctx context.Context, userID, friendUserID uint64) (*entity.Friend, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO friends (user_id, friend_user_id) VALUES (?, ?)`,
		userID, friendUserID)
	if err != nil {
		return nil, fmt.Errorf("insert friend: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, uint64(id))
}

// GetByID returns an enriched friend row.
func (r *FriendRepository) GetByID(ctx context.Context, id uint64) (*entity.Friend, error) {
	var f entity.Friend
	err := r.db.GetContext(ctx, &f,
		`SELECT f.*, u.email AS friend_email, u.username AS friend_name
		   FROM friends f
		   JOIN users u ON u.id = f.friend_user_id
		  WHERE f.id = ?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// List returns all accepted friends of a user, enriched with friend metadata.
// Because accepted friendships are stored in both directions, this list is mutual.
func (r *FriendRepository) List(ctx context.Context, userID uint64) ([]entity.Friend, error) {
	var friends []entity.Friend
	err := r.db.SelectContext(ctx, &friends,
		`SELECT f.*, u.email AS friend_email, u.username AS friend_name
		   FROM friends f
		   JOIN users u ON u.id = f.friend_user_id
		  WHERE f.user_id = ?
		  ORDER BY u.username ASC`, userID)
	if err != nil {
		return nil, err
	}
	return friends, nil
}

// Find returns an existing friend relationship for the given direction, or ErrNotFound.
func (r *FriendRepository) Find(ctx context.Context, userID, friendUserID uint64) (*entity.Friend, error) {
	var f entity.Friend
	err := r.db.GetContext(ctx, &f,
		`SELECT f.*, u.email AS friend_email, u.username AS friend_name
		   FROM friends f
		   JOIN users u ON u.id = f.friend_user_id
		  WHERE f.user_id = ? AND f.friend_user_id = ?`,
		userID, friendUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// FindEither returns an existing friend relationship in either direction.
func (r *FriendRepository) FindEither(ctx context.Context, userA, userB uint64) (*entity.Friend, error) {
	var f entity.Friend
	err := r.db.GetContext(ctx, &f,
		`SELECT f.*, u.email AS friend_email, u.username AS friend_name
		   FROM friends f
		   JOIN users u ON u.id = f.friend_user_id
		  WHERE (f.user_id = ? AND f.friend_user_id = ?)
		     OR (f.user_id = ? AND f.friend_user_id = ?)
		  LIMIT 1`,
		userA, userB, userB, userA)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// Delete removes a friend relationship by its ID for the given user.
func (r *FriendRepository) Delete(ctx context.Context, id, userID uint64) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM friends WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// DeleteByUsers removes both directions of a friendship between two users.
func (r *FriendRepository) DeleteByUsers(ctx context.Context, userA, userB uint64) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM friends
		  WHERE (user_id = ? AND friend_user_id = ?)
		     OR (user_id = ? AND friend_user_id = ?)`,
		userA, userB, userB, userA)
	return err
}

// DeleteRequestByUsers removes pending friend requests between two users in either direction.
func (r *FriendRepository) DeleteRequestByUsers(ctx context.Context, userA, userB uint64) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM friend_requests
		  WHERE status = 'pending'
		    AND ((requester_user_id = ? AND recipient_user_id = ?)
		         OR (requester_user_id = ? AND recipient_user_id = ?))`,
		userA, userB, userB, userA)
	return err
}
