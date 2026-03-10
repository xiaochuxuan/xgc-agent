package rag

import "strings"

// Chunker splits text into smaller chunks for embedding and retrieval.
type Chunker interface {
	Chunk(text string) []string
}

// FixedSizeChunker splits text into fixed-size chunks with optional overlap.
type FixedSizeChunker struct {
	ChunkSize int
	Overlap   int
}

func NewFixedSizeChunker(size, overlap int) *FixedSizeChunker {
	if size <= 0 {
		size = 512
	}
	if overlap < 0 || overlap >= size {
		overlap = 0
	}
	return &FixedSizeChunker{ChunkSize: size, Overlap: overlap}
}

func (c *FixedSizeChunker) Chunk(text string) []string {
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}
	var chunks []string
	step := c.ChunkSize - c.Overlap
	if step <= 0 {
		step = c.ChunkSize
	}
	for i := 0; i < len(runes); i += step {
		end := i + c.ChunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
		if end == len(runes) {
			break
		}
	}
	return chunks
}

// ParagraphChunker splits text by double newlines (paragraphs).
type ParagraphChunker struct{}

func (c *ParagraphChunker) Chunk(text string) []string {
	parts := strings.Split(text, "\n\n")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

var _ Chunker = (*FixedSizeChunker)(nil)
var _ Chunker = (*ParagraphChunker)(nil)
