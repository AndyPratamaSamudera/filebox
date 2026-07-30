package handler

import (
	"github.com/gofiber/fiber/v2"

	"filebox/internal/service"
	"filebox/internal/utils"
)

// AdminHandler exposes optional admin configuration endpoints.
type AdminHandler struct {
	BaseHandler
	svc *service.AdminService
}

// NewAdminHandler creates an AdminHandler.
func NewAdminHandler(svc *service.AdminService) *AdminHandler {
	return &AdminHandler{svc: svc}
}

// Config godoc
// @Summary      Get admin config
// @Description  Returns the current runtime configuration. Only available when HAS_ADMIN_PAGE=true.
// @Tags         admin
// @Produce      json
// @Success      200  {object}  utils.SuccessResponse{data=service.AdminConfig}
// @Failure      403  {object}  utils.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/config [get]
func (h *AdminHandler) Config(c *fiber.Ctx) error {
	cfg, err := h.svc.Config(c.Context())
	if err != nil {
		utils.Log.Error().Err(err).Msg("admin config failed")
		return respondError(c, err)
	}
	return utils.JSON(c, "admin config fetched", cfg)
}
