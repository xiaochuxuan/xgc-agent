package type_

import (
	"context"
	"time"
	"xgc-agent/memory"
	"xgc-agent/memory/store"
)

// EpisodicMemoryImpl implements EpisodicMemory backed by a StoreManager.
type EpisodicMemoryImpl struct {
	userID string
	store  store.StoreManager
}

func NewEpisodicMemory(userID string, s store.StoreManager) *EpisodicMemoryImpl {
	return &EpisodicMemoryImpl{userID: userID, store: s}
}

func (m *EpisodicMemoryImpl) Add(ctx context.Context, items ...memory.MemoryItem) error {
	for _, it := range items {
		it.Type = memory.MemoryEpisodic
		if err := m.store.Add(ctx, it); err != nil {
			return err
		}
	}
	return nil
}

func (m *EpisodicMemoryImpl) AddEpisode(ctx context.Context, items ...memory.MemoryItem) error {
	return m.Add(ctx, items...)
}

func (m *EpisodicMemoryImpl) Update(ctx context.Context, items ...memory.MemoryItem) error {
	for _, it := range items {
		if it.Memory == nil {
			continue
		}
		if err := m.store.Update(ctx, m.userID, it.MemoryID, it.Memory.Content, it.Memory.Topics, it.Memory.Metadata); err != nil {
			return err
		}
	}
	return nil
}

func (m *EpisodicMemoryImpl) Get(ctx context.Context, id string) (memory.MemoryItem, error) {
	item, err := m.store.Get(ctx, m.userID, id)
	if err != nil {
		return memory.MemoryItem{}, err
	}
	return *item, nil
}

func (m *EpisodicMemoryImpl) List(ctx context.Context, opts memory.ListOptions) ([]memory.MemoryItem, error) {
	items, err := m.store.List(ctx, m.userID, opts.Limit)
	if err != nil {
		return nil, err
	}
	out := make([]memory.MemoryItem, 0, len(items))
	for _, it := range items {
		out = append(out, *it)
	}
	return out, nil
}

func (m *EpisodicMemoryImpl) ListByTimeRange(ctx context.Context, since, until time.Time, limit int) ([]memory.MemoryItem, error) {
	items, err := m.store.List(ctx, m.userID, 0)
	if err != nil {
		return nil, err
	}
	var out []memory.MemoryItem
	for _, it := range items {
		if it.CreatedAt.Before(since) || it.CreatedAt.After(until) {
			continue
		}
		out = append(out, *it)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (m *EpisodicMemoryImpl) Delete(ctx context.Context, id string) error {
	return m.store.Delete(ctx, m.userID, id)
}

func (m *EpisodicMemoryImpl) Clear(ctx context.Context) error {
	return m.store.Clear(ctx, m.userID)
}

func (m *EpisodicMemoryImpl) Search(ctx context.Context, query string, limit int) ([]memory.MemoryItem, error) {
	return nil, memory.ErrSearchNotSupported
}

var _ EpisodicMemory = (*EpisodicMemoryImpl)(nil)
