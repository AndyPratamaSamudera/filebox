package utils

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

// Log is the centralized structured logger used across the application.
var Log zerolog.Logger

// InitLogger initializes the global zerolog logger. Development mode uses a
// pretty console writer; production emits fast JSON to stderr.
func InitLogger(env string) {
	zerolog.TimeFieldFormat = time.RFC3339

	if env == "development" {
		Log = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"}).
			With().
			Timestamp().
			Logger()
	} else {
		Log = zerolog.New(os.Stderr).
			With().
			Timestamp().
			Logger()
	}
}

// LogAuthAttempt logs a user authentication event (login/register/refresh).
func LogAuthAttempt(event string, email string, ip string, userAgent string, success bool, reason string) {
	e := Log.Info()
	if !success {
		e = Log.Warn()
	}
	e.
		Str("event", event).
		Str("email", email).
		Str("ip", ip).
		Str("user_agent", userAgent).
		Bool("success", success).
		Str("reason", reason).
		Msgf("%s for email: %s - Success: %v", event, email, success)
}

// LogActivity logs a user file/folder activity (upload/download/delete/rename).
func LogActivity(userID uint64, action string, targetType string, targetID uint64, ip string) {
	Log.Info().
		Str("event", "activity").
		Uint64("user_id", userID).
		Str("action", action).
		Str("target_type", targetType).
		Uint64("target_id", targetID).
		Str("ip", ip).
		Msgf("Activity %s by user %d on %s %d", action, userID, targetType, targetID)
}
