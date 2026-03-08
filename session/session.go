package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"xgc-agent/message"
	"xgc-agent/schedule"
	"xgc-agent/tools"
)

const defaultBufferTokens = 4096

var sessionKeyCounter uint64

// Session represents a user session in runtime.
type Session struct {
	mu sync.RWMutex `json:"-"`

	ID     string `json:"id"`
	UserID string `json:"user_id"`

	Scheduler *schedule.Scheduler `json:"scheduler"`
	History   *Buffer
	Tools     *tools.Registry
	Metadata  map[string]any `json:"metadata,omitempty"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

type SessionKey struct {
	ID        string
	SessionID string
}

func (s *SessionKey) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("session key ID is required")
	}
	if s.SessionID == "" {
		return fmt.Errorf("session key SessionID is required")
	}
	return nil
}

func generateSessionKey(userID string) string {
	normalized := strings.TrimSpace(userID)
	if normalized == "" {
		normalized = "anonymous"
	}

	var randomPart [8]byte
	if _, err := rand.Read(randomPart[:]); err != nil {
		counter := atomic.AddUint64(&sessionKeyCounter, 1)
		return fmt.Sprintf("sess_%s_%d_%d", normalized, time.Now().UnixNano(), counter)
	}

	return fmt.Sprintf("sess_%s_%d_%s", normalized, time.Now().UnixNano(), hex.EncodeToString(randomPart[:]))
}

func (s *Session) Dump() ([]byte, error) {
	if s == nil {
		return nil, fmt.Errorf("session is nil")
	}

	s.mu.RLock()
	scheduler := s.Scheduler
	history := s.historyMessagesLocked()
	metadata := cloneMetadata(s.Metadata)
	bufferSize := s.bufferSizeLocked()
	createdAt := s.CreatedAt
	updatedAt := s.UpdatedAt
	id := s.ID
	userID := s.UserID
	s.mu.RUnlock()

	var schedulerState *schedule.SchedulerState
	if scheduler != nil {
		var err error
		schedulerState, err = scheduler.Snapshot()
		if err != nil {
			return nil, fmt.Errorf("dump scheduler state: %w", err)
		}
	}

	state := sessionState{
		ID:         id,
		UserID:     userID,
		Metadata:   metadata,
		History:    history,
		BufferSize: bufferSize,
		Scheduler:  schedulerState,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}

	b, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("dump session: %w", err)
	}
	return b, nil
}

func (s *Session) Load(data []byte) error {
	if s == nil {
		return fmt.Errorf("session is nil")
	}
	if len(data) == 0 {
		return fmt.Errorf("session data is empty")
	}

	var state sessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("load session: %w", err)
	}
	if strings.TrimSpace(state.ID) == "" {
		return fmt.Errorf("invalid session data: id is required")
	}
	if strings.TrimSpace(state.UserID) == "" {
		return fmt.Errorf("invalid session data: user_id is required")
	}
	if state.BufferSize <= 0 {
		state.BufferSize = defaultBufferTokens
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.ID = state.ID
	s.UserID = state.UserID
	s.Metadata = cloneMetadata(state.Metadata)
	if s.Metadata == nil {
		s.Metadata = map[string]any{}
	}

	s.History = NewBuffer(state.BufferSize, nil)
	for _, msg := range state.History {
		s.History.Append(msg)
	}

	if s.Scheduler == nil {
		s.Scheduler = schedule.NewScheduler()
	}
	if state.Scheduler != nil {
		if err := s.Scheduler.Restore(state.Scheduler); err != nil {
			return fmt.Errorf("restore scheduler state: %w", err)
		}
	}
	if s.Tools == nil {
		s.Tools = tools.NewRegistry(tools.DefaultMaxTools)
	}

	s.CreatedAt = state.CreatedAt
	s.UpdatedAt = state.UpdatedAt
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}
	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = s.CreatedAt
	}

	return nil
}

// NewSession creates a minimal session with default history buffer and tool registry.
func NewSession(userID string, bufferSize int, scheduler *schedule.Scheduler, toolRegistry *tools.Registry) (*Session, error) {
	now := time.Now()
	if bufferSize <= 0 {
		bufferSize = defaultBufferTokens
	}
	if scheduler == nil {
		scheduler = schedule.NewScheduler()
	}
	if toolRegistry == nil {
		toolRegistry = tools.NewRegistry(tools.DefaultMaxTools)
	}

	return &Session{
		ID:        generateSessionKey(userID),
		UserID:    userID,
		Scheduler: scheduler,
		History:   NewBuffer(bufferSize, nil),
		Tools:     toolRegistry,
		Metadata:  map[string]any{},
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// SessionSnapshot captures a lightweight, read-only copy of session state.
type SessionSnapshot struct {
	ID        string
	UserID    string
	Metadata  map[string]any
	History   []message.Message
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SessionStore defines persistence behavior for session payload bytes.
type SessionStore interface {
	Save(ctx context.Context, sessionID string, data []byte) error
	Load(ctx context.Context, sessionID string) ([]byte, error)
}

type sessionState struct {
	ID         string                   `json:"id"`
	UserID     string                   `json:"user_id"`
	Metadata   map[string]any           `json:"metadata,omitempty"`
	History    []message.Message        `json:"history,omitempty"`
	BufferSize int                      `json:"buffer_size"`
	Scheduler  *schedule.SchedulerState `json:"scheduler,omitempty"`
	CreatedAt  time.Time                `json:"created_at"`
	UpdatedAt  time.Time                `json:"updated_at"`
}

// Save persists current session state using the provided store.
func (s *Session) Save(ctx context.Context, store SessionStore) error {
	if store == nil {
		return fmt.Errorf("session store is nil")
	}

	b, err := s.Dump()
	if err != nil {
		return err
	}

	s.mu.RLock()
	sessionID := s.ID
	s.mu.RUnlock()
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("session id is required")
	}

	if err := store.Save(ctx, sessionID, b); err != nil {
		return fmt.Errorf("save session %s: %w", sessionID, err)
	}
	return nil
}

// LoadFromStorage loads session bytes from store then restores session state.
func (s *Session) LoadFromStorage(ctx context.Context, store SessionStore, sessionID string) error {
	if s == nil {
		return fmt.Errorf("session is nil")
	}
	if store == nil {
		return fmt.Errorf("session store is nil")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		s.mu.RLock()
		sessionID = strings.TrimSpace(s.ID)
		s.mu.RUnlock()
	}
	if sessionID == "" {
		return fmt.Errorf("session id is required")
	}

	b, err := store.Load(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("load session %s: %w", sessionID, err)
	}

	return s.Load(b)
}

func (s *Session) NewConversation() {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.History == nil {
		s.History = NewBuffer(defaultBufferTokens, nil)
	} else {
		s.History.Clear()
	}
	s.UpdatedAt = time.Now()
}

func (s *Session) AddMessage(msg *message.Message) {
	if s == nil || msg == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.History == nil {
		s.History = NewBuffer(defaultBufferTokens, nil)
	}
	s.History.Append(*msg)
	s.UpdatedAt = time.Now()
}

func (s *Session) Snapshot() *SessionSnapshot {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return &SessionSnapshot{
		ID:        s.ID,
		UserID:    s.UserID,
		Metadata:  cloneMetadata(s.Metadata),
		History:   s.historyMessagesLocked(),
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

func (s *Session) historyMessagesLocked() []message.Message {
	if s.History == nil {
		return nil
	}
	return s.History.Messages()
}

func (s *Session) bufferSizeLocked() int {
	if s.History == nil || s.History.MaxTokens() <= 0 {
		return defaultBufferTokens
	}
	return s.History.MaxTokens()
}

func cloneMetadata(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
