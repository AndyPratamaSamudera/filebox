package database

import (
	"fmt"

	"github.com/jmoiron/sqlx"

	_ "github.com/go-sql-driver/mysql"

	"filebox/internal/config"
)

// DSN builds a MariaDB/MySQL DSN for the given config. When multi is true the
// connection allows multiple statements per Exec — used only by the migration
// runner to apply whole migration scripts in one call.
func DSN(cfg *config.Config, multi bool) string {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci&loc=Local",
		cfg.DBUser, cfg.DBPass, cfg.DBHost, cfg.DBPort, cfg.DBName,
	)
	if multi {
		dsn += "&multiStatements=true"
	}
	return dsn
}

// Connect opens a MariaDB/MySQL connection pool tuned for a lightweight home
// server. Connection limits are intentionally modest to keep RAM low on
// low-spec devices (STB, Mini PC, Raspberry Pi).
func Connect(cfg *config.Config) (*sqlx.DB, error) {
	db, err := sqlx.Connect("mysql", DSN(cfg, false))
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}

	// Conservative pool sizing for low-RAM deployments.
	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(50)
	db.SetConnMaxLifetime(0) // reuse connections for the process lifetime

	return db, nil
}
