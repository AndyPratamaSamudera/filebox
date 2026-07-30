package utils

import "github.com/gofiber/fiber/v2"

// SuccessResponse is the standard envelope returned for successful requests.
type SuccessResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Meta    interface{} `json:"meta,omitempty"`
}

// ErrorResponse is the standard envelope returned for failed requests.
type ErrorResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Errors  interface{} `json:"errors,omitempty"`
}

// PaginationMeta holds pagination metadata for list responses.
type PaginationMeta struct {
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
	Total int64 `json:"total"`
	Pages int   `json:"pages"`
}

// JSON sends a 200 success response.
func JSON(c *fiber.Ctx, message string, data interface{}) error {
	return c.Status(fiber.StatusOK).JSON(SuccessResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// JSONWithMeta sends a 200 success response with pagination meta.
func JSONWithMeta(c *fiber.Ctx, message string, data interface{}, meta interface{}) error {
	return c.Status(fiber.StatusOK).JSON(SuccessResponse{
		Success: true,
		Message: message,
		Data:    data,
		Meta:    meta,
	})
}

// Created sends a 201 success response.
func Created(c *fiber.Ctx, message string, data interface{}) error {
	return c.Status(fiber.StatusCreated).JSON(SuccessResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// Error sends a failed response with the given HTTP status.
func Error(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(ErrorResponse{
		Success: false,
		Message: message,
	})
}

// ValidationError sends a 422 response carrying validation errors.
func ValidationError(c *fiber.Ctx, message string, errors interface{}) error {
	return c.Status(fiber.StatusUnprocessableEntity).JSON(ErrorResponse{
		Success: false,
		Message: message,
		Errors:  errors,
	})
}
