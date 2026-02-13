package memory

import (
	"time"

	basememory "xgc-agent/memory"
)

type Session struct {
	ID        string
	CreatedAt time.Time
	Metadata  map[string]any
}

func NewSession(sessionID string) (*Session, error) {
	if sessionID == "" {
		return nil, basememory.ErrInvalidID
	}
	return &Session{ID: sessionID, CreatedAt: time.Now()}, nil
}

type SessionMemory interface {
	basememory.BaseMemory
	SessionID() string
}

type SessionStore interface {
	Open(sessionID string) (SessionMemory, error)
	Delete(sessionID string)
	Clear()
}

func NewSessionStore(ttl time.Duration, maxItems int) SessionStore {
	return NewWorkingMemorySessions(ttl, maxItems)
}
