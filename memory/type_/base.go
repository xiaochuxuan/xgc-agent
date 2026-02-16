package type_

import (
	"context"
	"time"
	"xgc-agent/memory"
)

// ---------------------------------------------------------------------------
// Layer-specific interfaces
// ---------------------------------------------------------------------------

// ShortTermMemory represents intra-session conversation history with TTL
// and sliding-window eviction.
type BaseShortTermMemory interface {
	memory.BaseMemory
	// SessionID returns the session this memory belongs to.
	SessionID() string
	// Snapshot returns all live items ordered by creation time (for promotion).
	Snapshot(ctx context.Context) ([]memory.MemoryItem, error)
	// Len returns the current number of live items.
	Len() int
}

// EpisodicMemory stores timestamped conversational episodes / events.
type EpisodicMemory interface {
	memory.BaseMemory
	// AddEpisode stores one or more episodic items.
	AddEpisode(ctx context.Context, items ...MemoryItem) error
	// ListByTimeRange returns episodes within a time range.
	ListByTimeRange(ctx context.Context, since, until time.Time, limit int) ([]MemoryItem, error)
}

// SemanticMemory stores distilled knowledge / facts with vector retrieval.
type SemanticMemory interface {
	memory.BaseMemory
	// AddKnowledge stores one or more semantic items, computing embeddings
	// automatically when absent.
	AddKnowledge(ctx context.Context, items ...MemoryItem) error
	// SearchByTopic returns items matching a topic label.
	SearchByTopic(ctx context.Context, topic string, limit int) ([]MemoryItem, error)
}

// LongTermMemory is the composite container that holds episodic + semantic
// sub-stores and exposes a unified search across both.
type LongTermMemory interface {
	memory.BaseMemory
	// Episodic returns the underlying episodic store.
	Episodic() EpisodicMemory
	// Semantic returns the underlying semantic store.
	Semantic() SemanticMemory
}
