package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	basememory "xgc-agent/memory"
)

func TestSessionAbstraction_OpenAndValidation(t *testing.T) {
	store := NewSessionStore(time.Minute, 5)
	mem, err := store.Open("session-abstract")
	if err != nil {
		t.Fatalf("Open valid session: %v", err)
	}
	if mem.SessionID() != "session-abstract" {
		t.Fatalf("SessionID=%s, want session-abstract", mem.SessionID())
	}

	if _, err := store.Open(""); !errors.Is(err, basememory.ErrInvalidID) {
		t.Fatalf("Open empty session err=%v, want ErrInvalidID", err)
	}

	s, err := NewSession("session-meta")
	if err != nil {
		t.Fatalf("NewSession valid: %v", err)
	}
	if s.ID != "session-meta" {
		t.Fatalf("NewSession ID=%s, want session-meta", s.ID)
	}
	if s.CreatedAt.IsZero() {
		t.Fatalf("NewSession CreatedAt is zero")
	}

	if _, err := NewSession(""); !errors.Is(err, basememory.ErrInvalidID) {
		t.Fatalf("NewSession empty err=%v, want ErrInvalidID", err)
	}
}

func TestWorkingMemorySessions_Isolation(t *testing.T) {
	store := NewSessionStore(time.Minute, 10)
	ctx := context.Background()

	s1, err := store.Open("session-1")
	if err != nil {
		t.Fatalf("Open session-1: %v", err)
	}
	s2, err := store.Open("session-2")
	if err != nil {
		t.Fatalf("Open session-2: %v", err)
	}

	if err := s1.Add(ctx, basememory.MemoryItem{ID: "m1", Content: "hello s1"}); err != nil {
		t.Fatalf("s1 Add: %v", err)
	}
	if err := s2.Add(ctx, basememory.MemoryItem{ID: "m1", Content: "hello s2"}); err != nil {
		t.Fatalf("s2 Add: %v", err)
	}

	s1Items, err := s1.List(ctx, basememory.ListOptions{})
	if err != nil {
		t.Fatalf("s1 List: %v", err)
	}
	s2Items, err := s2.List(ctx, basememory.ListOptions{})
	if err != nil {
		t.Fatalf("s2 List: %v", err)
	}

	if len(s1Items) != 1 || s1Items[0].Content != "hello s1" {
		t.Fatalf("unexpected s1 items: %+v", s1Items)
	}
	if len(s2Items) != 1 || s2Items[0].Content != "hello s2" {
		t.Fatalf("unexpected s2 items: %+v", s2Items)
	}

	if got := s1Items[0].Metadata["session_id"]; got != "session-1" {
		t.Fatalf("s1 session_id=%v, want session-1", got)
	}
	if got := s2Items[0].Metadata["session_id"]; got != "session-2" {
		t.Fatalf("s2 session_id=%v, want session-2", got)
	}
}

func TestWorkingMemory_TTLAndEviction(t *testing.T) {
	ctx := context.Background()
	wm, err := NewWorkingMemory("session-a", 20*time.Millisecond, 2)
	if err != nil {
		t.Fatalf("NewWorkingMemory: %v", err)
	}

	if err := wm.Add(ctx, basememory.MemoryItem{ID: "a", Content: "A"}); err != nil {
		t.Fatalf("Add a: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	if err := wm.Add(ctx, basememory.MemoryItem{ID: "b", Content: "B"}); err != nil {
		t.Fatalf("Add b: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	if err := wm.Add(ctx, basememory.MemoryItem{ID: "c", Content: "C"}); err != nil {
		t.Fatalf("Add c: %v", err)
	}

	items, err := wm.List(ctx, basememory.ListOptions{})
	if err != nil {
		t.Fatalf("List after eviction: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items len=%d, want 2", len(items))
	}
	if items[0].ID != "b" || items[1].ID != "c" {
		t.Fatalf("eviction order unexpected: %+v", items)
	}

	time.Sleep(30 * time.Millisecond)
	items, err = wm.List(ctx, basememory.ListOptions{})
	if err != nil {
		t.Fatalf("List after ttl: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("items len=%d, want 0 after ttl", len(items))
	}

	_, err = wm.Get(ctx, "b")
	if !errors.Is(err, basememory.ErrNotFound) {
		t.Fatalf("Get expired err=%v, want ErrNotFound", err)
	}
}
