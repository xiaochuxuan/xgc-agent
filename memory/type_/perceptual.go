package type_

import (
	"context"
	"strings"
	"time"
	"xgc-agent/memory"
	"xgc-agent/memory/store"
)

// PerceptualMemoryImpl stores multimodal references in long-term memory.
// Raw blobs should be stored externally; this layer keeps normalized metadata.
type PerceptualMemoryImpl struct {
	userID string
	store  store.StoreManager
}

func NewPerceptualMemory(userID string, s store.StoreManager) *PerceptualMemoryImpl {
	return &PerceptualMemoryImpl{userID: userID, store: s}
}

func (m *PerceptualMemoryImpl) Add(ctx context.Context, items ...memory.MemoryItem) error {
	now := time.Now()
	for _, item := range items {
		it := item.Clone()
		if it.UserID == "" {
			it.UserID = m.userID
		}
		if it.Memory == nil {
			it.Memory = &memory.Memory{}
		}
		if it.MemoryID == "" {
			it.MemoryID = memory.GenerateMemoryID(it.Memory, it.UserID)
		}
		if it.CreatedAt.IsZero() {
			it.CreatedAt = now
		}
		it.UpdatedAt = now
		it.Type = memory.MemoryPerceptual
		if it.Memory.Metadata == nil {
			it.Memory.Metadata = map[string]any{}
		}
		if _, ok := it.Memory.Metadata["modality"]; !ok {
			it.Memory.Metadata["modality"] = "text"
		}
		it.Memory.Metadata["memory_type"] = string(memory.MemoryPerceptual)
		if err := m.store.Add(ctx, it); err != nil {
			return err
		}
	}
	return nil
}

func (m *PerceptualMemoryImpl) AddPerception(ctx context.Context, items ...memory.MemoryItem) error {
	return m.Add(ctx, items...)
}

func (m *PerceptualMemoryImpl) Update(ctx context.Context, items ...memory.MemoryItem) error {
	for _, it := range items {
		if it.Memory == nil {
			continue
		}
		if it.Memory.Metadata == nil {
			it.Memory.Metadata = map[string]any{}
		}
		it.Memory.Metadata["memory_type"] = string(memory.MemoryPerceptual)
		if err := m.store.Update(ctx, m.userID, it.MemoryID, it.Memory.Content, it.Memory.Topics, it.Memory.Metadata); err != nil {
			return err
		}
	}
	return nil
}

func (m *PerceptualMemoryImpl) Get(ctx context.Context, id string) (memory.MemoryItem, error) {
	item, err := m.store.Get(ctx, m.userID, id)
	if err != nil {
		return memory.MemoryItem{}, err
	}
	return *item, nil
}

func (m *PerceptualMemoryImpl) List(ctx context.Context, opts memory.ListOptions) ([]memory.MemoryItem, error) {
	items, err := m.store.List(ctx, m.userID, opts.Limit)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	out := make([]memory.MemoryItem, 0, len(items))
	for _, it := range items {
		if it == nil || !isPerceptualType(*it) {
			continue
		}
		if !opts.IncludeExpired && it.IsExpired(now) {
			continue
		}
		if opts.Since != nil && it.CreatedAt.Before(*opts.Since) {
			continue
		}
		if opts.Until != nil && it.CreatedAt.After(*opts.Until) {
			continue
		}
		out = append(out, it.Clone())
	}
	if opts.Limit > 0 && len(out) > opts.Limit {
		out = out[:opts.Limit]
	}
	return out, nil
}

func (m *PerceptualMemoryImpl) ListByModality(ctx context.Context, modality string, limit int) ([]memory.MemoryItem, error) {
	all, err := m.List(ctx, memory.ListOptions{})
	if err != nil {
		return nil, err
	}
	modality = strings.ToLower(strings.TrimSpace(modality))
	if modality == "" {
		if limit > 0 && len(all) > limit {
			return all[:limit], nil
		}
		return all, nil
	}
	out := make([]memory.MemoryItem, 0, len(all))
	for _, it := range all {
		if it.Memory == nil || it.Memory.Metadata == nil {
			continue
		}
		mod, _ := it.Memory.Metadata["modality"].(string)
		if strings.EqualFold(strings.TrimSpace(mod), modality) {
			out = append(out, it)
		}
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (m *PerceptualMemoryImpl) Delete(ctx context.Context, id string) error {
	return m.store.Delete(ctx, m.userID, id)
}

func (m *PerceptualMemoryImpl) Clear(ctx context.Context) error {
	return m.store.Clear(ctx, m.userID)
}

func (m *PerceptualMemoryImpl) Search(ctx context.Context, query string, limit int) ([]memory.MemoryItem, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil, nil
	}
	all, err := m.List(ctx, memory.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]memory.MemoryItem, 0, len(all))
	for _, it := range all {
		if it.Memory == nil {
			continue
		}
		if strings.Contains(strings.ToLower(it.Memory.Content), q) {
			it.Score = 0.5
			out = append(out, it)
		}
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func isPerceptualType(item memory.MemoryItem) bool {
	if item.Type == memory.MemoryPerceptual {
		return true
	}
	if item.Memory == nil || item.Memory.Metadata == nil {
		return false
	}
	if t, ok := item.Memory.Metadata["memory_type"].(string); ok {
		return strings.EqualFold(strings.TrimSpace(t), string(memory.MemoryPerceptual))
	}
	return false
}

var _ PerceptualMemory = (*PerceptualMemoryImpl)(nil)
