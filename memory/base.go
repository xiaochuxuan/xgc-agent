package memory

import (
	"context"
	"time"
)

const (
	// MemoryShortTerm is for intra-session conversation history.
	MemoryShortTerm MemoryType = "short_term"
	// MemoryEpisodic is for timestamped events / episodes (long-term).
	MemoryEpisodic MemoryType = "episodic"
	// MemorySemantic is for distilled knowledge / facts (long-term).
	MemorySemantic MemoryType = "semantic"
	// MemoryPerceptual is for long-term multimodal references (image/file/audio/video).
	MemoryPerceptual MemoryType = "perceptual"
)

type MemoryType string

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
	Update(ctx context.Context, items ...MemoryItem) error
	Get(ctx context.Context, id string) (MemoryItem, error)
	List(ctx context.Context, opts ListOptions) ([]MemoryItem, error)
	Delete(ctx context.Context, id string) error
	Clear(ctx context.Context) error

	Search(ctx context.Context, query string, limit int) ([]MemoryItem, error)
}

// Memory represents a single memory entry with content and metadata.
type Memory struct {
	Content  string         `json:"content"`
	Topics   []string       `json:"topics,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// MemoryItem represents a memory item structured for storage and retrieval.
type MemoryItem struct {
	// the unique ID of the memory item
	MemoryID string `json:"memory_id"`
	// the user ID related to this memory
	UserID string `json:"user_id"`
	// the session ID this memory belongs to (for session-scoped memories)
	SessionID string `json:"session_id,omitempty"`
	// MemoryType indicates the type/layer of memory (e.g. short-term, episodic, semantic)
	Type MemoryType `json:"type,omitempty"`
	// the actual memory content
	Memory *Memory `json:"memory"`
	// timestamp when the memory was created
	CreatedAt time.Time `json:"created_at"`
	// timestamp when the memory was last updated
	UpdatedAt time.Time `json:"updated_at"`
	// optional expiration time for the memory item
	ExpiresAt *time.Time `json:"expires_at,omitempty"`

	Embedding []float32
	Score     float64 // relevance score (set by search)
	// the confidence score of the memory retrieval
	Confidence float64 `json:"confidence,omitempty"`
}

// Clone returns a shallow copy with metadata, embedding and topics copied.
func (m MemoryItem) Clone() MemoryItem {
	clone := m

	// deep copy Memory, Metadata, Embedding and Topics to avoid shared references
	if m.Memory != nil {
		memoryCopy := *m.Memory
		clone.Memory = &memoryCopy

		if m.Memory.Topics != nil {
			clone.Memory.Topics = make([]string, len(m.Memory.Topics))
			copy(clone.Memory.Topics, m.Memory.Topics)
		}
		if m.Memory.Metadata != nil {
			clone.Memory.Metadata = make(map[string]any, len(m.Memory.Metadata))
			for k, v := range m.Memory.Metadata {
				clone.Memory.Metadata[k] = v
			}
		}
	}

	if m.ExpiresAt != nil {
		expiresAtCopy := *m.ExpiresAt
		clone.ExpiresAt = &expiresAtCopy
	}

	if m.Embedding != nil {
		clone.Embedding = make([]float32, len(m.Embedding))
		copy(clone.Embedding, m.Embedding)
	}

	return clone
}

// IsExpired reports whether the item has exceeded its TTL at the given time.
func (m MemoryItem) IsExpired(now time.Time) bool {
	return m.ExpiresAt != nil && now.After(*m.ExpiresAt)
}

type BaseKey interface {
	Validate() error
}

// MemoryKey uniquely identifies a memory item within an application and user context.
type MemoryKey struct {
	// the unique ID of the memory item
	MemoryID string `json:"memory_id"`
	// the user ID related to this memory
	UserID string `json:"user_id"`
	// the session ID this memory belongs to (for session-scoped memories)
	SessionID string `json:"session_id,omitempty"`
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

// CreateMemoryItem is a helper function to create a MemoryItem with generated ID and timestamps.
func CreateMemoryItem(userID string, content string, topics []string, metadata map[string]any) *MemoryItem {
	time := time.Now()
	memoryObj := Memory{
		Content:  content,
		Topics:   topics,
		Metadata: metadata,
	}

	return &MemoryItem{
		MemoryID:  GenerateMemoryID(&memoryObj, userID),
		Memory:    &memoryObj,
		UserID:    userID,
		CreatedAt: time,
		UpdatedAt: time,
	}
}
