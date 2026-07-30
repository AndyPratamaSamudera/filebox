package entity

import "time"

// FriendRequestStatus enumerates the possible states of a friend request.
type FriendRequestStatus string

const (
	FriendRequestPending   FriendRequestStatus = "pending"
	FriendRequestAccepted  FriendRequestStatus = "accepted"
	FriendRequestRejected  FriendRequestStatus = "rejected"
	FriendRequestCancelled FriendRequestStatus = "cancelled"
)

// FriendRequest maps to the friend_requests table.
type FriendRequest struct {
	ID              uint64              `db:"id" json:"id"`
	RequesterUserID uint64              `db:"requester_user_id" json:"requester_user_id"`
	RecipientUserID uint64              `db:"recipient_user_id" json:"recipient_user_id"`
	Status          FriendRequestStatus `db:"status" json:"status"`
	RequesterEmail  string              `db:"requester_email" json:"requester_email"`
	RequesterName   string              `db:"requester_name" json:"requester_name"`
	RecipientEmail  string              `db:"recipient_email" json:"recipient_email"`
	RecipientName   string              `db:"recipient_name" json:"recipient_name"`
	CreatedAt       time.Time           `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time           `db:"updated_at" json:"updated_at"`
}
