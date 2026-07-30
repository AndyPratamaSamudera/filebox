package service

import (
	"context"
	"fmt"
	"strconv"

	"filebox/internal/config"
	"filebox/internal/repository"
)

// AdminConfig is the aggregated configuration exposed to the admin page.
type AdminConfig struct {
	DatabaseHost      string `json:"database_host"`
	DatabasePort      int    `json:"database_port"`
	DatabaseName      string `json:"database_name"`
	UploadMaxDirect   int64  `json:"upload_max_direct"`
	ChunkSize         int64  `json:"chunk_size"`
	StoragePath       string `json:"storage_path"`
	DefaultUserQuota  uint64 `json:"default_user_quota"`
	HasAdminPage      bool   `json:"has_admin_page"`
}

// AdminService provides operations for the optional admin page.
type AdminService struct {
	cfg     *config.Config
	setRepo *repository.SettingRepository
	userRepo *repository.UserRepository
}

// NewAdminService creates an AdminService.
func NewAdminService(cfg *config.Config, setRepo *repository.SettingRepository, userRepo *repository.UserRepository) *AdminService {
	return &AdminService{cfg: cfg, setRepo: setRepo, userRepo: userRepo}
}

// Config returns the current runtime configuration merged with persisted settings.
func (s *AdminService) Config(ctx context.Context) (*AdminConfig, error) {
	settings, err := s.setRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list settings: %w", err)
	}
	m := make(map[string]string, len(settings))
	for _, st := range settings {
		if st.Value != nil {
			m[st.Key] = *st.Value
		}
	}

	cfg := &AdminConfig{
		DatabaseHost:     coalesce(m["database_host"], s.cfg.DBHost),
		DatabasePort:     int(coalesceInt64(m["database_port"], parseInt64(s.cfg.DBPort))),
		DatabaseName:     coalesce(m["database_name"], s.cfg.DBName),
		UploadMaxDirect:  int64(coalesceInt64(m["upload_max_direct"], int64(s.cfg.UploadMaxDirect))),
		ChunkSize:        int64(coalesceInt64(m["chunk_size"], int64(s.cfg.ChunkSize))),
		StoragePath:      coalesce(m["storage_path"], s.cfg.StoragePath),
		DefaultUserQuota: s.cfg.DefaultStorageQuota,
		HasAdminPage:     s.cfg.HasAdminPage,
	}
	return cfg, nil
}

// Set persists a setting value.
func (s *AdminService) Set(ctx context.Context, key, value string) error {
	return s.setRepo.Set(ctx, key, value)
}

// UpdateUserTotalStorage updates a user's total_storage quota.
func (s *AdminService) UpdateUserTotalStorage(ctx context.Context, userID uint64, totalStorage uint64) error {
	if _, err := s.userRepo.GetByID(ctx, userID); err != nil {
		return err
	}
	_, err := s.userRepo.DB().ExecContext(ctx, "UPDATE users SET total_storage = ? WHERE id = ?", totalStorage, userID)
	return err
}

func coalesce(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}

func coalesceInt64(s string, fallback int64) int64 {
	if s == "" {
		return fallback
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fallback
	}
	return v
}

func parseInt64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}
