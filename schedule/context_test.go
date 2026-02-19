package schedule

import (
	"testing"
	"time"
)

func TestGraphContext_GetSet(t *testing.T) {
	ctx := &graphContext{}

	if v, ok := ctx.Get("missing"); ok || v != nil {
		t.Fatalf("expected missing key to return (nil,false), got (%v,%v)", v, ok)
	}

	ctx.Set("k", "v1")
	if v, ok := ctx.Get("k"); !ok || v != "v1" {
		t.Fatalf("expected key k to be v1, got (%v,%v)", v, ok)
	}

	ctx.Set("k", "v2")
	if v, ok := ctx.Get("k"); !ok || v != "v2" {
		t.Fatalf("expected key k to be overwritten to v2, got (%v,%v)", v, ok)
	}
}

func TestGraphContext_SubscribeWithoutRegister(t *testing.T) {
	ctx := &graphContext{}

	if ch := ctx.Subscribe("unknown"); ch != nil {
		t.Fatalf("expected nil subscription channel for unknown key")
	}
}

func TestGraphContext_RegisterSubscribe_HistoryAndRealtime(t *testing.T) {
	ctx := &graphContext{}
	producer := make(chan any, 8)
	ctx.Register("stream", producer)

	producer <- "m1"
	producer <- "m2"

	ch := ctx.Subscribe("stream")
	if ch == nil {
		t.Fatalf("expected non-nil subscription channel")
	}

	assertRecvEqual(t, ch, "m1")
	assertRecvEqual(t, ch, "m2")

	producer <- "m3"
	assertRecvEqual(t, ch, "m3")

	close(producer)
	assertClosed(t, ch)
}

func TestGraphContext_Register_DuplicateKeyIgnored(t *testing.T) {
	ctx := &graphContext{}
	producer1 := make(chan any, 2)
	producer2 := make(chan any, 2)

	ctx.Register("dup", producer1)
	ctx.Register("dup", producer2)

	ch := ctx.Subscribe("dup")
	if ch == nil {
		t.Fatalf("expected non-nil subscription channel")
	}

	producer1 <- "from-first"
	assertRecvEqual(t, ch, "from-first")

	producer2 <- "from-second"
	assertNoMessage(t, ch)

	close(producer1)
	assertClosed(t, ch)
}

func TestGraphContext_Register_BroadcastToMultipleSubscribersAndClose(t *testing.T) {
	ctx := &graphContext{}
	producer := make(chan any, 4)
	ctx.Register("fanout", producer)

	ch1 := ctx.Subscribe("fanout")
	ch2 := ctx.Subscribe("fanout")
	if ch1 == nil || ch2 == nil {
		t.Fatalf("expected non-nil subscription channels")
	}

	producer <- "payload"
	assertRecvEqual(t, ch1, "payload")
	assertRecvEqual(t, ch2, "payload")

	close(producer)
	assertClosed(t, ch1)
	assertClosed(t, ch2)
}

func assertRecvEqual(t *testing.T, ch <-chan any, expected any) {
	t.Helper()
	select {
	case v, ok := <-ch:
		if !ok {
			t.Fatalf("expected value %v, got closed channel", expected)
		}
		if v != expected {
			t.Fatalf("expected %v, got %v", expected, v)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out waiting for value %v", expected)
	}
}

func assertNoMessage(t *testing.T, ch <-chan any) {
	t.Helper()
	select {
	case v, ok := <-ch:
		if !ok {
			t.Fatalf("expected channel to stay open, got closed")
		}
		t.Fatalf("expected no message, got %v", v)
	case <-time.After(120 * time.Millisecond):
	}
}

func assertClosed(t *testing.T, ch <-chan any) {
	t.Helper()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatalf("expected channel to be closed")
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out waiting for channel to close")
	}
}
