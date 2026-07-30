package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"filebox/internal/entity"
	"filebox/internal/service"
	"filebox/internal/utils"
)

// APIKeyHandler exposes API key management endpoints.
type APIKeyHandler struct {
	BaseHandler
	svc *service.APIKeyService
}

// NewAPIKeyHandler creates an APIKeyHandler.
func NewAPIKeyHandler(svc *service.APIKeyService) *APIKeyHandler {
	return &APIKeyHandler{svc: svc}
}

type createAPIKeyRequest struct {
	Name string `json:"name"`
}

type revealAPIKeyRequest struct {
	Password string `json:"password"`
}

type apiKeyResponse struct {
	ID        uint64    `json:"id"`
	Name      string    `json:"name"`
	Key       string    `json:"key"`
	CreatedAt time.Time `json:"created_at"`
}

// Create godoc
// @Summary      Create API key
// @Description  Generate a new API key for programmatic access. The plaintext key
//
//	is returned only once in the response.
//
// @Tags         api-keys
// @Accept       json
// @Produce      json
// @Param        body  body  createAPIKeyRequest  true  "API key payload"
// @Success      201  {object}  utils.SuccessResponse{data=apiKeyResponse}
// @Failure      400  {object}  utils.ErrorResponse
// @Failure      401  {object}  utils.ErrorResponse
// @Security     BearerAuth
// @Router       /api-keys [post]
func (h *APIKeyHandler) Create(c *fiber.Ctx) error {
	userID, ok := h.UserID(c)
	if !ok {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	var req createAPIKeyRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.ValidationError(c, "invalid request body", err.Error())
	}

	key, plaintext, err := h.svc.Create(c.Context(), userID, req.Name)
	if err != nil {
		return respondError(c, err)
	}

	return utils.JSON(c, "api key created", fiber.Map{
		"id":         key.ID,
		"name":       key.Name,
		"key":        plaintext,
		"created_at": key.CreatedAt,
	})
}

// List godoc
// @Summary      List API keys
// @Description  List active API keys for the authenticated user. Key hashes are
//
//	never returned.
//
// @Tags         api-keys
// @Produce      json
// @Success      200  {object}  utils.SuccessResponse{data=[]entity.APIKey}
// @Failure      401  {object}  utils.ErrorResponse
// @Security     BearerAuth
// @Router       /api-keys [get]
func (h *APIKeyHandler) List(c *fiber.Ctx) error {
	userID, ok := h.UserID(c)
	if !ok {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	keys, err := h.svc.List(c.Context(), userID)
	if err != nil {
		return respondError(c, err)
	}
	if keys == nil {
		keys = []entity.APIKey{}
	}
	return utils.JSON(c, "api keys fetched", keys)
}

// Reveal godoc
// @Summary      Reveal API key
// @Description  Return the plaintext API key after verifying the account password.
//               The same key can be revealed multiple times; it does not need to be recreated.
// @Tags         api-keys
// @Accept       json
// @Produce      json
// @Param        id    path  uint64                true  "API Key ID"
// @Param        body  body  revealAPIKeyRequest  true  "Account password"
// @Success      200  {object}  utils.SuccessResponse{data=apiKeyResponse}
// @Failure      400  {object}  utils.ErrorResponse
// @Failure      401  {object}  utils.ErrorResponse
// @Security     BearerAuth
// @Router       /api-keys/:id/reveal [post]
func (h *APIKeyHandler) Reveal(c *fiber.Ctx) error {
	userID, ok := h.UserID(c)
	if !ok {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}
	id, err := h.ParseUintParam(c, "id")
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid api key id")
	}

	var req revealAPIKeyRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.ValidationError(c, "invalid request body", err.Error())
	}

	plaintext, err := h.svc.Reveal(c.Context(), userID, id, req.Password)
	if err != nil {
		return respondError(c, err)
	}

	return utils.JSON(c, "api key revealed", fiber.Map{
		"id":   id,
		"key":  plaintext,
		"name": "", // filled by caller from existing list
	})
}

// Revoke godoc
// @Summary      Revoke API key
// @Description  Revoke an API key so it can no longer be used.
// @Tags         api-keys
// @Produce      json
// @Param        id  path  uint64  true  "API Key ID"
// @Success      200  {object}  utils.SuccessResponse
// @Failure      404  {object}  utils.ErrorResponse
// @Security     BearerAuth
// @Router       /api-keys/:id [delete]
func (h *APIKeyHandler) Revoke(c *fiber.Ctx) error {
	userID, ok := h.UserID(c)
	if !ok {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}
	id, err := h.ParseUintParam(c, "id")
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid api key id")
	}
	if err := h.svc.Revoke(c.Context(), userID, id); err != nil {
		return respondError(c, err)
	}
	return utils.JSON(c, "api key revoked", nil)
}
