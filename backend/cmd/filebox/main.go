// Package main is the FileBox backend entry point.
//
// @title           FileBox API
// @version         1.0
// @description     Private cloud storage (Google Drive / NAS) for home LAN use.
// @BasePath        /api/v1
// @securityDefinitions.apikey BearerAuth
// @in              header
// @name            Authorization
// @securityDefinitions.apikey ApiKeyAuth
// @in              header
// @name            X-API-Key
package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"

	"filebox/internal/config"
	"filebox/internal/database"
	"filebox/internal/encryption"
	"filebox/internal/handler"
	"filebox/internal/repository"
	"filebox/internal/routes"
	"filebox/internal/service"
	"filebox/internal/storage"
	"filebox/internal/utils"
)

func main() {
	// 1. Load Configurations
	cfg, err := config.LoadConfig()
	if err != nil {
		panic("failed to load config: " + err.Error())
	}
	utils.InitLogger(cfg.Env)
	log := utils.Log
	log.Info().Str("env", cfg.Env).Str("app", cfg.AppName).Msg("starting FileBox")

	// 2. Open the application database pool
	// Schema is created manually by running scripts/schema.sql before first start.
	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("database connection failed")
	}
	defer db.Close()

	// 3. Initialize filesystem storage
	st, err := storage.NewLocalStorage(cfg.StoragePath)
	if err != nil {
		log.Fatal().Err(err).Msg("storage initialization failed")
	}

	// 3a. Initialize server-side encryption
	encService, err := encryption.NewService(cfg.EncryptionKey)
	if err != nil {
		log.Fatal().Err(err).Msg("encryption service initialization failed")
	}

	// 4. Build repositories
	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	itemRepo := repository.NewItemRepository(db)
	friendRepo := repository.NewFriendRepository(db)
	chunkRepo := repository.NewChunkUploadRepository(db)
	apiKeyRepo := repository.NewAPIKeyRepository(db)
	settingRepo := repository.NewSettingRepository(db)

	// 5. Build services
	jwtManager := utils.NewJWTManager(
		cfg.JWTSecret, cfg.JWTRefreshSecret,
		cfg.JWTAccessExpirationHours, cfg.JWTRefreshExpirationDays,
	)
	authService := service.NewAuthService(userRepo, sessionRepo, jwtManager, cfg)
	itemService := service.NewItemService(itemRepo, userRepo, st, encService, cfg.UploadMaxDirect)
	chunkService := service.NewChunkUploadService(chunkRepo, itemRepo, userRepo, st, encService)
	friendService := service.NewFriendService(friendRepo, userRepo)
	apiKeyService := service.NewAPIKeyService(apiKeyRepo, userRepo)
	adminService := service.NewAdminService(cfg, settingRepo, userRepo)

	// 6. Build handlers
	authHandler := handler.NewAuthHandler(authService, apiKeyService)
	itemHandler := handler.NewItemHandler(itemService, cfg)
	chunkHandler := handler.NewChunkHandler(chunkService, cfg)
	friendHandler := handler.NewFriendHandler(friendService)
	apiKeyHandler := handler.NewAPIKeyHandler(apiKeyService)
	adminHandler := handler.NewAdminHandler(adminService)

	// 7. Initialize Fiber + register routes
	app := fiber.New(fiber.Config{
		AppName:      cfg.AppName,
		ReadTimeout:  5 * time.Minute,
		WriteTimeout: 5 * time.Minute,
		BodyLimit:    bodyLimit(cfg.UploadMaxDirect),
		ErrorHandler: errorHandler,
	})
	routes.Register(app, routes.Deps{
		Config:        cfg,
		JWT:           jwtManager,
		RateLimitMax:  cfg.RateLimitBurst,
		AuthHandler:   authHandler,
		ItemHandler:   itemHandler,
		ChunkHandler:  chunkHandler,
		FriendHandler: friendHandler,
		APIKeyHandler: apiKeyHandler,
		APIKeyService: apiKeyService,
		AdminHandler:  adminHandler,
	})

	// 8. Start serving
	go func() {
		log.Info().Str("port", cfg.Port).Msg("HTTP server listening")
		if err := app.Listen(":" + cfg.Port); err != nil {
			log.Fatal().Err(err).Msg("server failed")
		}
	}()

	// 9. GRACEFUL SHUTDOWN on interrupt / terminate
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Error().Err(err).Msg("graceful shutdown failed")
	}
	log.Info().Msg("FileBox stopped")
}

// errorHandler renders unhandled errors as the standard JSON envelope.
func errorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	msg := "internal server error"
	var e *fiber.Error
	if errors.As(err, &e) {
		code = e.Code
		msg = e.Message
	}
	return utils.Error(c, code, msg)
}

// bodyLimit picks a request body limit with headroom over the direct upload cap.
func bodyLimit(uploadMax int64) int {
	const min = 4 << 20 // 4MB floor
	limit := uploadMax + (4 << 20)
	if limit < min {
		limit = min
	}
	return int(limit)
}
