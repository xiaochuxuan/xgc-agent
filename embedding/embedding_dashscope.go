// embedding_dashscope.go: DashScope embedding (stub with optional fallback).
package embedding

import (
	"context"
)

// DashScopeEmbedding represents a remote DashScope embedding service.
type DashScopeEmbedding struct {
	apiKey   string
	endpoint string
	model    string
	dim      int
	fallback bool
}

func NewDashScopeEmbedding(cfgs ...EmbeddingConfig) *DashScopeEmbedding {
	cfg := &embeddingConfig{}
	for _, c := range cfgs {
		c(cfg)
	}
	dim := cfg.localDim
	if dim <= 0 {
		dim = DefaultDashScopeEmbeddingDim
	}
	return &DashScopeEmbedding{
		apiKey:   cfg.dashScopeAPIKey,
		endpoint: cfg.dashScopeEndpoint,
		model:    cfg.dashScopeModel,
		dim:      dim,
		fallback: cfg.useFallback,
	}
}

func (d *DashScopeEmbedding) Provider() string { return "dashscope" }

func (d *DashScopeEmbedding) Dimension() int { return d.dim }

func (d *DashScopeEmbedding) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if d.apiKey == "" {
		if d.fallback {
			return embedWithFallback(texts, d.dim), nil
		}
		return nil, ErrDashScopeUnavailable
	}
	// TODO: integrate DashScope API; fallback to hashing for now.
	return embedWithFallback(texts, d.dim), nil
}

func embedWithFallback(texts []string, dim int) [][]float32 {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = hashEmbedding(t, dim)
	}
	return out
}
