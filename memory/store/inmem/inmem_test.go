package inmem

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"xgc-agent/memory"
	"xgc-agent/tools"
)

func TestInMemoryStore_CRUDListAndSearch(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore(WithMemoryLimit(2))
	user := "user-1"

	if err := s.Add(ctx, "", "content", nil, nil); !errors.Is(err, memory.ErrUserIDRequired) {
		t.Fatalf("Add empty user err=%v, want ErrUserIDRequired", err)
	}

	if err := s.Add(ctx, user, "first", []string{"t1"}, map[string]any{"k": "v"}); err != nil {
		t.Fatalf("Add first: %v", err)
	}
	if err := s.Add(ctx, user, "second", []string{"t2"}, map[string]any{"n": 2}); err != nil {
		t.Fatalf("Add second: %v", err)
	}
	if err := s.Add(ctx, user, "third", nil, nil); err == nil || !strings.Contains(err.Error(), "memory limit exceeded") {
		t.Fatalf("Add third err=%v, want memory limit exceeded", err)
	}

	if _, err := s.Get(ctx, "", "id"); !errors.Is(err, memory.ErrUserIDRequired) {
		t.Fatalf("Get empty user err=%v, want ErrUserIDRequired", err)
	}
	if _, err := s.Get(ctx, user, ""); !errors.Is(err, memory.ErrMemoryIDRequired) {
		t.Fatalf("Get empty memoryID err=%v, want ErrMemoryIDRequired", err)
	}
	if _, err := s.Get(ctx, "missing", "id"); !errors.Is(err, memory.ErrNotFound) {
		t.Fatalf("Get missing user err=%v, want ErrNotFound", err)
	}

	all, err := s.List(ctx, user, 0)
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("List len=%d, want 2", len(all))
	}
	if all[0].MemoryID == "" || all[1].MemoryID == "" {
		t.Fatalf("List returned empty memory ID")
	}
	if all[0].MemoryID == all[1].MemoryID {
		t.Fatalf("List returned duplicate memory ID")
	}

	idToUpdate := all[0].MemoryID
	if err := s.Update(ctx, "", idToUpdate, "x", nil, nil); !errors.Is(err, memory.ErrUserIDRequired) {
		t.Fatalf("Update empty user err=%v, want ErrUserIDRequired", err)
	}
	if err := s.Update(ctx, user, "", "x", nil, nil); !errors.Is(err, memory.ErrMemoryIDRequired) {
		t.Fatalf("Update empty memoryID err=%v, want ErrMemoryIDRequired", err)
	}
	if err := s.Update(ctx, "missing", idToUpdate, "x", nil, nil); !errors.Is(err, memory.ErrNotFound) {
		t.Fatalf("Update missing user err=%v, want ErrNotFound", err)
	}
	if err := s.Update(ctx, user, "missing", "x", nil, nil); !errors.Is(err, memory.ErrNotFound) {
		t.Fatalf("Update missing memoryID err=%v, want ErrNotFound", err)
	}

	time.Sleep(5 * time.Millisecond)
	if err := s.Update(ctx, user, idToUpdate, "first-updated", []string{"t1", "t3"}, map[string]any{"k": "v2"}); err != nil {
		t.Fatalf("Update existing: %v", err)
	}

	got, err := s.Get(ctx, user, idToUpdate)
	if err != nil {
		t.Fatalf("Get updated: %v", err)
	}
	if got.Memory.Content != "first-updated" {
		t.Fatalf("Update content=%q, want %q", got.Memory.Content, "first-updated")
	}
	if len(got.Memory.Topics) != 2 || got.Memory.Topics[0] != "t1" || got.Memory.Topics[1] != "t3" {
		t.Fatalf("Update topics=%v, want [t1 t3]", got.Memory.Topics)
	}
	if got.Memory.Metadata["k"] != "v2" {
		t.Fatalf("Update metadata=%v, want k=v2", got.Memory.Metadata)
	}

	all, err = s.List(ctx, user, 0)
	if err != nil {
		t.Fatalf("List after update: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("List after update len=%d, want 2", len(all))
	}
	if all[0].MemoryID != idToUpdate {
		t.Fatalf("List order want updated memory first, got %s", all[0].MemoryID)
	}

	if err := s.Delete(ctx, "", idToUpdate); !errors.Is(err, memory.ErrUserIDRequired) {
		t.Fatalf("Delete empty user err=%v, want ErrUserIDRequired", err)
	}
	if err := s.Delete(ctx, user, ""); !errors.Is(err, memory.ErrMemoryIDRequired) {
		t.Fatalf("Delete empty memoryID err=%v, want ErrMemoryIDRequired", err)
	}
	if err := s.Delete(ctx, user, "missing"); !errors.Is(err, memory.ErrNotFound) {
		t.Fatalf("Delete missing memoryID err=%v, want ErrNotFound", err)
	}
	if err := s.Delete(ctx, user, idToUpdate); err != nil {
		t.Fatalf("Delete existing: %v", err)
	}
	if _, err := s.Get(ctx, user, idToUpdate); !errors.Is(err, memory.ErrNotFound) {
		t.Fatalf("Get deleted err=%v, want ErrNotFound", err)
	}

	if err := s.Clear(ctx, ""); !errors.Is(err, memory.ErrUserIDRequired) {
		t.Fatalf("Clear empty user err=%v, want ErrUserIDRequired", err)
	}
	if err := s.Clear(ctx, user); err != nil {
		t.Fatalf("Clear user: %v", err)
	}
	left, err := s.List(ctx, user, 0)
	if err != nil {
		t.Fatalf("List after clear: %v", err)
	}
	if len(left) != 0 {
		t.Fatalf("List after clear len=%d, want 0", len(left))
	}

	if _, err := s.List(ctx, "", 0); !errors.Is(err, memory.ErrInvalidID) {
		t.Fatalf("List empty user err=%v, want ErrInvalidID", err)
	}

	if _, err := s.Search(ctx, user, []float32{0.1, 0.2, 0.3}, 10); !errors.Is(err, memory.ErrSearchNotSupported) {
		t.Fatalf("Search err=%v, want ErrSearchNotSupported", err)
	}
}

func TestInMemoryStore_OptionsAndToolRegistry(t *testing.T) {
	ctx := context.Background()
	reg := tools.NewRegistry(0)
	adder := tools.NewTool(func(ctx context.Context, in struct {
		A int `json:"a"`
	}) (struct {
		B int `json:"b"`
	}, error) {
		return struct {
			B int `json:"b"`
		}{B: in.A + 1}, nil
	}, tools.WithName("adder"))

	if err := reg.Register(adder); err != nil {
		t.Fatalf("register tool: %v", err)
	}
	enabled := map[string]bool{"adder": true}
	s := NewInMemoryStore(
		WithMemoryLimit(7),
		WithToolRegistry(reg),
		WithEnabledTools(enabled),
	)

	if s.options.memoryLimit != 7 {
		t.Fatalf("memoryLimit=%d, want 7", s.options.memoryLimit)
	}
	if s.options.toolRegistry != reg {
		t.Fatalf("toolRegistry not set")
	}
	if !s.options.enableTools["adder"] {
		t.Fatalf("enableTools missing adder")
	}

	base, ok := reg.Get("adder")
	if !ok {
		t.Fatalf("Get(adder) ok=false")
	}
	nonStream, err := tools.AsNonStreamingTool(base)
	if err != nil {
		t.Fatalf("AsNonStreamingTool: %v", err)
	}
	out, err := nonStream.Execute(ctx, []byte(`{"a":2}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	outObj, ok := out.(struct {
		B int `json:"b"`
	})
	if !ok {
		t.Fatalf("Execute output type=%T, want struct{B int}", out)
	}
	if outObj.B != 3 {
		t.Fatalf("Execute output b=%d, want 3", outObj.B)
	}
}
