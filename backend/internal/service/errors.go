package service

import "errors"

// UserError wraps a user-facing validation message. Handlers map it to HTTP 400.
type UserError struct {
	Message string
}

// Error implements error.
func (e *UserError) Error() string { return e.Message }

// NewUserError wraps a message as a UserError.
func NewUserError(msg string) error { return &UserError{Message: msg} }

// IsUserError reports whether err (or any error in its chain) is a UserError.
func IsUserError(err error) bool {
	var target *UserError
	return errors.As(err, &target)
}
