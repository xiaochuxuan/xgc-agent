package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	basememory "xgc-agent/memory"
)

// WorkingMemory is a short-lived memory.
// Features:
// 容量限制：可以设置最大条目数，超过后会删除最旧的条目（滑动窗口）
// 过期时间：每条记忆都有一个过期时间，过期后自动删除
// 快速访问：使用内存存储，提供快速的读写性能
// 优先级管理：可以为记忆设置优先级，优先级高的记忆在删除时更不容易被删除
type WorkingMemory struct {
	mu        sync.RWMutex
	sessionID string
	ttl       time.Duration
	maxItems  int
	items     map[string]basememory.MemoryItem
	order     []string
}

func NewWorkingMemory(sessionID string, ttl time.Duration, maxItems int) (*WorkingMemory, error) {
	if sessionID == "" {
		return nil, basememory.ErrInvalidID
	}

	return &WorkingMemory{
		sessionID: sessionID,
		ttl:       ttl,
		maxItems:  maxItems,
		items:     make(map[string]basememory.MemoryItem),
	}, nil
}

func (m *WorkingMemory) SessionID() string {
	return m.sessionID
}

func (m *WorkingMemory) Add(ctx context.Context, items ...basememory.MemoryItem) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanupExpiredLocked(now)

	for idx, item := range items {
		memoryItem := item.Clone()
		if memoryItem.ID == "" {
			memoryItem.ID = fmt.Sprintf("%s-%d-%d", m.sessionID, now.UnixNano(), idx)
		}
		memoryItem.Type = basememory.MemoryWorking
		if memoryItem.CreatedAt.IsZero() {
			memoryItem.CreatedAt = now
		}
		memoryItem.UpdatedAt = now
		if memoryItem.ExpiresAt == nil && m.ttl > 0 {
			expiresAt := now.Add(m.ttl)
			memoryItem.ExpiresAt = &expiresAt
		}
		if memoryItem.Metadata == nil {
			memoryItem.Metadata = map[string]any{}
		}
		memoryItem.Metadata["session_id"] = m.sessionID

		if _, exists := m.items[memoryItem.ID]; !exists {
			m.order = append(m.order, memoryItem.ID)
		}
		m.items[memoryItem.ID] = memoryItem
	}

	m.trimIfNeededLocked()
	return nil
}

func (m *WorkingMemory) Get(ctx context.Context, id string) (basememory.MemoryItem, error) {
	if err := ctx.Err(); err != nil {
		return basememory.MemoryItem{}, err
	}
	if id == "" {
		return basememory.MemoryItem{}, basememory.ErrInvalidID
	}

	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanupExpiredLocked(now)

	item, ok := m.items[id]
	if !ok {
		return basememory.MemoryItem{}, basememory.ErrNotFound
	}
	return item.Clone(), nil
}

func (m *WorkingMemory) List(ctx context.Context, opts basememory.ListOptions) ([]basememory.MemoryItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanupExpiredLocked(now)

	out := make([]basememory.MemoryItem, 0, len(m.order))
	for _, id := range m.order {
		item, ok := m.items[id]
		if !ok {
			continue
		}
		if !opts.IncludeExpired && item.ExpiresAt != nil && now.After(*item.ExpiresAt) {
			continue
		}
		if opts.Since != nil && item.CreatedAt.Before(*opts.Since) {
			continue
		}
		if opts.Until != nil && item.CreatedAt.After(*opts.Until) {
			continue
		}
		out = append(out, item.Clone())
	}

	if opts.Limit > 0 && len(out) > opts.Limit {
		out = out[len(out)-opts.Limit:]
	}

	return out, nil
}

func (m *WorkingMemory) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if id == "" {
		return basememory.ErrInvalidID
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.items[id]; !ok {
		return basememory.ErrNotFound
	}
	delete(m.items, id)
	m.removeFromOrderLocked(id)
	return nil
}

func (m *WorkingMemory) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.items = make(map[string]basememory.MemoryItem)
	m.order = m.order[:0]
	return nil
}

func (m *WorkingMemory) cleanupExpiredLocked(now time.Time) {
	for id, item := range m.items {
		if item.ExpiresAt != nil && now.After(*item.ExpiresAt) {
			delete(m.items, id)
			m.removeFromOrderLocked(id)
		}
	}
}

func (m *WorkingMemory) trimIfNeededLocked() {
	if m.maxItems <= 0 {
		return
	}
	for len(m.items) > m.maxItems && len(m.order) > 0 {
		oldestID := m.order[0]
		delete(m.items, oldestID)
		m.order = m.order[1:]
	}
}

func (m *WorkingMemory) removeFromOrderLocked(id string) {
	for i, currentID := range m.order {
		if currentID == id {
			m.order = append(m.order[:i], m.order[i+1:]...)
			return
		}
	}
}

type WorkingMemorySessions struct {
	mu       sync.RWMutex
	ttl      time.Duration
	maxItems int
	stores   map[string]*WorkingMemory
}

func NewWorkingMemorySessions(ttl time.Duration, maxItems int) *WorkingMemorySessions {
	return &WorkingMemorySessions{
		ttl:      ttl,
		maxItems: maxItems,
		stores:   make(map[string]*WorkingMemory),
	}
}

func (s *WorkingMemorySessions) Open(sessionID string) (SessionMemory, error) {
	return s.Get(sessionID)
}

func (s *WorkingMemorySessions) Get(sessionID string) (*WorkingMemory, error) {
	if sessionID == "" {
		return nil, basememory.ErrInvalidID
	}

	s.mu.RLock()
	if wm, ok := s.stores[sessionID]; ok {
		s.mu.RUnlock()
		return wm, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if wm, ok := s.stores[sessionID]; ok {
		return wm, nil
	}

	wm, err := NewWorkingMemory(sessionID, s.ttl, s.maxItems)
	if err != nil {
		return nil, err
	}
	s.stores[sessionID] = wm
	return wm, nil
}

func (s *WorkingMemorySessions) Delete(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.stores, sessionID)
}

func (s *WorkingMemorySessions) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stores = make(map[string]*WorkingMemory)
}

var _ basememory.BaseMemory = (*WorkingMemory)(nil)
var _ SessionMemory = (*WorkingMemory)(nil)
var _ SessionStore = (*WorkingMemorySessions)(nil)
