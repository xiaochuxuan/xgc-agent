package rag

import "context"

// Document represents a retrieved document chunk.
type Document struct {
	ID       string
	Content  string
	Metadata map[string]any
	Score    float64
}

// Retriever retrieves relevant documents for a query.
type Retriever interface {
	Retrieve(ctx context.Context, query string, topK int) ([]Document, error)
}
