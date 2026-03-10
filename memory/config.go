// config.go: Configuration for the memory system.
package memory

import "time"

// MemoryConfig configures the memory system across layers.
type MemoryConfig struct {
	WorkingTTL       time.Duration
	WorkingMaxItems  int
	DocumentProvider string
	DocumentDBPath   string
	VectorProvider   string
	VectorDBDSN      string
	VectorDim        int
	GraphProvider    string
	Embedding        EmbeddingConfig
}

// EmbeddingConfig configures the embedding service layer.
type EmbeddingConfig struct {
	Provider          string
	DashScopeAPIKey   string
	DashScopeEndpoint string
	DashScopeModel    string
	LocalDim          int
	TFIDFDim          int
	UseFallback       bool
}

// DefaultMemoryConfig provides sensible defaults.
func DefaultMemoryConfig() MemoryConfig {
	return MemoryConfig{
		WorkingTTL:       30 * time.Minute,
		WorkingMaxItems:  256,
		DocumentProvider: "sqlite",
		DocumentDBPath:   "./data/memory.db",
		VectorProvider:   "qdrant",
		VectorDBDSN:      "",
		VectorDim:        0,
		GraphProvider:    "neo4j",
		Embedding: EmbeddingConfig{
			Provider:    "tfidf",
			LocalDim:    384,
			TFIDFDim:    256,
			UseFallback: true,
		},
	}
}
