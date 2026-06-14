package domain

import "errors"

var (
	// ErrNotFound is returned by stores when an entity does not exist in cache.
	ErrNotFound = errors.New("not found")

	// ErrInvalidTransition is returned when a group lifecycle transition is not
	// permitted by the rules.
	ErrInvalidTransition = errors.New("invalid lifecycle transition")

	// ErrValidation wraps user-input validation problems.
	ErrValidation = errors.New("validation error")

	// ErrUnauthorized means the caller is not authenticated.
	ErrUnauthorized = errors.New("unauthorized")

	// ErrForbidden means the caller is authenticated but not allowed.
	ErrForbidden = errors.New("forbidden")

	// ErrConflict means the request collides with current state (e.g. duplicate).
	ErrConflict = errors.New("conflict")
)
