package type_

import (
	"context"
	"time"
	"xgc-agent/memory"
)

// ---------------------------------------------------------------------------
// Layer-specific interfaces
// ---------------------------------------------------------------------------

// BaseShortTermMemory represents intra-session conversation history with TTL
// and sliding-window eviction.
type BaseShortTermMemory interface {
	memory.BaseMemory
	SessionID() string
	Snapshot(ctx context.Context) ([]memory.MemoryItem, error)
	Len() int
}

// EpisodicMemory stores timestamped conversational episodes / events.
type EpisodicMemory interface {
	memory.BaseMemory
	AddEpisode(ctx context.Context, items ...memory.MemoryItem) error
	ListByTimeRange(ctx context.Context, since, until time.Time, limit int) ([]memory.MemoryItem, error)
}

// SemanticMemory stores distilled knowledge / facts with vector retrieval.
type SemanticMemory interface {
	memory.BaseMemory
	AddKnowledge(ctx context.Context, items ...memory.MemoryItem) error
	SearchByTopic(ctx context.Context, topic string, limit int) ([]memory.MemoryItem, error)
}

// LongTermMemory is the composite container that holds episodic + semantic
// sub-stores and exposes a unified search across both.
type LongTermMemory interface {
	memory.BaseMemory
	Episodic() EpisodicMemory
	Semantic() SemanticMemory
}
