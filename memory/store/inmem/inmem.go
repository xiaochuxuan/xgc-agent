package inmem

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
	"xgc-agent/memory"
)

// InMemoryStore keeps messages in memory. It is fast but not durable.
// It represents memories manager for a specific app.
type InMemoryStore struct {
	mu       sync.RWMutex
	memories map[string]map[string]*memory.MemoryItem // userID -> memoryID -> MemoryEntry
	// the manager options
	options storeOptions
}

// createMemoryEntry creates a MemoryEntry from the given parameters.
func createMemoryEntry(userID string, content string, topics []string, metadata map[string]any) *memory.MemoryItem {
	time := time.Now()
	memoryObj := &memory.Memory{
		Content:  content,
		Topics:   topics,
		Metadata: metadata,
	}

	return &memory.MemoryItem{
		MemoryID:  memory.GenerateMemoryID(memoryObj, userID),
		Memory:    memoryObj,
		UserID:    userID,
		CreatedAt: time,
		UpdatedAt: time,
	}
}

// NewInMemoryManager creates a new InMemoryManager with the given options.
func NewInMemoryStore(opts ...StoreOptions) *InMemoryStore {
	options := storeOptions{
		memoryLimit: 100,
	}
	for _, opt := range opts {
		opt(&options)
	}

	return &InMemoryStore{
		memories: make(map[string]map[string]*memory.MemoryItem),
		options:  options,
	}
}

func (s *InMemoryStore) Add(ctx context.Context, userID string, content string,
	topics []string, metadata map[string]any) error {

	if userID == "" {
		return memory.ErrUserIDRequired
	}

	memoryEntry := createMemoryEntry(userID, content, topics, metadata)

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.memories[userID]) >= s.options.memoryLimit {
		return errors.New("memory: memory limit exceeded for user " + userID)
	}
	if s.memories[userID] == nil {
		s.memories[userID] = make(map[string]*memory.MemoryItem)
	}
	s.memories[userID][memoryEntry.MemoryID] = memoryEntry
	return nil
}

func (s *InMemoryStore) Get(ctx context.Context, userID string, memoryID string) (*memory.MemoryItem, error) {
	if userID == "" {
		return nil, memory.ErrUserIDRequired
	}
	if memoryID == "" {
		return nil, memory.ErrMemoryIDRequired
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	userMemories, ok := s.memories[userID]
	if !ok {
		return nil, memory.ErrNotFound
	}

	memoryEntry, ok := userMemories[memoryID]
	if !ok {
		return nil, memory.ErrNotFound
	}

	return memoryEntry, nil
}

func (s *InMemoryStore) Update(ctx context.Context, userID string, memoryID string,
	content string, topic []string, metadata map[string]any) error {
	if userID == "" {
		return memory.ErrUserIDRequired
	}
	if memoryID == "" {
		return memory.ErrMemoryIDRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	userMemories, ok := s.memories[userID]
	if !ok {
		return memory.ErrNotFound
	}
	memoryEntry, ok := userMemories[memoryID]
	if !ok {
		return memory.ErrNotFound
	}

	now := time.Now()
	memoryEntry.Memory.Content = content
	memoryEntry.Memory.Topics = topic
	memoryEntry.Memory.Metadata = metadata
	memoryEntry.UpdatedAt = now

	s.memories[userID][memoryID] = memoryEntry
	return nil
}

func (s *InMemoryStore) Delete(ctx context.Context, userID string, memoryID string) error {
	if userID == "" {
		return memory.ErrUserIDRequired
	}
	if memoryID == "" {
		return memory.ErrMemoryIDRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	userMemories, ok := s.memories[userID]
	if !ok {
		return memory.ErrNotFound
	}
	if _, ok := userMemories[memoryID]; !ok {
		return memory.ErrNotFound
	}
	delete(s.memories[userID], memoryID)
	return nil
}

func (s *InMemoryStore) Clear(ctx context.Context, userID string) error {
	if userID == "" {
		return memory.ErrUserIDRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.memories, userID)
	return nil
}

func (s *InMemoryStore) List(ctx context.Context, userID string, limit int) ([]*memory.MemoryItem, error) {
	_ = ctx
	if userID == "" {
		return nil, memory.ErrInvalidID
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	userMemories, ok := s.memories[userID]
	if !ok {
		return nil, nil
	}

	var out []*memory.MemoryItem
	// If limit <= 0, return all memories. Otherwise, return up to the limit.
	if limit <= 0 || len(userMemories) <= limit {
		limit = len(userMemories)
	}
	out = make([]*memory.MemoryItem, 0, limit)

	count := 0
	for _, memory := range userMemories {
		out = append(out, memory)
		count++
		if count >= limit {
			break
		}
	}

	// Sort by updated time (newest first), tie-breaker by created time.
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})

	return out, nil
}

func (s *InMemoryStore) Search(ctx context.Context, userID string, queryEmbedding []float32, limit int) ([]*memory.MemoryItem, error) {
	_ = ctx
	_ = queryEmbedding
	_ = limit
	if userID == "" {
		return nil, memory.ErrUserIDRequired
	}
	return nil, memory.ErrSearchNotSupported
}
