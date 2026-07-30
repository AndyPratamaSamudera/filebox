package handler

import (
	"context"

	"github.com/gofiber/fiber/v2"

	"filebox/internal/entity"
	"filebox/internal/service"
	"filebox/internal/utils"
)

// AuthHandler exposes authentication endpoints.
type AuthHandler struct {
	BaseHandler
	svc       *service.AuthService
	apiKeySvc *service.APIKeyService
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(svc *service.AuthService, apiKeySvc *service.APIKeyService) *AuthHandler {
	return &AuthHandler{svc: svc, apiKeySvc: apiKeySvc}
}

type registerRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type registerResponse struct {
	User   *entity.User `json:"user"`
	APIKey string       `json:"api_key,omitempty"`
}

// Register godoc
// @Summary      Register a new account
// @Description  Create a FileBox user account. A default API key is generated
// automatically for first-time users and returned only once.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      registerRequest  true  "Registration payload"
// @Success      201   {object}  utils.SuccessResponse{data=registerResponse}
// @Failure      400   {object}  utils.ErrorResponse
// @Failure      409   {object}  utils.ErrorResponse
// @Router       /auth/register [post]
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req registerRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.ValidationError(c, "invalid request body", err.Error())
	}

	user, err := h.svc.Register(c.Context(), req.Email, req.Username, req.Password)
	if err != nil {
		return respondError(c, err)
	}
	apiKey := h.ensureAPIKey(c.Context(), user.ID)
	return utils.Created(c, "user registered", registerResponse{User: user, APIKey: apiKey})
}

// ensureAPIKey creates a default API key for the user if they don't have one.
// It returns the plaintext key when a new key is created.
func (h *AuthHandler) ensureAPIKey(ctx context.Context, userID uint64) string {
	keys, err := h.apiKeySvc.List(ctx, userID)
	if err != nil || len(keys) > 0 {
		return ""
	}
	_, plaintext, err := h.apiKeySvc.Create(ctx, userID, "Default")
	if err != nil {
		return ""
	}
	return plaintext
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	User   *entity.User      `json:"user"`
	Tokens service.TokenPair `json:"tokens"`
	APIKey string            `json:"api_key,omitempty"`
}

// Login godoc
// @Summary      Log in
// @Description  Authenticate and receive access/refresh tokens
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      loginRequest  true  "Login payload"
// @Success      200   {object}  utils.SuccessResponse{data=loginResponse}
// @Failure      401   {object}  utils.ErrorResponse
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req loginRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.ValidationError(c, "invalid request body", err.Error())
	}

	pair, user, err := h.svc.Login(c.Context(), req.Email, req.Password, h.ClientIP(c), h.UserAgent(c))
	if err != nil {
		return respondError(c, err)
	}
	apiKey := h.ensureAPIKey(c.Context(), user.ID)
	return utils.JSON(c, "login successful", loginResponse{User: user, Tokens: *pair, APIKey: apiKey})
}

type tokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// Refresh godoc
// @Summary      Refresh tokens
// @Description  Rotate a refresh token into a new access/refresh pair
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      tokenRequest  true  "Refresh payload"
// @Success      200   {object}  utils.SuccessResponse{data=loginResponse}
// @Failure      401   {object}  utils.ErrorResponse
// @Router       /auth/refresh [post]
func (h *AuthHandler) Refresh(c *fiber.Ctx) error {
	var req tokenRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.ValidationError(c, "invalid request body", err.Error())
	}

	pair, user, err := h.svc.Refresh(c.Context(), req.RefreshToken, h.ClientIP(c), h.UserAgent(c))
	if err != nil {
		return respondError(c, err)
	}
	return utils.JSON(c, "token refreshed", loginResponse{User: user, Tokens: *pair})
}

// Logout godoc
// @Summary      Log out
// @Description  Revoke the session backing a refresh token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      tokenRequest  true  "Refresh payload"
// @Success      200   {object}  utils.SuccessResponse
// @Failure      401   {object}  utils.ErrorResponse
// @Security     BearerAuth
// @Router       /auth/logout [post]
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	var req tokenRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.ValidationError(c, "invalid request body", err.Error())
	}

	if err := h.svc.Logout(c.Context(), req.RefreshToken, h.ClientIP(c)); err != nil {
		return respondError(c, err)
	}
	return utils.JSON(c, "logout successful", nil)
}

// Profile godoc
// @Summary      Get current user profile
// @Description  Returns the authenticated user
// @Tags         auth
// @Produce      json
// @Success      200   {object}  utils.SuccessResponse{data=entity.User}
// @Failure      401   {object}  utils.ErrorResponse
// @Security     BearerAuth
// @Router       /profile [get]
func (h *AuthHandler) Profile(c *fiber.Ctx) error {
	userID, ok := h.UserID(c)
	if !ok {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	user, err := h.svc.GetProfile(c.Context(), userID)
	if err != nil {
		return respondError(c, err)
	}
	return utils.JSON(c, "profile fetched", user)
}
