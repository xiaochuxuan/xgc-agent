// embedding_local.go: Local transformer embedding (hashed fallback).
package embedding

import "context"

// LocalTransformerEmbedding is a local embedding implementation.
// Use a simple hash-based embedding as a placeholder for a real transformer model.
// In a real implementation, this would load a local transformer model and generate embeddings.
type LocalTransformerEmbedding struct {
	dim int
}

func NewLocalTransformerEmbedding(cfgs ...EmbeddingConfig) *LocalTransformerEmbedding {
	cfg := &embeddingConfig{}
	for _, c := range cfgs {
		c(cfg)
	}
	dim := cfg.localDim
	if dim <= 0 {
		dim = DefaultLocalEmbeddingDim
	}
	return &LocalTransformerEmbedding{dim: dim}
}

func (l *LocalTransformerEmbedding) Provider() string { return "local" }

func (l *LocalTransformerEmbedding) Dimension() int { return l.dim }

func (l *LocalTransformerEmbedding) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = hashEmbedding(t, l.dim)
	}
	return out, nil
}
