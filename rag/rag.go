package rag

import (
	"context"
	"fmt"
	"xgc-agent/embedding"
	"xgc-agent/memory"
	"xgc-agent/memory/store"
)

// RAGPipeline combines chunking, embedding, and retrieval into a single pipeline.
type RAGPipeline struct {
	chunker  Chunker
	embedder embedding.EmbeddingService
	store    store.StoreManager
	userID   string
}

func NewRAGPipeline(chunker Chunker, embedder embedding.EmbeddingService, s store.StoreManager, userID string) *RAGPipeline {
	return &RAGPipeline{chunker: chunker, embedder: embedder, store: s, userID: userID}
}

// Ingest chunks a document, embeds each chunk, and stores them.
func (p *RAGPipeline) Ingest(ctx context.Context, docID, text string, metadata map[string]any) error {
	chunks := p.chunker.Chunk(text)
	for i, chunk := range chunks {
		vecs, err := p.embedder.Embed(ctx, []string{chunk})
		if err != nil {
			return fmt.Errorf("embed chunk %d: %w", i, err)
		}
		if len(vecs) == 0 {
			continue
		}
		meta := map[string]any{"doc_id": docID, "chunk_index": i}
		for k, v := range metadata {
			meta[k] = v
		}
		item := memory.MemoryItem{
			MemoryID:  fmt.Sprintf("%s_chunk_%d", docID, i),
			UserID:    p.userID,
			Memory:    &memory.Memory{Content: chunk, Metadata: meta},
			Embedding: vecs[0],
		}
		if err := p.store.Add(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// Query retrieves the top-K most relevant chunks for a query.
func (p *RAGPipeline) Query(ctx context.Context, question string, topK int) ([]Document, error) {
	qVecs, err := p.embedder.Embed(ctx, []string{question})
	if err != nil {
		return nil, err
	}
	if len(qVecs) == 0 {
		return nil, nil
	}

	all, err := p.store.List(ctx, p.userID, 0)
	if err != nil {
		return nil, err
	}

	queryVec := qVecs[0]
	type scored struct {
		doc   Document
		score float64
	}
	var results []scored
	for _, it := range all {
		if len(it.Embedding) == 0 {
			continue
		}
		s := cosine(queryVec, it.Embedding)
		content := ""
		var meta map[string]any
		if it.Memory != nil {
			content = it.Memory.Content
			meta = it.Memory.Metadata
		}
		results = append(results, scored{
			doc:   Document{ID: it.MemoryID, Content: content, Metadata: meta, Score: s},
			score: s,
		})
	}

	// Sort descending by score
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].score > results[i].score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}

	docs := make([]Document, len(results))
	for i, r := range results {
		docs[i] = r.doc
	}
	return docs, nil
}

// cosine computes the cosine similarity between two vectors.
func cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (sqrtf(na) * sqrtf(nb))
}

func sqrtf(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 20; i++ {
		z = (z + x/z) / 2
	}
	return z
}

var _ Retriever = (*RAGPipeline)(nil)

// Retrieve implements the Retriever interface.
func (p *RAGPipeline) Retrieve(ctx context.Context, query string, topK int) ([]Document, error) {
	return p.Query(ctx, query, topK)
}
