// embedding_tfidf.go: TFIDF embedding implementation.
package embedding

import "context"

// TFIDFEmbedding provides a lightweight TFIDF-style embedding.
type TFIDFEmbedding struct {
	dim int
}

func NewTFIDFEmbedding(cfgs ...EmbeddingConfig) *TFIDFEmbedding {
	cfg := &embeddingConfig{}
	for _, c := range cfgs {
		c(cfg)
	}
	dim := cfg.tfidfDim
	if dim <= 0 {
		dim = DefaultTFIDFEmbeddingDim
	}
	return &TFIDFEmbedding{dim: dim}
}

func (t *TFIDFEmbedding) Provider() string { return "tfidf" }

func (t *TFIDFEmbedding) Dimension() int { return t.dim }

func (t *TFIDFEmbedding) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := make([][]float32, len(texts))
	for i, text := range texts {
		out[i] = tfidfEmbedding(text, t.dim)
	}
	return out, nil
}
