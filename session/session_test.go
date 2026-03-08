package session

import (
	"context"
	"path/filepath"
	"testing"
	"xgc-agent/message"
	"xgc-agent/schedule"
)

func TestNewSessionDefaults(t *testing.T) {
	s, err := NewSession("user-1", 0, nil, nil)
	if err != nil {
		t.Fatalf("NewSession error = %v", err)
	}
	if s == nil {
		t.Fatalf("NewSession returned nil session")
	}
	if s.ID == "" {
		t.Fatalf("expected non-empty session ID")
	}
	if s.UserID != "user-1" {
		t.Fatalf("expected user id user-1, got %s", s.UserID)
	}
	if s.Scheduler == nil {
		t.Fatalf("expected scheduler initialized")
	}
	if s.Tools == nil {
		t.Fatalf("expected tool registry initialized")
	}
	if s.History == nil {
		t.Fatalf("expected history initialized")
	}
	if s.History.MaxTokens() != defaultBufferTokens {
		t.Fatalf("expected default buffer size %d, got %d", defaultBufferTokens, s.History.MaxTokens())
	}
}

func TestAddMessageAndSnapshotIsolation(t *testing.T) {
	s, err := NewSession("user-2", 256, nil, nil)
	if err != nil {
		t.Fatalf("NewSession error = %v", err)
	}
	s.Metadata["topic"] = "demo"

	msg := message.NewMessage(message.RoleUser, "hello session")
	s.AddMessage(&msg)

	snap := s.Snapshot()
	if snap == nil {
		t.Fatalf("expected non-nil snapshot")
	}
	if len(snap.History) != 1 {
		t.Fatalf("expected snapshot history length 1, got %d", len(snap.History))
	}
	if snap.Metadata["topic"] != "demo" {
		t.Fatalf("expected metadata copied")
	}

	snap.Metadata["topic"] = "changed"
	snap.History[0].Content = "changed content"

	if s.Metadata["topic"] != "demo" {
		t.Fatalf("expected session metadata unaffected by snapshot mutation")
	}
	liveHistory := s.History.Messages()
	if len(liveHistory) != 1 || liveHistory[0].Content != "hello session" {
		t.Fatalf("expected session history unaffected by snapshot mutation")
	}
}

func TestDumpLoadRoundTrip(t *testing.T) {
	origin, err := NewSession("user-3", 128, nil, nil)
	if err != nil {
		t.Fatalf("NewSession error = %v", err)
	}
	origin.Metadata["k1"] = "v1"
	origin.Metadata["num"] = 42
	origin.Scheduler.GraphState.Context.Set("route", "alpha")
	origin.Scheduler.Entry.State = schedule.NodeFinished
	origin.AddMessage(&message.Message{Role: message.RoleUser, Content: "first"})
	origin.AddMessage(&message.Message{Role: message.RoleAssistant, Content: "second"})

	blob, err := origin.Dump()
	if err != nil {
		t.Fatalf("Dump error = %v", err)
	}
	if len(blob) == 0 {
		t.Fatalf("expected non-empty dumped bytes")
	}

	target := &Session{}
	if err := target.Load(blob); err != nil {
		t.Fatalf("Load error = %v", err)
	}

	if target.ID != origin.ID {
		t.Fatalf("expected id %s, got %s", origin.ID, target.ID)
	}
	if target.UserID != origin.UserID {
		t.Fatalf("expected user id %s, got %s", origin.UserID, target.UserID)
	}
	if target.Metadata["k1"] != "v1" {
		t.Fatalf("expected metadata k1=v1")
	}
	if target.History == nil {
		t.Fatalf("expected history initialized")
	}
	msgs := target.History.Messages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 history messages, got %d", len(msgs))
	}
	if msgs[0].Content != "first" || msgs[1].Content != "second" {
		t.Fatalf("unexpected history content after load")
	}
	if target.Scheduler == nil {
		t.Fatalf("expected scheduler initialized during load")
	}
	if target.Tools == nil {
		t.Fatalf("expected tools initialized during load")
	}
	v, ok := target.Scheduler.GraphState.Context.Get("route")
	if !ok || v != "alpha" {
		t.Fatalf("expected scheduler context route=alpha after load, got %v (exists=%v)", v, ok)
	}
	if target.Scheduler.Entry.State != schedule.NodeFinished {
		t.Fatalf("expected scheduler entry state restored")
	}
}

func TestNewConversationClearsHistory(t *testing.T) {
	s, err := NewSession("user-4", 64, nil, nil)
	if err != nil {
		t.Fatalf("NewSession error = %v", err)
	}
	s.AddMessage(&message.Message{Role: message.RoleUser, Content: "msg"})
	if len(s.History.Messages()) != 1 {
		t.Fatalf("expected one message before reset")
	}

	s.NewConversation()

	if len(s.History.Messages()) != 0 {
		t.Fatalf("expected history cleared after NewConversation")
	}
}

func TestSessionSaveLoadWithFileStore(t *testing.T) {
	ctx := context.Background()
	s, err := NewSession("user-file", 128, nil, nil)
	if err != nil {
		t.Fatalf("NewSession error = %v", err)
	}
	s.Metadata["m"] = "file-store"
	s.AddMessage(&message.Message{Role: message.RoleUser, Content: "persist me"})

	store := NewFileStore(t.TempDir())
	if err := s.Save(ctx, store); err != nil {
		t.Fatalf("Save error = %v", err)
	}

	restored := &Session{}
	if err := restored.LoadFromStorage(ctx, store, s.ID); err != nil {
		t.Fatalf("LoadFromStorage error = %v", err)
	}

	if restored.UserID != s.UserID {
		t.Fatalf("expected restored user id %s, got %s", s.UserID, restored.UserID)
	}
	if restored.Metadata["m"] != "file-store" {
		t.Fatalf("expected restored metadata")
	}
	if len(restored.History.Messages()) != 1 {
		t.Fatalf("expected restored history size 1")
	}
}

func TestSessionSaveLoadWithSQLiteStore(t *testing.T) {
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "session.db")
	store, err := NewSQLiteStore(dsn)
	if err != nil {
		t.Fatalf("NewSQLiteStore error = %v", err)
	}
	defer func() { _ = store.Close() }()

	s, err := NewSession("user-sqlite", 128, nil, nil)
	if err != nil {
		t.Fatalf("NewSession error = %v", err)
	}
	s.AddMessage(&message.Message{Role: message.RoleAssistant, Content: "db"})

	if err := s.Save(ctx, store); err != nil {
		t.Fatalf("Save error = %v", err)
	}

	restored := &Session{}
	if err := restored.LoadFromStorage(ctx, store, s.ID); err != nil {
		t.Fatalf("LoadFromStorage error = %v", err)
	}

	if restored.ID != s.ID {
		t.Fatalf("expected restored id %s, got %s", s.ID, restored.ID)
	}
	if len(restored.History.Messages()) != 1 {
		t.Fatalf("expected restored history size 1")
	}
}
