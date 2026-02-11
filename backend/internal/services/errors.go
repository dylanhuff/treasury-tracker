package services

import "errors"

var (
	ErrHoldingNotFound     = errors.New("holding not found")
	ErrUnauthorized        = errors.New("unauthorized: holding does not belong to user")
	ErrInsufficientBalance = errors.New("insufficient balance")
)

// ValidationError represents invalid input from the client.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}
