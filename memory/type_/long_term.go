package type_

import (
	"context"
	"xgc-agent/memory"
)

// LongTermMemoryImpl combines episodic and semantic memory into a unified long-term store.
type LongTermMemoryImpl struct {
	episodic EpisodicMemory
	semantic SemanticMemory
}

func NewLongTermMemory(episodic EpisodicMemory, semantic SemanticMemory) *LongTermMemoryImpl {
	return &LongTermMemoryImpl{episodic: episodic, semantic: semantic}
}

func (m *LongTermMemoryImpl) Episodic() EpisodicMemory { return m.episodic }
func (m *LongTermMemoryImpl) Semantic() SemanticMemory { return m.semantic }

func (m *LongTermMemoryImpl) Add(ctx context.Context, items ...memory.MemoryItem) error {
	for _, it := range items {
		switch it.Type {
		case memory.MemoryEpisodic:
			if err := m.episodic.Add(ctx, it); err != nil {
				return err
			}
		default:
			// Default to semantic
			if err := m.semantic.Add(ctx, it); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *LongTermMemoryImpl) Update(ctx context.Context, items ...memory.MemoryItem) error {
	for _, it := range items {
		switch it.Type {
		case memory.MemoryEpisodic:
			if err := m.episodic.Update(ctx, it); err != nil {
				return err
			}
		default:
			if err := m.semantic.Update(ctx, it); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *LongTermMemoryImpl) Get(ctx context.Context, id string) (memory.MemoryItem, error) {
	item, err := m.semantic.Get(ctx, id)
	if err == nil {
		return item, nil
	}
	return m.episodic.Get(ctx, id)
}

func (m *LongTermMemoryImpl) List(ctx context.Context, opts memory.ListOptions) ([]memory.MemoryItem, error) {
	sem, _ := m.semantic.List(ctx, opts)
	epi, _ := m.episodic.List(ctx, opts)
	return append(sem, epi...), nil
}

func (m *LongTermMemoryImpl) Delete(ctx context.Context, id string) error {
	err1 := m.semantic.Delete(ctx, id)
	err2 := m.episodic.Delete(ctx, id)
	if err1 != nil && err2 != nil {
		return err1
	}
	return nil
}

func (m *LongTermMemoryImpl) Clear(ctx context.Context) error {
	if err := m.semantic.Clear(ctx); err != nil {
		return err
	}
	return m.episodic.Clear(ctx)
}

// Search performs semantic search across long-term memory.
func (m *LongTermMemoryImpl) Search(ctx context.Context, query string, limit int) ([]memory.MemoryItem, error) {
	return m.semantic.Search(ctx, query, limit)
}

var _ LongTermMemory = (*LongTermMemoryImpl)(nil)
