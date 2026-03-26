package store

import (
	"context"
	"xgc-agent/memory"
)

// Store persists conversation memories for one or more sessions.
// Implementations should ensure thread safety and efficient retrieval of memories.
type StoreManager interface {
	// Add adds a message to the store. It should return an error if the store limit is exceeded.
	Add(ctx context.Context, item memory.MemoryItem) error
	// Get retrieves a memory entry by its ID. It should return an error if the entry does not exist.
	Get(ctx context.Context, userID string, memoryID string) (*memory.MemoryItem, error)
	// Update updates an existing memory entry. It should return an error if the entry does not exist.
	Update(ctx context.Context, userID string, memoryID string, content string, topic []string, metadata map[string]any) error
	// Delete removes a memory entry by its ID. It should return an error if the entry does not exist.
	Delete(ctx context.Context, userID string, memoryID string) error
	// Clear removes all memory entries for a user. It should return an error if the user does not exist.
	Clear(ctx context.Context, userID string) error
	// List returns messages in chronological order (oldest -> newest).
	// If limit <= 0, it returns all messages.
	List(ctx context.Context, userID string, limit int) ([]*memory.MemoryItem, error)
	// Search performs vector similarity retrieval for a user's memories.
	// If limit <= 0, implementations may return all matched items.
	Search(ctx context.Context, userID string, queryEmbedding []float32, limit int) ([]*memory.MemoryItem, error)
}
