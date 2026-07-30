package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"filebox/internal/middleware"
)

// BaseHandler provides shared request helpers used by feature handlers.
type BaseHandler struct{}

// UserID returns the authenticated user ID stored in Locals by the auth
// middleware. The boolean is false when no user is present.
func (h *BaseHandler) UserID(c *fiber.Ctx) (uint64, bool) {
	if v, ok := c.Locals(middleware.UserIDKey).(uint64); ok {
		return v, true
	}
	return 0, false
}

// ClientIP returns the request peer IP (honoring trusted proxy headers).
func (h *BaseHandler) ClientIP(c *fiber.Ctx) string {
	return c.IP()
}

// UserAgent returns the request User-Agent header.
func (h *BaseHandler) UserAgent(c *fiber.Ctx) string {
	return c.Get(fiber.HeaderUserAgent)
}

// ParseUintParam parses a route parameter as uint64.
func (h *BaseHandler) ParseUintParam(c *fiber.Ctx, key string) (uint64, error) {
	return strconv.ParseUint(c.Params(key), 10, 64)
}

// OptionalUintQuery returns a query parameter as a *uint64, or nil if absent.
func (h *BaseHandler) OptionalUintQuery(c *fiber.Ctx, key string) *uint64 {
	s := c.Query(key)
	if s == "" {
		return nil
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return nil
	}
	return &v
}
