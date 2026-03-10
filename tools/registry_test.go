package tools

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

type TestInput struct {
	N int `json:"n"`
}

type TestOutput struct {
	Sum int `json:"sum"`
}

func TestRegistry_RegisterGetListCountSchemas_AndConversions(t *testing.T) {
	r := NewRegistry(DefaultMaxTools)

	// Two non-streaming tools, with names intentionally out of order.
	toolB := NewTool(func(ctx context.Context, in struct {
		X int `json:"x"`
	}) (struct {
		Y int `json:"y"`
	}, error) {
		return struct {
			Y int `json:"y"`
		}{Y: in.X + 1}, nil
	}, WithName("btool"), WithDescription("non-stream tool"))

	toolA := NewTool(func(ctx context.Context, in struct {
		Msg string `json:"msg"`
	}) (struct {
		Echo string `json:"echo"`
	}, error) {
		return struct {
			Echo string `json:"echo"`
		}{Echo: in.Msg}, nil
	}, WithName("atool"))

	if err := r.Register(toolB); err != nil {
		t.Fatalf("register toolB: %v", err)
	}
	if err := r.Register(toolA); err != nil {
		t.Fatalf("register toolA: %v", err)
	}
	if got := r.Count(); got != 2 {
		t.Fatalf("Count() = %d, want 2", got)
	}

	// List() should be sorted by name.
	items := r.List()
	if len(items) != 2 {
		t.Fatalf("List() len = %d, want 2", len(items))
	}
	if items[0].Name() != "atool" || items[1].Name() != "btool" {
		t.Fatalf("List() order = [%s, %s], want [atool, btool]", items[0].Name(), items[1].Name())
	}

	// Schemas() should align with List() ordering.
	schemas := r.Schemas()
	if len(schemas) != 2 {
		t.Fatalf("Schemas() len = %d, want 2", len(schemas))
	}
	if schemas[0].Input == nil || schemas[0].Input.Type != "object" {
		t.Fatalf("Schemas()[0].Input = %#v, want object schema", schemas[0].Input)
	}
	if schemas[1].Input == nil || schemas[1].Input.Type != "object" {
		t.Fatalf("Schemas()[1].Input = %#v, want object schema", schemas[1].Input)
	}

	// Get() trims spaces.
	got, ok := r.Get("  atool  ")
	if !ok || got == nil {
		t.Fatalf("Get(atool) ok=%v tool=%v, want ok=true", ok, got)
	}

	// Interface conversion: BaseTool -> Tool (non-streaming).
	nonStream, err := AsNonStreamingTool(got)
	if err != nil {
		t.Fatalf("AsNonStreamingTool: %v", err)
	}
	out, err := nonStream.Execute(context.Background(), []byte(`{"msg":"hi"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	outObj, ok := out.(struct {
		Echo string `json:"echo"`
	})
	if !ok {
		t.Fatalf("Execute output type = %T, want struct{Echo string}", out)
	}
	if outObj.Echo != "hi" {
		t.Fatalf("Execute output echo=%q, want %q", outObj.Echo, "hi")
	}
	if _, err := AsStreamingTool(got); err == nil {
		t.Fatalf("AsStreamingTool(non-stream) expected error, got nil")
	}

	// Register a streaming tool and validate conversion + streaming execution.
	streamer := NewStreamTool[TestInput, TestOutput](func(in TestInput) *StreamReader {
		s := NewStream(4)
		go func() {
			for i := 0; i < in.N; i++ {
				item := StreamChunk{
					Data: i,
				}
				s.Writer.Send(item, nil)
			}
			s.Writer.Close()
		}()
		return s.Reader
	}, WithName("streamer"))

	if err := r.Register(streamer); err != nil {
		t.Fatalf("register streamer: %v", err)
	}
	base, ok := r.Get("streamer")
	if !ok {
		t.Fatalf("Get(streamer) ok=false")
	}
	st, err := AsStreamingTool(base)
	if err != nil {
		t.Fatalf("AsStreamingTool: %v", err)
	}
	args, err := NewArgumentsFromStruct(TestInput{N: 3})
	if err != nil {
		t.Fatalf("NewArgumentsFromStruct: %v", err)
	}
	reader, err := st.StreamExecute(context.Background(), args)
	if err != nil {
		t.Fatalf("StreamExecute: %v", err)
	}
	defer reader.Close()

	for want := 0; want < 3; want++ {
		vAny, err := reader.Recv()
		if err != nil {
			t.Fatalf("Recv err=%v, want nil", err)
		}
		// v, ok := vAny.(int)
		// if !ok {
		// 	t.Fatalf("Recv type=%T, want int", vAny)
		// }
		v := vAny.Data
		if v != want {
			t.Fatalf("Recv=%d, want %d", v, want)
		}
	}
	_, err = reader.Recv()
	if !errors.Is(err, io.EOF) {
		t.Fatalf("final Recv err=%v, want io.EOF", err)
	}
	if _, err := AsNonStreamingTool(base); err == nil {
		t.Fatalf("AsNonStreamingTool(streaming) expected error, got nil")
	}
}

func TestRegistry_RegisterAndUnregister_Errors(t *testing.T) {
	r := NewRegistry(DefaultMaxTools)

	if err := r.Register(nil); !errors.Is(err, ErrNilTool) {
		t.Fatalf("Register(nil) err=%v, want ErrNilTool", err)
	}

	// Empty name (after TrimSpace) should be rejected.
	emptyNameTool := NewTool(func(ctx context.Context, in struct{}) (struct{}, error) {
		return struct{}{}, nil
	}, WithName("   "))
	if err := r.Register(emptyNameTool); !errors.Is(err, ErrEmptyToolName) {
		t.Fatalf("Register(empty name) err=%v, want ErrEmptyToolName", err)
	}

	dup1 := NewTool(func(ctx context.Context, in struct{}) (struct{}, error) {
		return struct{}{}, nil
	}, WithName("dup"))
	dup2 := NewTool(func(ctx context.Context, in struct{}) (struct{}, error) {
		return struct{}{}, nil
	}, WithName("dup"))

	if err := r.Register(dup1); err != nil {
		t.Fatalf("Register(dup1) err=%v, want nil", err)
	}
	if err := r.Register(dup2); err == nil {
		t.Fatalf("Register(dup2) expected error, got nil")
	} else {
		if !errors.Is(err, ErrToolAlreadyRegister) {
			t.Fatalf("Register(dup2) err=%v, want ErrToolAlreadyRegister", err)
		}
		if !strings.Contains(err.Error(), "dup") {
			t.Fatalf("Register(dup2) err=%q should contain tool name", err.Error())
		}
	}

	if ok := r.Unregister(""); ok {
		t.Fatalf("Unregister(\"\") = true, want false")
	}
	if ok := r.Unregister("not-exists"); ok {
		t.Fatalf("Unregister(not-exists) = true, want false")
	}
	if ok := r.Unregister("dup"); !ok {
		t.Fatalf("Unregister(dup) = false, want true")
	}
	if ok := r.Unregister("dup"); ok {
		t.Fatalf("Unregister(dup again) = true, want false")
	}

	if t0, ok := r.Get(""); ok || t0 != nil {
		t.Fatalf("Get(\"\") ok=%v tool=%v, want ok=false", ok, t0)
	}
}
