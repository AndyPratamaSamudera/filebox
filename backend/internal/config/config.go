package config

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all application configuration loaded from the environment.
type Config struct {
	Port                     string
	Env                      string
	AppName                  string
	DBHost                   string
	DBPort                   string
	DBUser                   string
	DBPass                   string
	DBName                   string
	JWTSecret                string
	JWTRefreshSecret         string
	JWTAccessExpirationHours int
	JWTRefreshExpirationDays int
	StoragePath              string
	UploadMaxDirect          int64
	ChunkSize                int64
	DefaultStorageQuota      uint64
	RateLimitRPS             float64
	RateLimitBurst           int
	HasAdminPage             bool
	EncryptionKey            []byte
}

const (
	defaultUploadMaxDirect int64 = 10 << 20 // 10 MB
	defaultChunkSize       int64 = 10 << 20 // 10 MB
)

// LoadConfig loads variables from .env and processes them into a Config struct.
func LoadConfig() (*Config, error) {
	// Look for .env in the current directory, then common source-layout locations
	// so the binary runs from the project root, the backend dir, or a flat deploy.
	loadEnv()

	key, err := loadEncryptionKey()
	if err != nil {
		return nil, err
	}

	return &Config{
		Port:                     getEnv("PORT"),
		Env:                      getEnv("ENV"),
		AppName:                  getEnv("APP_NAME"),
		DBHost:                   getEnv("DB_HOST"),
		DBPort:                   getEnv("DB_PORT"),
		DBUser:                   getEnv("DB_USER"),
		DBPass:                   getEnv("DB_PASS"),
		DBName:                   getEnv("DB_NAME"),
		JWTSecret:                getEnv("JWT_SECRET"),
		JWTRefreshSecret:         getEnv("JWT_REFRESH_SECRET"),
		JWTAccessExpirationHours: getEnvAsInt("JWT_ACCESS_EXPIRATION_HOURS"),
		JWTRefreshExpirationDays: getEnvAsInt("JWT_REFRESH_EXPIRATION_DAYS"),
		StoragePath:              getEnv("STORAGE_PATH"),
		UploadMaxDirect:          int64WithDefault(getEnvAsInt64("UPLOAD_MAX_DIRECT"), defaultUploadMaxDirect),
		ChunkSize:                int64WithDefault(getEnvAsInt64("CHUNK_SIZE"), defaultChunkSize),
		DefaultStorageQuota:      getEnvAsUint64("DEFAULT_STORAGE_QUOTA"),
		RateLimitRPS:             getEnvAsFloat("RATE_LIMIT_RPS"),
		RateLimitBurst:           getEnvAsInt("RATE_LIMIT_BURST"),
		HasAdminPage:             getEnvAsBool("HAS_ADMIN_PAGE"),
		EncryptionKey:            key,
	}, nil
}

// IsDevelopment reports whether the app runs in development mode.
func (c *Config) IsDevelopment() bool { return c.Env == "development" }

// loadEnv loads the first existing .env from a set of candidate paths. The
// project-root .env is the single source of truth; the legacy fallback paths
// remain so existing installs and dev workflows keep working.
// Errors are ignored so the app still runs when env vars are injected externally.
func loadEnv() {
	for _, p := range []string{".env", "backend/.env", "../.env"} {
		if _, err := os.Stat(p); err == nil {
			_ = godotenv.Load(p)
			return
		}
	}
	_ = godotenv.Load()
}

func getEnv(key string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return ""
}

func getEnvAsInt(name string) int {
	if value, err := strconv.Atoi(getEnv(name)); err == nil {
		return value
	}
	return 0
}

func getEnvAsInt64(name string) int64 {
	if value, err := strconv.ParseInt(getEnv(name), 10, 64); err == nil {
		return value
	}
	return 0
}

func getEnvAsUint64(name string) uint64 {
	if value, err := strconv.ParseUint(getEnv(name), 10, 64); err == nil {
		return value
	}
	return 0
}

func getEnvAsFloat(name string) float64 {
	if value, err := strconv.ParseFloat(getEnv(name), 64); err == nil {
		return value
	}
	return 0.0
}

func getEnvAsBool(name string) bool {
	if value, err := strconv.ParseBool(getEnv(name)); err == nil {
		return value
	}
	return false
}

func int64WithDefault(value, fallback int64) int64 {
	if value > 0 {
		return value
	}
	return fallback
}

// loadEncryptionKey reads FILEBOX_ENCRYPTION_KEY from the environment and
// validates that it decodes to a 32-byte AES-256 key (base64, hex, or raw).
func loadEncryptionKey() ([]byte, error) {
	val := getEnv("FILEBOX_ENCRYPTION_KEY")
	if val == "" {
		return nil, errors.New("FILEBOX_ENCRYPTION_KEY is required")
	}

	if key, err := base64.StdEncoding.DecodeString(val); err == nil && len(key) == 32 {
		return key, nil
	}
	if key, err := hex.DecodeString(val); err == nil && len(key) == 32 {
		return key, nil
	}
	if len(val) == 32 {
		return []byte(val), nil
	}

	return nil, fmt.Errorf("FILEBOX_ENCRYPTION_KEY must decode to 32 bytes (base64/hex/raw); got %d bytes", len(val))
}
