package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"filebox/internal/config"
	"filebox/internal/entity"
	"filebox/internal/repository"
	"filebox/internal/utils"
)

// Auth-domain errors. Handlers map these to HTTP status codes.
var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailExists        = errors.New("email already registered")
	ErrUsernameExists     = errors.New("username already taken")
	ErrInvalidToken       = errors.New("invalid or expired token")
)

// TokenPair is the response shape for issued access/refresh tokens.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// AuthService orchestrates registration, login, refresh and logout.
type AuthService struct {
	userRepo    *repository.UserRepository
	sessionRepo *repository.SessionRepository
	jwt         *utils.JWTManager
	cfg         *config.Config
}

// NewAuthService creates an AuthService.
func NewAuthService(
	userRepo *repository.UserRepository,
	sessionRepo *repository.SessionRepository,
	jwt *utils.JWTManager,
	cfg *config.Config,
) *AuthService {
	return &AuthService{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		jwt:         jwt,
		cfg:         cfg,
	}
}

// Register creates a new user account.
func (s *AuthService) Register(ctx context.Context, email, username, password string) (*entity.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	username = strings.TrimSpace(username)

	if !isValidEmail(email) {
		return nil, NewUserError("invalid email format")
	}
	if len(username) < 3 || len(username) > 64 {
		return nil, NewUserError("username must be 3-64 characters")
	}
	if len(password) < 8 {
		return nil, NewUserError("password must be at least 8 characters")
	}

	if existing, err := s.userRepo.GetByEmail(ctx, email); err == nil && existing != nil {
		return nil, ErrEmailExists
	} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	if existing, err := s.userRepo.GetByUsername(ctx, username); err == nil && existing != nil {
		return nil, ErrUsernameExists
	} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	hash, err := utils.HashPassword(password)
	if err != nil {
		return nil, err
	}

	return s.userRepo.Create(ctx, email, username, hash, s.defaultQuota())
}

func (s *AuthService) defaultQuota() uint64 {
	if s.cfg != nil && s.cfg.DefaultStorageQuota > 0 {
		return s.cfg.DefaultStorageQuota
	}
	return 0
}

// isValidName checks that a string contains only safe filename characters.
func isValidName(name string) bool {
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			continue
		}
		return false
	}
	return len(name) > 0
}

// Login validates credentials and issues a fresh token pair.
func (s *AuthService) Login(ctx context.Context, email, password, ip, userAgent string) (*TokenPair, *entity.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			utils.LogAuthAttempt("login", email, ip, userAgent, false, "user not found")
			return nil, nil, ErrInvalidCredentials
		}
		return nil, nil, err
	}

	if !utils.CheckPassword(password, user.PasswordHash) {
		utils.LogAuthAttempt("login", email, ip, userAgent, false, "bad password")
		return nil, nil, ErrInvalidCredentials
	}
	if user.Status != entity.UserStatusActive {
		utils.LogAuthAttempt("login", email, ip, userAgent, false, "inactive account")
		return nil, nil, ErrInvalidCredentials
	}

	pair, err := s.issueTokens(ctx, user, ip, userAgent)
	if err != nil {
		return nil, nil, err
	}

	_ = s.userRepo.UpdateLastLogin(ctx, user.ID)
	utils.LogAuthAttempt("login", email, ip, userAgent, true, "ok")

	return pair, user, nil
}

// Refresh rotates a refresh token into a new access/refresh pair.
func (s *AuthService) Refresh(ctx context.Context, refreshToken, ip, userAgent string) (*TokenPair, *entity.User, error) {
	claims, err := s.jwt.VerifyRefreshToken(refreshToken)
	if err != nil {
		return nil, nil, ErrInvalidToken
	}

	session, err := s.sessionRepo.GetByTokenHash(ctx, utils.HashToken(refreshToken))
	if err != nil {
		return nil, nil, ErrInvalidToken
	}
	if s.sessionRepo.IsExpired(session) {
		return nil, nil, ErrInvalidToken
	}

	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, nil, ErrInvalidToken
	}

	// Rotate: revoke the old session before issuing a new one.
	if err := s.sessionRepo.Revoke(ctx, session.TokenHash); err != nil {
		return nil, nil, err
	}

	pair, err := s.issueTokens(ctx, user, ip, userAgent)
	if err != nil {
		return nil, nil, err
	}
	return pair, user, nil
}

// Logout revokes the session backing a refresh token.
func (s *AuthService) Logout(ctx context.Context, refreshToken, ip string) error {
	if _, err := s.jwt.VerifyRefreshToken(refreshToken); err != nil {
		return ErrInvalidToken
	}
	if err := s.sessionRepo.Revoke(ctx, utils.HashToken(refreshToken)); err != nil {
		return err
	}
	return nil
}

// GetProfile returns the current user.
func (s *AuthService) GetProfile(ctx context.Context, userID uint64) (*entity.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}
	return user, nil
}

// issueTokens generates a new access/refresh pair and persists the session.
func (s *AuthService) issueTokens(ctx context.Context, user *entity.User, ip, userAgent string) (*TokenPair, error) {
	access, err := s.jwt.GenerateAccessToken(user.ID, user.Email, user.Username, string(user.Role))
	if err != nil {
		return nil, err
	}

	sessionID := uuid.NewString()
	refresh, err := s.jwt.GenerateRefreshToken(user.ID, user.Email, user.Username, string(user.Role), sessionID)
	if err != nil {
		return nil, err
	}

	ua, iptr := &userAgent, &ip
	if userAgent == "" {
		ua = nil
	}
	if ip == "" {
		iptr = nil
	}
	session := &entity.Session{
		UserID:    user.ID,
		TokenHash: utils.HashToken(refresh),
		UserAgent: ua,
		IP:        iptr,
		ExpiresAt: time.Now().Add(s.jwt.RefreshTTL()),
	}
	if _, err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(s.jwt.AccessTTL().Seconds()),
	}, nil
}

// isValidEmail performs a lightweight email sanity check.
func isValidEmail(email string) bool {
	at := strings.IndexByte(email, '@')
	if at <= 0 || at == len(email)-1 {
		return false
	}
	return strings.IndexByte(email[at+1:], '.') != -1
}
