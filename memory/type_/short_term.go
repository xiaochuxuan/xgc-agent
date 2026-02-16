package type_

import (
	"context"
	"fmt"
	"sync"
	"time"
	"xgc-agent/memory"
	"xgc-agent/memory/store"
)

// ShortTermMemory implements ShortTermMemory with TTL auto-expiry and
// sliding-window capacity limits. It delegates actual storage to an
// injected BaseMemory backend (typically inmem.InMemoryStore), while
// adding TTL assignment, capacity trimming, insertion ordering, and
// session tagging on top.
type ShortTermMemory struct {
	mu        sync.RWMutex
	userID    string
	sessionID string
	ttl       time.Duration
	capacity  int
	store     store.StoreManager // underlying storage backend
	order     *idRing            // insertion-order IDs (ring buffer)
}

// NewShortTermMemory creates a ShortTermMemory bound to the given session.
// The store parameter is the underlying storage backend (xgc-agent/memory/store).
// If store is nil, an error is returned.
//
//   - ttl:      how long each item stays alive (0 = no expiry).
//   - capacity: max number of items before oldest is evicted (0 = unlimited).
func NewShortTermMemory(userID, sessionID string, ttl time.Duration, capacity int, store store.StoreManager) (*ShortTermMemory, error) {
	if sessionID == "" {
		return nil, memory.ErrInvalidSessionID
	}
	if store == nil {
		return nil, memory.ErrStoreManagerNil
	}
	return &ShortTermMemory{
		userID:    userID,
		sessionID: sessionID,
		ttl:       ttl,
		capacity:  capacity,
		store:     store,
		order:     newIDRing(capacity),
	}, nil
}

// Store returns the underlying storage backend.
func (m *ShortTermMemory) Store() store.StoreManager { return m.store }

func (m *ShortTermMemory) SessionID() string { return m.sessionID }

func (m *ShortTermMemory) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.order == nil {
		return 0
	}
	return m.order.Len()
}

// Add inserts items, assigns IDs/TTL/type, and evicts the oldest when over
// capacity. Items are stored in the underlying store backend.
// the below fields need to be set by the caller:
//   - MemoryID: required, if empty, it will be auto-generated
//   - Content: required
//   - Metadata: optional, session_id will be added automatically
//
// The below fields will be set by this method:
//   - Type: set to MemoryShortTerm
//   - CreatedAt: set to now if zero
//   - UpdatedAt: set to now
//   - ExpiresAt: set to now+ttl if ttl > 0 and ExpiresAt is nil
func (m *ShortTermMemory) Add(ctx context.Context, items ...memory.MemoryItem) error {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()

	// cleanup expired items first to free up space
	m.cleanupExpiredLocked(ctx, now)

	// standardize and prepare items for storage, while maintaining insertion order
	prepared := make([]memory.MemoryItem, 0, len(items))
	for idx, item := range items {
		it := item.Clone()
		if it.MemoryID == "" {
			it.MemoryID = memory.GenerateMemoryID(item.Memory, item.UserID)
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
		if it.Metadata == nil {
			it.Metadata = map[string]any{}
		}
		it.Metadata["session_id"] = m.sessionID
		if !m.orderContains(it.ID) {
			m.order.PushBack(it.ID)
		}
		prepared = append(prepared, it)
	}
	// delegate storage to the backend
	if err := m.store.Add(ctx, prepared...); err != nil {
		return err
	}
	m.trimLocked(ctx)
	return nil
}

func (m *ShortTermMemory) Update(ctx context.Context, items ...MemoryItem) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, item := range items {
		existing, err := m.store.Get(ctx, item.ID)
		if err != nil {
			return ErrNotFound
		}
		if item.Content != "" {
			existing.Content = item.Content
		}
		if item.Metadata != nil {
			if existing.Metadata == nil {
				existing.Metadata = make(map[string]any)
			}
			for k, v := range item.Metadata {
				existing.Metadata[k] = v
			}
		}
		existing.UpdatedAt = now
		if err := m.store.Update(ctx, existing); err != nil {
			return err
		}
	}
	return nil
}

func (m *ShortTermMemory) Get(ctx context.Context, id string) (MemoryItem, error) {
	if err := ctx.Err(); err != nil {
		return MemoryItem{}, err
	}
	if id == "" {
		return MemoryItem{}, ErrInvalidID
	}
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupExpiredLocked(ctx, now)
	return m.store.Get(ctx, id)
}

func (m *ShortTermMemory) List(ctx context.Context, opts ListOptions) ([]MemoryItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupExpiredLocked(ctx, now)
	// iterate in insertion order for deterministic results
	out := make([]MemoryItem, 0, m.order.Len())
	for _, id := range m.order.Values() {
		item, err := m.store.Get(ctx, id)
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
		if opts.Type != "" && item.Type != opts.Type {
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
		return ErrInvalidID
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.store.Delete(ctx, id); err != nil {
		return err
	}
	m.removeOrderLocked(id)
	return nil
}

func (m *ShortTermMemory) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.store.Clear(ctx); err != nil {
		return err
	}
	m.order.Reset()
	return nil
}

// Snapshot returns all live items in creation order (oldest first).
func (m *ShortTermMemory) Snapshot(ctx context.Context) ([]MemoryItem, error) {
	return m.List(ctx, ListOptions{IncludeExpired: false})
}

// ---- internal helpers ----

// cleanupExpiredLocked removes expired items from the store and order list.
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
		m.removeOrderLocked(id)
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

func (m *ShortTermMemory) orderContains(id string) bool {
	if id == "" {
		return false
	}
	return m.order.Contains(id)
}

func (m *ShortTermMemory) removeOrderLocked(id string) {
	m.order.Remove(id)
}

// compile-time checks
var _ ShortTermMemory = (*ShortTermMemory)(nil)
