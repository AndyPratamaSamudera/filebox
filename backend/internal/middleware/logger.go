package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"filebox/internal/utils"
)

// Logger logs each request as a structured zerolog event with method, path,
// status, IP and latency. It is intentionally lightweight.
func Logger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()

		status := c.Response().StatusCode()
		event := utils.Log.Info()
		switch {
		case status >= 500:
			event = utils.Log.Error().Err(err)
		case status >= 400:
			event = utils.Log.Warn()
		}

		event.
			Str("method", c.Method()).
			Str("path", c.Path()).
			Int("status", status).
			Str("ip", c.IP()).
			Dur("latency", time.Since(start)).
			Msg("request")
		return err
	}
}
