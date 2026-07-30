package service

import (
	"context"
	"errors"
	"strings"

	"filebox/internal/entity"
	"filebox/internal/repository"
)

// FriendService manages the user's contact list and friend requests.
type FriendService struct {
	friendRepo *repository.FriendRepository
	userRepo   *repository.UserRepository
}

// NewFriendService creates a FriendService.
func NewFriendService(friendRepo *repository.FriendRepository, userRepo *repository.UserRepository) *FriendService {
	return &FriendService{
		friendRepo: friendRepo,
		userRepo:   userRepo,
	}
}

// Add sends a friend request by email.
func (s *FriendService) Add(ctx context.Context, userID uint64, email string) (*entity.FriendRequest, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return nil, NewUserError("email is required")
	}

	friend, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, NewUserError("user not found")
		}
		return nil, err
	}
	if friend.ID == userID {
		return nil, NewUserError("cannot add yourself as a friend")
	}

	// Already accepted friends?
	if _, err := s.friendRepo.FindEither(ctx, userID, friend.ID); err == nil {
		return nil, NewUserError("already friends")
	} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	// Re-use or revive an existing request between the same users.
	if existing, err := s.friendRepo.FindRequest(ctx, userID, friend.ID); err == nil && existing != nil {
		switch existing.Status {
		case entity.FriendRequestPending:
			return existing, nil
		case entity.FriendRequestAccepted:
			return nil, NewUserError("already friends")
		}
		if err := s.friendRepo.UpdateRequestStatus(ctx, existing.ID, entity.FriendRequestPending); err != nil {
			return nil, err
		}
		return s.friendRepo.GetRequestByID(ctx, existing.ID)
	} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	return s.friendRepo.CreateRequest(ctx, userID, friend.ID)
}

// List returns the user's accepted friends.
func (s *FriendService) List(ctx context.Context, userID uint64) ([]entity.Friend, error) {
	return s.friendRepo.List(ctx, userID)
}

// ListRequests returns pending friend requests for a user.
func (s *FriendService) ListRequests(ctx context.Context, userID uint64, direction string) ([]entity.FriendRequest, error) {
	return s.friendRepo.ListRequests(ctx, userID, direction)
}

// AcceptRequest accepts an incoming friend request and creates a mutual friendship.
func (s *FriendService) AcceptRequest(ctx context.Context, userID, requestID uint64) (*entity.FriendRequest, error) {
	req, err := s.friendRepo.GetRequestByID(ctx, requestID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, NewUserError("friend request not found")
		}
		return nil, err
	}
	if req.RecipientUserID != userID {
		return nil, NewUserError("not authorized to accept this request")
	}
	if req.Status != entity.FriendRequestPending {
		return nil, NewUserError("request is not pending")
	}

	// Create mutual friendship if not already present.
	if _, err := s.friendRepo.Find(ctx, req.RequesterUserID, req.RecipientUserID); errors.Is(err, repository.ErrNotFound) {
		if _, err := s.friendRepo.Create(ctx, req.RequesterUserID, req.RecipientUserID); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	if _, err := s.friendRepo.Find(ctx, req.RecipientUserID, req.RequesterUserID); errors.Is(err, repository.ErrNotFound) {
		if _, err := s.friendRepo.Create(ctx, req.RecipientUserID, req.RequesterUserID); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	if err := s.friendRepo.UpdateRequestStatus(ctx, requestID, entity.FriendRequestAccepted); err != nil {
		return nil, err
	}
	return s.friendRepo.GetRequestByID(ctx, requestID)
}

// RejectRequest rejects an incoming friend request.
func (s *FriendService) RejectRequest(ctx context.Context, userID, requestID uint64) (*entity.FriendRequest, error) {
	req, err := s.friendRepo.GetRequestByID(ctx, requestID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, NewUserError("friend request not found")
		}
		return nil, err
	}
	if req.RecipientUserID != userID {
		return nil, NewUserError("not authorized to reject this request")
	}
	if req.Status != entity.FriendRequestPending {
		return nil, NewUserError("request is not pending")
	}
	if err := s.friendRepo.UpdateRequestStatus(ctx, requestID, entity.FriendRequestRejected); err != nil {
		return nil, err
	}
	return s.friendRepo.GetRequestByID(ctx, requestID)
}

// CancelRequest cancels an outgoing friend request.
func (s *FriendService) CancelRequest(ctx context.Context, userID, requestID uint64) error {
	req, err := s.friendRepo.GetRequestByID(ctx, requestID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return NewUserError("friend request not found")
		}
		return err
	}
	if req.RequesterUserID != userID {
		return NewUserError("not authorized to cancel this request")
	}
	if req.Status != entity.FriendRequestPending {
		return NewUserError("request is not pending")
	}
	return s.friendRepo.DeleteRequest(ctx, requestID)
}

// Delete removes a friend relationship (mutual). It returns an error if the
// relationship does not exist for the current user.
func (s *FriendService) Delete(ctx context.Context, userID, id uint64) error {
	friend, err := s.friendRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return NewUserError("friend not found")
		}
		return err
	}
	if friend.UserID != userID {
		return NewUserError("friend not found")
	}
	// Remove both directions and mark any related request as rejected.
	if err := s.friendRepo.DeleteByUsers(ctx, userID, friend.FriendUserID); err != nil {
		return err
	}
	// Clean up pending requests in either direction.
	_ = s.friendRepo.DeleteRequestByUsers(ctx, userID, friend.FriendUserID)
	return nil
}
