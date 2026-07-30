package entity

import "time"

// Friend represents a contact relationship. Sharing works as soon as a friend
// is added and the friendship is mutual.
type Friend struct {
	ID           uint64    `db:"id" json:"id"`
	UserID       uint64    `db:"user_id" json:"user_id"`
	FriendUserID uint64    `db:"friend_user_id" json:"friend_user_id"`
	FriendEmail  string    `db:"friend_email" json:"friend_email"`
	FriendName   string    `db:"friend_name" json:"friend_name"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
}
