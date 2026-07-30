package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"filebox/internal/service"
	"filebox/internal/utils"
)

// UserIDKey is the Locals key under which the JWT auth middleware stores the
// authenticated user's ID. Handlers read it via the BaseHandler.
const UserIDKey = "user_id"

// Auth validates an access token, refresh token, or API key and loads the user
// ID into Locals. The token is read from the Authorization: Bearer header,
// falling back to a ?token= query parameter so browser-initiated
// downloads/previews can authenticate. If access token validation fails, the
// token is also tried as a refresh token and finally as a raw API key.
func Auth(jwt *utils.JWTManager, apiKeySvc *service.APIKeyService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := extractToken(c)
		if token == "" {
			return utils.Error(c, fiber.StatusUnauthorized, "missing authorization")
		}

		// Try access token first.
		if claims, err := jwt.VerifyAccessToken(token); err == nil {
			c.Locals(UserIDKey, claims.UserID)
			return c.Next()
		}

		// Fall back to refresh token.
		if claims, err := jwt.VerifyRefreshToken(token); err == nil {
			c.Locals(UserIDKey, claims.UserID)
			return c.Next()
		}

		// Fall back to API key.
		if apiKeySvc != nil {
			if userID, err := apiKeySvc.Validate(c.Context(), token); err == nil {
				c.Locals(UserIDKey, userID)
				return c.Next()
			}
		}

		return utils.Error(c, fiber.StatusUnauthorized, "invalid or expired access token")
	}
}

// extractToken reads the access token from the Authorization header (with or
// without the Bearer prefix), then the ?token= query parameter, and finally the
// X-API-Key header.
func extractToken(c *fiber.Ctx) string {
	if header := c.Get(fiber.HeaderAuthorization); header != "" {
		parts := strings.SplitN(header, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") && parts[1] != "" {
			return parts[1]
		}
		// Allow raw token without Bearer prefix.
		return header
	}
	if token := c.Query("token"); token != "" {
		return token
	}
	return c.Get("X-API-Key")
}
