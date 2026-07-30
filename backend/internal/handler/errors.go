package handler

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"filebox/internal/repository"
	"filebox/internal/service"
	"filebox/internal/utils"
)

// respondError maps a service/repository error to an HTTP status and writes the
// standard error envelope. Unknown errors yield 500.
func respondError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return utils.Error(c, fiber.StatusNotFound, "resource not found")

	case errors.Is(err, service.ErrInvalidCredentials):
		return utils.Error(c, fiber.StatusUnauthorized, err.Error())

	case errors.Is(err, service.ErrInvalidToken):
		return utils.Error(c, fiber.StatusUnauthorized, err.Error())

	case errors.Is(err, service.ErrEmailExists), errors.Is(err, service.ErrUsernameExists):
		return utils.Error(c, fiber.StatusConflict, err.Error())

	case isUserError(err):
		return utils.Error(c, fiber.StatusBadRequest, err.Error())

	default:
		return utils.Error(c, fiber.StatusInternalServerError, "internal server error")
	}
}

// isUserError reports whether err is a validation-style message (created via
// errors.New with a plain message). Such errors carry user-facing text and are
// safe to return as 400.
func isUserError(err error) bool {
	return service.IsUserError(err)
}
