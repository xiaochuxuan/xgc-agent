// embedding.go: Embedding service interfaces and helpers.
package embedding

import (
	"context"
	"hash/fnv"
	"math"
	"strings"
)

const (
	DefaultLocalEmbeddingDim     = 384
	DefaultTFIDFEmbeddingDim     = 256
	DefaultDashScopeEmbeddingDim = 1024
)

// EmbeddingService provides text embeddings.
type EmbeddingService interface {
	// Provider returns the name of the embedding provider.
	Provider() string
	// Dimension returns the dimensionality of the embeddings.
	Dimension() int
	// Embed generates embeddings for the given texts.
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// hashEmbedding generates a simple hash-based embedding for the given text.
func hashEmbedding(text string, dim int) []float32 {
	vec := make([]float32, dim)
	if dim <= 0 {
		return vec
	}
	for _, token := range tokenize(text) {
		h := fnv.New32a()
		_, _ = h.Write([]byte(token))
		idx := int(h.Sum32()) % dim
		vec[idx] += 1.0
	}
	normalize(vec)
	return vec
}

// tfidfEmbedding generates a TFIDF-style embedding for the given text.
func tfidfEmbedding(text string, dim int) []float32 {
	vec := make([]float32, dim)
	if dim <= 0 {
		return vec
	}
	counts := map[int]int{}
	tokens := tokenize(text)
	for _, token := range tokens {
		h := fnv.New32a()
		_, _ = h.Write([]byte(token))
		idx := int(h.Sum32()) % dim
		counts[idx]++
	}
	for idx, count := range counts {
		vec[idx] = float32(1 + math.Log(float64(count)))
	}
	normalize(vec)
	return vec
}

// tokenize splits text into tokens.
func tokenize(text string) []string {
	text = strings.ToLower(text)
	// Split on common delimiters; this is a very basic tokenizer.
	parts := strings.FieldsFunc(text, func(r rune) bool {
		return r == ' ' || r == '\n' || r == '\t' || r == ',' || r == '.' || r == '!' || r == '?' || r == ';' || r == ':' || r == '(' || r == ')' || r == '[' || r == ']' || r == '{' || r == '}' || r == '"' || r == '\'' || r == '/' || r == '\\'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// normalize normalizes the vector to unit length.
func normalize(vec []float32) {
	var sum float32
	for _, v := range vec {
		sum += v * v
	}
	if sum == 0 {
		return
	}
	inv := float32(1.0 / math.Sqrt(float64(sum)))
	for i := range vec {
		vec[i] *= inv
	}
}

// // EmbeddingConfig configures the embedding service layer.
// type EmbeddingConfig struct {
// 	Provider          string
// 	DashScopeAPIKey   string
// 	DashScopeEndpoint string
// 	DashScopeModel    string
// 	LocalDim          int
// 	TFIDFDim          int
// 	UseFallback       bool
// }

// EmbeddingConfig configures the embedding service layer.
type embeddingConfig struct {
	provider          string
	dashScopeAPIKey   string
	dashScopeEndpoint string
	dashScopeModel    string
	localDim          int
	tfidfDim          int
	useFallback       bool
}

type EmbeddingConfig func(*embeddingConfig)

func WithEmbeddingProvider(provider string) EmbeddingConfig {
	return func(cfg *embeddingConfig) {
		cfg.provider = provider
	}
}

func WithDashScopeConfig(apiKey, endpoint, model string) EmbeddingConfig {
	return func(cfg *embeddingConfig) {
		cfg.dashScopeAPIKey = apiKey
		cfg.dashScopeEndpoint = endpoint
		cfg.dashScopeModel = model
	}
}

func WithLocalDim(dim int) EmbeddingConfig {
	return func(cfg *embeddingConfig) {
		cfg.localDim = dim
	}
}

func WithTFIDFDim(dim int) EmbeddingConfig {
	return func(cfg *embeddingConfig) {
		cfg.tfidfDim = dim
	}
}

func WithEmbeddingFallback(useFallback bool) EmbeddingConfig {
	return func(cfg *embeddingConfig) {
		cfg.useFallback = useFallback
	}
}
