package memory

import (
	"context"
	"time"
)

// ListOptions filters list results.
type ListOptions struct {
	Limit          int
	Since          *time.Time
	Until          *time.Time
	IncludeExpired bool
}

// BaseMemory defines common behaviors for all memory types.
type BaseMemory interface {
	Add(ctx context.Context, items ...MemoryItem) error
	Get(ctx context.Context, id string) (MemoryItem, error)
	List(ctx context.Context, opts ListOptions) ([]MemoryItem, error)
	Delete(ctx context.Context, id string) error
	Clear(ctx context.Context) error
}

// SearchableMemory extends BaseMemory with semantic search.
type SearchableMemory interface {
	BaseMemory
	Search(ctx context.Context, query string, limit int) ([]MemoryItem, error)
}

// Memory represents a single memory entry with content and metadata.
type Memory struct {
	Content  string         `json:"content"`
	Topics   []string       `json:"topics,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// MemoryEntry represents a memory item structured for storage and retrieval.
type MemoryEntry struct {
	// the unique ID of the memory item
	MemoryID string `json:"memory_id"`
	// the confidence score of the memory retrieval
	Confidence float64 `json:"confidence"`
	// the actual memory content
	Memory *Memory `json:"memory"`
	// the user ID related to this memory
	UserID string `json:"user_id"`
	// timestamp when the memory was created
	CreatedAt time.Time `json:"created_at"`
	// timestamp when the memory was last updated
	UpdatedAt time.Time `json:"updated_at"`
}

type BaseKey interface {
	Validate() error
}

// MemoryKey uniquely identifies a memory item within an application and user context.
type MemoryKey struct {
	// the user ID related to this memory
	UserID string `json:"user_id"`
	// the unique ID of the memory item
	MemoryID string `json:"memory_id"`
}

func (k *MemoryKey) Validate() error {
	if k.UserID == "" {
		return ErrUserIDRequired
	}
	if k.MemoryID == "" {
		return ErrMemoryIDRequired
	}
	return nil
}
