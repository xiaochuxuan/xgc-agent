package manager

import (
	"context"
	"xgc-agent/embedding"
	"xgc-agent/memory"
	type_ "xgc-agent/memory/type_"
	"xgc-agent/message"
)

// MemoryManager coordinates short-term and long-term memory lifecycle.
type MemoryManager struct {
	shortTerm  type_.BaseShortTermMemory
	longTerm   type_.LongTermMemory
	embedder   embedding.EmbeddingService
	summarizer Summarizer
}

type Option func(*MemoryManager)

func WithShortTerm(st type_.BaseShortTermMemory) Option {
	return func(m *MemoryManager) { m.shortTerm = st }
}

func WithLongTerm(lt type_.LongTermMemory) Option {
	return func(m *MemoryManager) { m.longTerm = lt }
}

func WithEmbedder(e embedding.EmbeddingService) Option {
	return func(m *MemoryManager) { m.embedder = e }
}

func WithSummarizer(s Summarizer) Option {
	return func(m *MemoryManager) { m.summarizer = s }
}

func NewMemoryManager(opts ...Option) *MemoryManager {
	mm := &MemoryManager{}
	for _, o := range opts {
		o(mm)
	}
	return mm
}

// AddMessage stores a message into short-term memory.
func (m *MemoryManager) AddMessage(ctx context.Context, sessionID string, msg message.Message) error {
	if m.shortTerm == nil {
		return nil
	}
	item := memory.MemoryItem{
		Memory: &memory.Memory{
			Content:  msg.Content,
			Metadata: map[string]any{"role": string(msg.Role), "session_id": sessionID},
		},
		SessionID: sessionID,
		Type:      memory.MemoryShortTerm,
	}
	return m.shortTerm.Add(ctx, item)
}

// Retrieve performs semantic search over long-term memory.
func (m *MemoryManager) Retrieve(ctx context.Context, query string, limit int) ([]memory.MemoryItem, error) {
	if m.longTerm == nil {
		return nil, nil
	}
	return m.longTerm.Search(ctx, query, limit)
}

// Promote summarizes short-term memory and stores the summary in long-term memory.
func (m *MemoryManager) Promote(ctx context.Context, sessionID string) error {
	if m.shortTerm == nil || m.longTerm == nil {
		return nil
	}
	items, err := m.shortTerm.Snapshot(ctx)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}

	msgs := make([]message.Message, 0, len(items))
	for _, it := range items {
		if it.Memory == nil {
			continue
		}
		role := message.RoleUser
		if r, ok := it.Memory.Metadata["role"]; ok {
			role = message.Role(r.(string))
		}
		msgs = append(msgs, message.Message{Role: role, Content: it.Memory.Content})
	}

	summary := ""
	if m.summarizer != nil && len(msgs) > 0 {
		summary, err = m.summarizer.Summarize(ctx, msgs)
		if err != nil {
			return err
		}
	}
	if summary == "" {
		return nil
	}

	item := memory.MemoryItem{
		Memory: &memory.Memory{
			Content:  summary,
			Topics:   []string{"session_summary"},
			Metadata: map[string]any{"session_id": sessionID, "source": "promotion"},
		},
		SessionID: sessionID,
		Type:      memory.MemorySemantic,
	}
	return m.longTerm.Add(ctx, item)
}

// Summarize generates a summary from messages using the configured summarizer.
func (m *MemoryManager) Summarize(ctx context.Context, messages []message.Message) (string, error) {
	if m.summarizer == nil {
		return "", nil
	}
	return m.summarizer.Summarize(ctx, messages)
}
