// errors.go: Common errors for memory system.
package memory

import "errors"

var (
	ErrNotFound             = errors.New("memory: not found")
	ErrInvalidID            = errors.New("memory: invalid id")
	ErrUserIDRequired       = errors.New("memory: user ID is required")
	ErrMemoryIDRequired     = errors.New("memory: memory ID is required")
	ErrSearchNotSupported   = errors.New("memory: search not supported")
	ErrEmbeddingUnavailable = errors.New("memory: embedding unavailable")
	ErrMemoryLimitExceeded  = errors.New("memory: memory limit exceeded")
)
