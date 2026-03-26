package type_

import (
	"context"
	"sync"
	"time"
	"xgc-agent/memory"
	"xgc-agent/memory/store"
)

// ShortTermMemory implements BaseShortTermMemory with TTL auto-expiry and
// sliding-window capacity limits. It delegates actual storage to an
// injected StoreManager backend, while adding TTL assignment, capacity
// trimming, insertion ordering, and session tagging on top.
type ShortTermMemory struct {
	mu        sync.RWMutex
	userID    string
	sessionID string
	ttl       time.Duration
	capacity  int
	store     store.StoreManager
	order     *idRing
}

func NewShortTermMemory(userID, sessionID string, ttl time.Duration, capacity int, s store.StoreManager) (*ShortTermMemory, error) {
	if sessionID == "" {
		return nil, memory.ErrInvalidSessionID
	}
	if s == nil {
		return nil, memory.ErrStoreManagerNil
	}
	return &ShortTermMemory{
		userID:    userID,
		sessionID: sessionID,
		ttl:       ttl,
		capacity:  capacity,
		store:     s,
		order:     newIDRing(capacity),
	}, nil
}

func (m *ShortTermMemory) Store() store.StoreManager { return m.store }
func (m *ShortTermMemory) SessionID() string         { return m.sessionID }

func (m *ShortTermMemory) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.order == nil {
		return 0
	}
	return m.order.Len()
}

func (m *ShortTermMemory) Add(ctx context.Context, items ...memory.MemoryItem) error {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanupExpiredLocked(ctx, now)

	for _, item := range items {
		it := item.Clone()
		if it.Memory == nil {
			it.Memory = &memory.Memory{}
		}
		if it.UserID == "" {
			it.UserID = m.userID
		}
		it.SessionID = m.sessionID
		if it.MemoryID == "" {
			it.MemoryID = memory.GenerateMemoryID(it.Memory, it.UserID)
		}
		it.Type = memory.MemoryShortTerm
		if it.CreatedAt.IsZero() {
			it.CreatedAt = now
		}
		it.UpdatedAt = now
		if it.ExpiresAt == nil && m.ttl > 0 {
			exp := now.Add(m.ttl)
			it.ExpiresAt = &exp
		}
		if it.Memory != nil {
			if it.Memory.Metadata == nil {
				it.Memory.Metadata = map[string]any{}
			}
			it.Memory.Metadata["user_id"] = it.UserID
			it.Memory.Metadata["session_id"] = m.sessionID
		}
		if !m.order.Contains(it.MemoryID) {
			m.order.PushBack(it.MemoryID)
		}
		if err := m.store.Add(ctx, it); err != nil {
			return err
		}
	}
	m.trimLocked(ctx)
	return nil
}

func (m *ShortTermMemory) Update(ctx context.Context, items ...memory.MemoryItem) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, item := range items {
		if item.Memory == nil {
			continue
		}
		if err := m.store.Update(ctx, m.userID, item.MemoryID, item.Memory.Content, item.Memory.Topics, item.Memory.Metadata); err != nil {
			return err
		}
	}
	return nil
}

func (m *ShortTermMemory) Get(ctx context.Context, id string) (memory.MemoryItem, error) {
	if err := ctx.Err(); err != nil {
		return memory.MemoryItem{}, err
	}
	if id == "" {
		return memory.MemoryItem{}, memory.ErrInvalidID
	}
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupExpiredLocked(ctx, now)
	item, err := m.store.Get(ctx, m.userID, id)
	if err != nil {
		return memory.MemoryItem{}, err
	}
	return *item, nil
}

func (m *ShortTermMemory) List(ctx context.Context, opts memory.ListOptions) ([]memory.MemoryItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupExpiredLocked(ctx, now)

	out := make([]memory.MemoryItem, 0, m.order.Len())
	for _, id := range m.order.Values() {
		item, err := m.store.Get(ctx, m.userID, id)
		if err != nil {
			continue
		}
		if !opts.IncludeExpired && item.IsExpired(now) {
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

func (m *ShortTermMemory) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if id == "" {
		return memory.ErrInvalidID
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.store.Delete(ctx, m.userID, id); err != nil {
		return err
	}
	m.order.Remove(id)
	return nil
}

func (m *ShortTermMemory) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.store.Clear(ctx, m.userID); err != nil {
		return err
	}
	m.order.Reset()
	return nil
}

func (m *ShortTermMemory) Search(ctx context.Context, query string, limit int) ([]memory.MemoryItem, error) {
	return nil, memory.ErrSearchNotSupported
}

func (m *ShortTermMemory) Snapshot(ctx context.Context) ([]memory.MemoryItem, error) {
	return m.List(ctx, memory.ListOptions{IncludeExpired: false})
}

func (m *ShortTermMemory) cleanupExpiredLocked(ctx context.Context, now time.Time) {
	var expired []string
	for _, memId := range m.order.Values() {
		item, err := m.store.Get(ctx, m.userID, memId)
		if err != nil {
			expired = append(expired, memId)
			continue
		}
		if item.IsExpired(now) {
			expired = append(expired, memId)
		}
	}
	for _, id := range expired {
		_ = m.store.Delete(ctx, m.userID, id)
		m.order.Remove(id)
	}
}

func (m *ShortTermMemory) trimLocked(ctx context.Context) {
	if m.capacity <= 0 {
		return
	}
	for m.order.Len() > m.capacity {
		oldestID, ok := m.order.PopFront()
		if !ok {
			break
		}
		_ = m.store.Delete(ctx, m.userID, oldestID)
	}
}

var _ BaseShortTermMemory = (*ShortTermMemory)(nil)
