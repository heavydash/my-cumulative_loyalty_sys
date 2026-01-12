package storage

import "errors"

var (
	// Ошибки для balance
	ErrUserNotFound          = errors.New("user not found")
	ErrInsufficientFunds     = errors.New("insufficient funds")
	ErrOrderAlreadyWithdrawn = errors.New("order already used for withdraw")
	ErrInvalidSum            = errors.New("invalid sum")
	ErrOrderAlreadyUsed      = errors.New("order already used")

	// Ошибки для order
	ErrInvalidOrderNumber      = errors.New("invalid order number")
	ErrOrderAlreadyAddedByUser = errors.New("order already added by user")
	ErrOrderAddedAnotherUser   = errors.New("order added by another user")
	ErrOrderIsEmpty            = errors.New("order is empty")

	// Ошибки для user

	// Ошибки для storage
)
