package schedule

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestScheduler_Run(t *testing.T) {
	start := time.Now()
	defer func() { fmt.Println("time cost : ", time.Since(start).String()) }()
	sb := NewScheduler()
	var mu sync.Mutex
	executed := map[string]bool{}

	a, err := sb.AddNode("a", NodeTypeNormal, &DemoNode{Name: "a", Executed: executed, Mu: &mu})
	if err != nil {
		t.Fatalf("add node a failed: %v", err)
	}
	b, err := sb.AddNode("b", NodeTypeNormal, &DemoNode{Name: "b", Executed: executed, Mu: &mu})
	if err != nil {
		t.Fatalf("add node b failed: %v", err)
	}
	c, err := sb.AddNode("c", NodeTypeNormal, &DemoNode{Name: "c", Executed: executed, Mu: &mu})
	if err != nil {
		t.Fatalf("add node c failed: %v", err)
	}
	d, err := sb.AddNode("d", NodeTypeNormal, &DemoNode{Name: "d", Executed: executed, Mu: &mu})
	if err != nil {
		t.Fatalf("add node d failed: %v", err)
	}
	e, err := sb.AddNode("e", NodeTypeNormal, &DemoNode{Name: "e", Executed: executed, Mu: &mu})
	if err != nil {
		t.Fatalf("add node e failed: %v", err)
	}

	sb.Graph[a.id] = a
	sb.Graph[b.id] = b
	sb.Graph[c.id] = c
	sb.Graph[d.id] = d
	sb.Graph[e.id] = e

	sb.SetEntry(a)
	sb.SetExit(e)

	if err = sb.AddEdge(a, b); err != nil {
		t.Fatalf("add edge a->b failed: %v", err)
	}
	if err = sb.AddEdge(a, c); err != nil {
		t.Fatalf("add edge a->c failed: %v", err)
	}
	if err = sb.AddEdge(b, d); err != nil {
		t.Fatalf("add edge b->d failed: %v", err)
	}
	if err = sb.AddEdge(c, e); err != nil {
		t.Fatalf("add edge c->e failed: %v", err)
	}
	if err = sb.AddEdge(d, e); err != nil {
		t.Fatalf("add edge d->e failed: %v", err)
	}

	if _, err := sb.Run(context.Background(), nil); err != nil {
		t.Fatalf("scheduler run failed: %v", err)
	}

	for _, name := range []string{"a", "b", "c", "d", "e"} {
		if !executed[name] {
			t.Fatalf("node %s not executed", name)
		}
	}
}

func TestScheduler_ConditionalGraph(t *testing.T) {
	start := time.Now()
	defer func() { fmt.Println("time cost : ", time.Since(start).String()) }()
	sb := NewScheduler()
	var mu sync.Mutex
	executed := map[string]bool{}

	a, err := sb.AddNode("a", NodeTypeNormal, &DemoNode{Name: "a", Executed: executed, Mu: &mu})
	if err != nil {
		t.Fatalf("add node a failed: %v", err)
	}
	cond, err := sb.AddConditionalNode("cond", func(ctx context.Context, state *GraphState) ([]string, error) {
		return []string{"right"}, nil
	})
	if err != nil {
		t.Fatalf("add conditional node failed: %v", err)
	}
	left, err := sb.AddNode("leftNode", NodeTypeNormal, &DemoNode{Name: "leftNode", Executed: executed, Mu: &mu})
	if err != nil {
		t.Fatalf("add left node failed: %v", err)
	}
	right, err := sb.AddNode("rightNode", NodeTypeNormal, &DemoNode{Name: "rightNode", Executed: executed, Mu: &mu})
	if err != nil {
		t.Fatalf("add right node failed: %v", err)
	}
	end, err := sb.AddNode("end", NodeTypeNormal, &DemoNode{Name: "end", Executed: executed, Mu: &mu})
	if err != nil {
		t.Fatalf("add end node failed: %v", err)
	}

	sb.SetEntry(a)
	sb.SetExit(end)
	if err = sb.AddEdge(a, cond); err != nil {
		t.Fatalf("add edge a->cond failed: %v", err)
	}
	if err = sb.AddConditionalEdge(cond, left, "left"); err != nil {
		t.Fatalf("add conditional edge cond->left failed: %v", err)
	}
	if err = sb.AddConditionalEdge(cond, right, "right"); err != nil {
		t.Fatalf("add conditional edge cond->right failed: %v", err)
	}
	if err = sb.AddEdge(left, end); err != nil {
		t.Fatalf("add edge left->end failed: %v", err)
	}
	if err = sb.AddEdge(right, end); err != nil {
		t.Fatalf("add edge right->end failed: %v", err)
	}

	if len(cond.Next) != 2 {
		t.Fatalf("expected 2 conditional branches, got %d", len(cond.Next))
	}

	foundLeft := false
	foundRight := false
	for _, virtual := range cond.Next {
		if virtual.Type != _NodeTypeConditionalVirtual {
			t.Fatalf("expected virtual node type, got %d", virtual.Type)
		}
		if len(virtual.Next) != 1 {
			t.Fatalf("expected virtual node to have one next node, got %d", len(virtual.Next))
		}
		switch virtual.Name {
		case "left":
			if virtual.Next[0] != left {
				t.Fatalf("left tag should point to leftNode")
			}
			foundLeft = true
		case "right":
			if virtual.Next[0] != right {
				t.Fatalf("right tag should point to rightNode")
			}
			foundRight = true
		default:
			t.Fatalf("unexpected conditional tag: %s", virtual.Name)
		}
	}

	if !foundLeft || !foundRight {
		t.Fatalf("expected left and right conditional branches, got left=%v right=%v", foundLeft, foundRight)
	}

	if _, err := sb.Run(context.Background(), nil); err != nil {
		t.Fatalf("scheduler run failed: %v", err)
	}
}

func TestScheduler_FullGraphDataFlow(t *testing.T) {
	sb := NewScheduler()

	ingest, err := sb.AddNode("ingest", NodeTypeNormal, &FuncNode{Fn: func(ctx context.Context, state *GraphState) error {
		input, ok := state.Context.Get("input")
		if !ok {
			return errors.New("input not found")
		}
		text, ok := input.(string)
		if !ok {
			return errors.New("input must be string")
		}
		state.Context.Set("raw", strings.TrimSpace(text))
		return nil
	}})
	if err != nil {
		t.Fatalf("add ingest node failed: %v", err)
	}

	enrich, err := sb.AddNode("enrich", NodeTypeNormal, &FuncNode{Fn: func(ctx context.Context, state *GraphState) error {
		rawAny, ok := state.Context.Get("raw")
		if !ok {
			return errors.New("raw not found")
		}
		raw, ok := rawAny.(string)
		if !ok {
			return errors.New("raw must be string")
		}
		normalized := strings.ToUpper(raw)
		state.Context.Set("normalized", normalized)
		state.Context.Set("score", len(normalized))
		return nil
	}})
	if err != nil {
		t.Fatalf("add enrich node failed: %v", err)
	}

	audit, err := sb.AddNode("audit", NodeTypeNormal, &FuncNode{Fn: func(ctx context.Context, state *GraphState) error {
		rawAny, ok := state.Context.Get("raw")
		if !ok {
			return errors.New("raw not found")
		}
		raw, ok := rawAny.(string)
		if !ok {
			return errors.New("raw must be string")
		}
		state.Context.Set("audit", fmt.Sprintf("audit:%s", raw))
		return nil
	}})
	if err != nil {
		t.Fatalf("add audit node failed: %v", err)
	}

	cond, err := sb.AddConditionalNode("route", func(ctx context.Context, state *GraphState) ([]string, error) {
		scoreAny, ok := state.Context.Get("score")
		if !ok {
			return nil, errors.New("score not found")
		}
		score, ok := scoreAny.(int)
		if !ok {
			return nil, errors.New("score must be int")
		}
		if score >= 5 {
			return []string{"high"}, nil
		}
		return []string{"low"}, nil
	})
	if err != nil {
		t.Fatalf("add conditional node failed: %v", err)
	}

	high, err := sb.AddNode("highNode", NodeTypeNormal, &FuncNode{Fn: func(ctx context.Context, state *GraphState) error {
		scoreAny, ok := state.Context.Get("score")
		if !ok {
			return errors.New("score not found")
		}
		score, ok := scoreAny.(int)
		if !ok {
			return errors.New("score must be int")
		}
		state.Context.Set("route_result", "high")
		state.Context.Set("result", score*2)
		return nil
	}})
	if err != nil {
		t.Fatalf("add high node failed: %v", err)
	}

	low, err := sb.AddNode("lowNode", NodeTypeNormal, &FuncNode{Fn: func(ctx context.Context, state *GraphState) error {
		scoreAny, ok := state.Context.Get("score")
		if !ok {
			return errors.New("score not found")
		}
		score, ok := scoreAny.(int)
		if !ok {
			return errors.New("score must be int")
		}
		state.Context.Set("route_result", "low")
		state.Context.Set("result", score+1)
		return nil
	}})
	if err != nil {
		t.Fatalf("add low node failed: %v", err)
	}

	merge, err := sb.AddNode("merge", NodeTypeNormal, &FuncNode{Fn: func(ctx context.Context, state *GraphState) error {
		normalizedAny, ok := state.Context.Get("normalized")
		if !ok {
			return errors.New("normalized not found")
		}
		normalized, ok := normalizedAny.(string)
		if !ok {
			return errors.New("normalized must be string")
		}

		auditAny, ok := state.Context.Get("audit")
		if !ok {
			return errors.New("audit not found")
		}
		audit, ok := auditAny.(string)
		if !ok {
			return errors.New("audit must be string")
		}

		routeAny, ok := state.Context.Get("route_result")
		if !ok {
			return errors.New("route_result not found")
		}
		route, ok := routeAny.(string)
		if !ok {
			return errors.New("route_result must be string")
		}

		resultAny, ok := state.Context.Get("result")
		if !ok {
			return errors.New("result not found")
		}
		result, ok := resultAny.(int)
		if !ok {
			return errors.New("result must be int")
		}

		state.Context.Set("final", fmt.Sprintf("%s|%s|%s|%d", normalized, audit, route, result))
		return nil
	}})
	if err != nil {
		t.Fatalf("add merge node failed: %v", err)
	}

	sb.SetEntry(ingest)
	sb.SetExit(merge)

	if err = sb.AddEdge(ingest, enrich); err != nil {
		t.Fatalf("add edge ingest->enrich failed: %v", err)
	}
	if err = sb.AddEdge(ingest, audit); err != nil {
		t.Fatalf("add edge ingest->audit failed: %v", err)
	}
	if err = sb.AddEdge(enrich, cond); err != nil {
		t.Fatalf("add edge enrich->cond failed: %v", err)
	}
	if err = sb.AddConditionalEdge(cond, high, "high"); err != nil {
		t.Fatalf("add conditional edge cond->high failed: %v", err)
	}
	if err = sb.AddConditionalEdge(cond, low, "low"); err != nil {
		t.Fatalf("add conditional edge cond->low failed: %v", err)
	}
	if err = sb.AddEdge(high, merge); err != nil {
		t.Fatalf("add edge high->merge failed: %v", err)
	}
	if err = sb.AddEdge(low, merge); err != nil {
		t.Fatalf("add edge low->merge failed: %v", err)
	}
	if err = sb.AddEdge(audit, merge); err != nil {
		t.Fatalf("add edge audit->merge failed: %v", err)
	}

	if _, err = sb.Run(context.Background(), map[string]any{"input": " hello "}); err != nil {
		t.Fatalf("scheduler run failed: %v", err)
	}

	if ingest.State != NodeFinished {
		t.Fatalf("expected ingest to be finished, got %d", ingest.State)
	}
	if enrich.State != NodeFinished {
		t.Fatalf("expected enrich to be finished, got %d", enrich.State)
	}
	if audit.State != NodeFinished {
		t.Fatalf("expected audit to be finished, got %d", audit.State)
	}
	if cond.State != NodeFinished {
		t.Fatalf("expected route condition node to be finished, got %d", cond.State)
	}
	if high.State != NodeFinished {
		t.Fatalf("expected high node to be finished, got %d", high.State)
	}
	if low.State != NodeSkipped {
		t.Fatalf("expected low node to be skipped, got %d", low.State)
	}
	if merge.State != NodeFinished {
		t.Fatalf("expected merge node to be finished, got %d", merge.State)
	}

	finalAny, ok := sb.GraphState.Context.Get("final")
	if !ok {
		t.Fatalf("final result not found in graph context")
	}
	final, ok := finalAny.(string)
	if !ok {
		t.Fatalf("final must be string, got %T", finalAny)
	}
	expected := "HELLO|audit:hello|high|10"
	if final != expected {
		t.Fatalf("expected final result %q, got %q", expected, final)
	}
}

func TestScheduler_FullGraph_RegisterSubscribeFlow(t *testing.T) {
	sb := NewScheduler()

	producer, err := sb.AddNode("producer", NodeTypeNormal, &FuncNode{Fn: func(ctx context.Context, state *GraphState) error {
		stream := make(chan any, 8)
		ready := make(chan struct{})

		state.Context.Register("stream", stream)
		state.Context.Set("stream_producer", stream)
		state.Context.Set("stream_ready", ready)

		stream <- "h1"
		stream <- "h2"
		return nil
	}})
	if err != nil {
		t.Fatalf("add producer node failed: %v", err)
	}

	subscriber, err := sb.AddNode("subscriber", NodeTypeNormal, &FuncNode{Fn: func(ctx context.Context, state *GraphState) error {
		readyAny, ok := state.Context.Get("stream_ready")
		if !ok {
			return errors.New("stream_ready not found")
		}
		ready, ok := readyAny.(chan struct{})
		if !ok {
			return errors.New("stream_ready type invalid")
		}

		sub := state.Context.Subscribe("stream")
		if sub == nil {
			return errors.New("subscribe stream returned nil")
		}

		collected := make([]string, 0, 3)
		for i := 0; i < 2; i++ {
			select {
			case msg, ok := <-sub:
				if !ok {
					return errors.New("stream closed before historical messages consumed")
				}
				s, ok := msg.(string)
				if !ok {
					return fmt.Errorf("historical message type invalid: %T", msg)
				}
				collected = append(collected, s)
			case <-time.After(1 * time.Second):
				return errors.New("timeout reading historical messages")
			}
		}

		close(ready)

		for len(collected) < 3 {
			select {
			case msg, ok := <-sub:
				if !ok {
					return errors.New("stream closed before receiving expected messages")
				}
				s, ok := msg.(string)
				if !ok {
					return fmt.Errorf("stream message type invalid: %T", msg)
				}
				collected = append(collected, s)
			case <-time.After(1 * time.Second):
				return errors.New("timeout reading stream messages")
			}
		}

		closeWait := time.After(1 * time.Second)
		for {
			select {
			case msg, ok := <-sub:
				if !ok {
					state.Context.Set("stream_collected", collected)
					return nil
				}
				s, ok := msg.(string)
				if !ok {
					return fmt.Errorf("stream message type invalid while waiting close: %T", msg)
				}
				collected = append(collected, s)
			case <-closeWait:
				return errors.New("timeout waiting subscription channel close")
			}
		}
	}})
	if err != nil {
		t.Fatalf("add subscriber node failed: %v", err)
	}

	emitRealtime, err := sb.AddNode("emitRealtime", NodeTypeNormal, &FuncNode{Fn: func(ctx context.Context, state *GraphState) error {
		readyAny, ok := state.Context.Get("stream_ready")
		if !ok {
			return errors.New("stream_ready not found")
		}
		ready, ok := readyAny.(chan struct{})
		if !ok {
			return errors.New("stream_ready type invalid")
		}

		producerAny, ok := state.Context.Get("stream_producer")
		if !ok {
			return errors.New("stream_producer not found")
		}
		stream, ok := producerAny.(chan any)
		if !ok {
			return errors.New("stream_producer type invalid")
		}

		select {
		case <-ready:
		case <-time.After(1 * time.Second):
			return errors.New("timeout waiting subscriber ready")
		}

		stream <- "r1"
		close(stream)
		return nil
	}})
	if err != nil {
		t.Fatalf("add emitRealtime node failed: %v", err)
	}

	verify, err := sb.AddNode("verify", NodeTypeNormal, &FuncNode{Fn: func(ctx context.Context, state *GraphState) error {
		collectedAny, ok := state.Context.Get("stream_collected")
		if !ok {
			return errors.New("stream_collected not found")
		}
		collected, ok := collectedAny.([]string)
		if !ok {
			return fmt.Errorf("stream_collected type invalid: %T", collectedAny)
		}

		if len(collected) < 3 {
			return fmt.Errorf("expected at least 3 messages, got %v", collected)
		}

		if collected[len(collected)-1] != "r1" {
			return fmt.Errorf("expected last message to be realtime r1, got %v", collected)
		}

		h1Count := 0
		h2Count := 0
		r1Count := 0
		for _, msg := range collected {
			switch msg {
			case "h1":
				h1Count++
			case "h2":
				h2Count++
			case "r1":
				r1Count++
			}
		}

		if h1Count == 0 || h2Count == 0 {
			return fmt.Errorf("expected stream to contain historical messages h1 and h2, got %v", collected)
		}
		if r1Count != 1 {
			return fmt.Errorf("expected exactly one realtime message r1, got %d in %v", r1Count, collected)
		}

		state.Context.Set("stream_verified", true)
		return nil
	}})
	if err != nil {
		t.Fatalf("add verify node failed: %v", err)
	}

	sb.SetEntry(producer)
	sb.SetExit(verify)

	if err = sb.AddEdge(producer, subscriber); err != nil {
		t.Fatalf("add edge producer->subscriber failed: %v", err)
	}
	if err = sb.AddEdge(producer, emitRealtime); err != nil {
		t.Fatalf("add edge producer->emitRealtime failed: %v", err)
	}
	if err = sb.AddEdge(subscriber, verify); err != nil {
		t.Fatalf("add edge subscriber->verify failed: %v", err)
	}
	if err = sb.AddEdge(emitRealtime, verify); err != nil {
		t.Fatalf("add edge emitRealtime->verify failed: %v", err)
	}

	if _, err = sb.Run(context.Background(), nil); err != nil {
		t.Fatalf("scheduler run failed: %v", err)
	}

	if producer.State != NodeFinished {
		t.Fatalf("expected producer to be finished, got %d", producer.State)
	}
	if subscriber.State != NodeFinished {
		t.Fatalf("expected subscriber to be finished, got %d", subscriber.State)
	}
	if emitRealtime.State != NodeFinished {
		t.Fatalf("expected emitRealtime to be finished, got %d", emitRealtime.State)
	}
	if verify.State != NodeFinished {
		t.Fatalf("expected verify to be finished, got %d", verify.State)
	}

	verifiedAny, ok := sb.GraphState.Context.Get("stream_verified")
	if !ok {
		t.Fatalf("stream_verified not found")
	}
	verified, ok := verifiedAny.(bool)
	if !ok || !verified {
		t.Fatalf("expected stream_verified=true, got %v (%T)", verifiedAny, verifiedAny)
	}
}

func TestScheduler_LLMStream_ProducerReturnsThenAsyncEmits(t *testing.T) {
	sb := NewScheduler()

	llmNode, err := sb.AddNode("llm", NodeTypeNormal, &FuncNode{Fn: func(ctx context.Context, state *GraphState) error {
		stream := make(chan any, 8)
		state.Context.Register("llm_stream", stream)
		state.Context.Set("llm_returned", true)
		state.Context.Set("llm_returned_at", time.Now())

		go func() {
			time.Sleep(30 * time.Millisecond)
			stream <- "Hel"
			time.Sleep(20 * time.Millisecond)
			stream <- "lo"
			time.Sleep(20 * time.Millisecond)
			stream <- "!"
			close(stream)
		}()

		return nil
	}})
	if err != nil {
		t.Fatalf("add llm node failed: %v", err)
	}

	consumerNode, err := sb.AddNode("consumer", NodeTypeNormal, &FuncNode{Fn: func(ctx context.Context, state *GraphState) error {
		returnedAny, ok := state.Context.Get("llm_returned")
		if !ok {
			return errors.New("llm_returned not found")
		}
		returned, ok := returnedAny.(bool)
		if !ok || !returned {
			return fmt.Errorf("expected llm_returned=true, got %v (%T)", returnedAny, returnedAny)
		}

		sub := state.Context.Subscribe("llm_stream")
		if sub == nil {
			return errors.New("subscribe llm_stream returned nil")
		}

		tokens := make([]string, 0, 3)
		firstTokenAt := time.Time{}
		for msg := range sub {
			token, ok := msg.(string)
			if !ok {
				return fmt.Errorf("token type invalid: %T", msg)
			}
			if firstTokenAt.IsZero() {
				firstTokenAt = time.Now()
			}
			tokens = append(tokens, token)
		}

		if len(tokens) == 0 {
			return errors.New("no token received from llm_stream")
		}

		state.Context.Set("llm_tokens", tokens)
		state.Context.Set("llm_output", strings.Join(tokens, ""))
		state.Context.Set("llm_first_token_at", firstTokenAt)
		return nil
	}})
	if err != nil {
		t.Fatalf("add consumer node failed: %v", err)
	}

	verifyNode, err := sb.AddNode("verify", NodeTypeNormal, &FuncNode{Fn: func(ctx context.Context, state *GraphState) error {
		outputAny, ok := state.Context.Get("llm_output")
		if !ok {
			return errors.New("llm_output not found")
		}
		output, ok := outputAny.(string)
		if !ok {
			return fmt.Errorf("llm_output type invalid: %T", outputAny)
		}
		if output != "Hello!" {
			return fmt.Errorf("expected llm output Hello!, got %q", output)
		}

		returnedAtAny, ok := state.Context.Get("llm_returned_at")
		if !ok {
			return errors.New("llm_returned_at not found")
		}
		returnedAt, ok := returnedAtAny.(time.Time)
		if !ok {
			return fmt.Errorf("llm_returned_at type invalid: %T", returnedAtAny)
		}

		firstTokenAtAny, ok := state.Context.Get("llm_first_token_at")
		if !ok {
			return errors.New("llm_first_token_at not found")
		}
		firstTokenAt, ok := firstTokenAtAny.(time.Time)
		if !ok {
			return fmt.Errorf("llm_first_token_at type invalid: %T", firstTokenAtAny)
		}
		if firstTokenAt.Before(returnedAt) {
			return fmt.Errorf("expected first token after llm node returned, returnedAt=%v firstTokenAt=%v", returnedAt, firstTokenAt)
		}

		state.Context.Set("llm_stream_verified", true)
		return nil
	}})
	if err != nil {
		t.Fatalf("add verify node failed: %v", err)
	}

	sb.SetEntry(llmNode)
	sb.SetExit(verifyNode)

	if err = sb.AddEdge(llmNode, consumerNode); err != nil {
		t.Fatalf("add edge llm->consumer failed: %v", err)
	}
	if err = sb.AddEdge(consumerNode, verifyNode); err != nil {
		t.Fatalf("add edge consumer->verify failed: %v", err)
	}

	if _, err = sb.Run(context.Background(), nil); err != nil {
		t.Fatalf("scheduler run failed: %v", err)
	}

	if llmNode.State != NodeFinished {
		t.Fatalf("expected llm node finished, got %d", llmNode.State)
	}
	if consumerNode.State != NodeFinished {
		t.Fatalf("expected consumer node finished, got %d", consumerNode.State)
	}
	if verifyNode.State != NodeFinished {
		t.Fatalf("expected verify node finished, got %d", verifyNode.State)
	}

	verifiedAny, ok := sb.GraphState.Context.Get("llm_stream_verified")
	if !ok {
		t.Fatalf("llm_stream_verified not found")
	}
	verified, ok := verifiedAny.(bool)
	if !ok || !verified {
		t.Fatalf("expected llm_stream_verified=true, got %v (%T)", verifiedAny, verifiedAny)
	}
}

func TestScheduler_SubGraphNode_ReActLoop(t *testing.T) {
	parent := NewScheduler()

	initNode, err := parent.AddNode("init", NodeTypeNormal, &FuncNode{Fn: func(ctx context.Context, state *GraphState) error {
		fmt.Println("[init] initialize react state")
		state.Context.Set("question", "2 + 3 = ?")
		state.Context.Set("iteration", 0)
		state.Context.Set("max_steps", 4)
		state.Context.Set("done", false)
		state.Context.Set("final_answer", "")
		state.Context.Set("trace", []string{})
		return nil
	}})
	if err != nil {
		t.Fatalf("add init node failed: %v", err)
	}

	sub := NewScheduler()

	thinkNode, err := sub.AddNode("think", NodeTypeNormal, &FuncNode{Fn: func(ctx context.Context, state *GraphState) error {
		iterationAny, ok := state.Context.Get("iteration")
		if !ok {
			return errors.New("iteration not found")
		}
		iteration, ok := iterationAny.(int)
		if !ok {
			return fmt.Errorf("iteration type invalid: %T", iterationAny)
		}

		traceAny, ok := state.Context.Get("trace")
		if !ok {
			return errors.New("trace not found")
		}
		trace, ok := traceAny.([]string)
		if !ok {
			return fmt.Errorf("trace type invalid: %T", traceAny)
		}

		fmt.Printf("[subgraph/think] iteration=%d\n", iteration)
		trace = append(trace, fmt.Sprintf("think#%d", iteration+1))
		state.Context.Set("trace", trace)
		return nil
	}})
	if err != nil {
		t.Fatalf("add think node failed: %v", err)
	}

	actNode, err := sub.AddNode("act", NodeTypeNormal, &FuncNode{Fn: func(ctx context.Context, state *GraphState) error {
		iterationAny, ok := state.Context.Get("iteration")
		if !ok {
			return errors.New("iteration not found")
		}
		iteration, ok := iterationAny.(int)
		if !ok {
			return fmt.Errorf("iteration type invalid: %T", iterationAny)
		}

		traceAny, ok := state.Context.Get("trace")
		if !ok {
			return errors.New("trace not found")
		}
		trace, ok := traceAny.([]string)
		if !ok {
			return fmt.Errorf("trace type invalid: %T", traceAny)
		}

		fmt.Printf("[subgraph/act] iteration=%d\n", iteration)
		trace = append(trace, fmt.Sprintf("act#%d", iteration+1))
		state.Context.Set("trace", trace)

		if iteration == 0 {
			state.Context.Set("observation", "need exact arithmetic")
		} else {
			state.Context.Set("observation", "answer=5")
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("add act node failed: %v", err)
	}

	observeNode, err := sub.AddNode("observe", NodeTypeNormal, &FuncNode{Fn: func(ctx context.Context, state *GraphState) error {
		iterationAny, ok := state.Context.Get("iteration")
		if !ok {
			return errors.New("iteration not found")
		}
		iteration, ok := iterationAny.(int)
		if !ok {
			return fmt.Errorf("iteration type invalid: %T", iterationAny)
		}

		observationAny, ok := state.Context.Get("observation")
		if !ok {
			return errors.New("observation not found")
		}
		observation, ok := observationAny.(string)
		if !ok {
			return fmt.Errorf("observation type invalid: %T", observationAny)
		}

		traceAny, ok := state.Context.Get("trace")
		if !ok {
			return errors.New("trace not found")
		}
		trace, ok := traceAny.([]string)
		if !ok {
			return fmt.Errorf("trace type invalid: %T", traceAny)
		}

		fmt.Printf("[subgraph/observe] iteration=%d observation=%s\n", iteration, observation)
		trace = append(trace, fmt.Sprintf("observe#%d:%s", iteration+1, observation))
		state.Context.Set("trace", trace)

		nextIteration := iteration + 1
		state.Context.Set("iteration", nextIteration)
		if strings.Contains(observation, "answer=5") {
			state.Context.Set("done", true)
			state.Context.Set("final_answer", "5")
		} else {
			state.Context.Set("done", false)
			state.Context.Set("final_answer", "")
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("add observe node failed: %v", err)
	}

	sub.SetEntry(thinkNode)
	sub.SetExit(observeNode)
	if err = sub.AddEdge(thinkNode, actNode); err != nil {
		t.Fatalf("add edge think->act failed: %v", err)
	}
	if err = sub.AddEdge(actNode, observeNode); err != nil {
		t.Fatalf("add edge act->observe failed: %v", err)
	}

	reactNode, err := parent.AddSubGraphNode(
		"react_loop",
		sub,
		map[string]string{
			"question":  "question",
			"iteration": "iteration",
			"trace":     "trace",
		},
		func(ctx context.Context, state *GraphState) bool {
			doneAny, _ := state.Context.Get("done")
			done, _ := doneAny.(bool)
			iterationAny, _ := state.Context.Get("iteration")
			iteration, _ := iterationAny.(int)
			maxStepsAny, _ := state.Context.Get("max_steps")
			maxSteps, _ := maxStepsAny.(int)

			keep := !done && iteration < maxSteps
			fmt.Printf("[parent/react_loop cond] done=%v iteration=%d max=%d keep=%v\n", done, iteration, maxSteps, keep)
			return keep
		},
	)
	if err != nil {
		t.Fatalf("add react subgraph node failed: %v", err)
	}

	reactImpl, ok := reactNode.Fn.(*subGraphNode)
	if !ok {
		t.Fatalf("react node fn type invalid: %T", reactNode.Fn)
	}
	reactImpl.outputKeys = map[string]string{
		"iteration":    "iteration",
		"trace":        "trace",
		"done":         "done",
		"final_answer": "final_answer",
	}

	verifyNode, err := parent.AddNode("verify", NodeTypeNormal, &FuncNode{Fn: func(ctx context.Context, state *GraphState) error {
		fmt.Println("[verify] validating react final state")

		doneAny, ok := state.Context.Get("done")
		if !ok {
			return errors.New("done not found")
		}
		done, ok := doneAny.(bool)
		if !ok || !done {
			return fmt.Errorf("expected done=true, got %v (%T)", doneAny, doneAny)
		}

		answerAny, ok := state.Context.Get("final_answer")
		if !ok {
			return errors.New("final_answer not found")
		}
		answer, ok := answerAny.(string)
		if !ok {
			return fmt.Errorf("final_answer type invalid: %T", answerAny)
		}
		if answer != "5" {
			return fmt.Errorf("expected final_answer=5, got %q", answer)
		}

		iterationAny, ok := state.Context.Get("iteration")
		if !ok {
			return errors.New("iteration not found")
		}
		iteration, ok := iterationAny.(int)
		if !ok {
			return fmt.Errorf("iteration type invalid: %T", iterationAny)
		}
		if iteration != 2 {
			return fmt.Errorf("expected iteration=2, got %d", iteration)
		}

		traceAny, ok := state.Context.Get("trace")
		if !ok {
			return errors.New("trace not found")
		}
		trace, ok := traceAny.([]string)
		if !ok {
			return fmt.Errorf("trace type invalid: %T", traceAny)
		}
		if len(trace) < 6 {
			return fmt.Errorf("expected trace to include >=6 steps, got %v", trace)
		}
		last := trace[len(trace)-1]
		if !strings.Contains(last, "answer=5") {
			return fmt.Errorf("expected last trace to contain answer=5, got %q", last)
		}

		fmt.Printf("[verify] trace=%v\n", trace)
		state.Context.Set("react_verified", true)
		return nil
	}})
	if err != nil {
		t.Fatalf("add verify node failed: %v", err)
	}

	parent.SetEntry(initNode)
	parent.SetExit(verifyNode)
	if err = parent.AddEdge(initNode, reactNode); err != nil {
		t.Fatalf("add edge init->react_loop failed: %v", err)
	}
	if err = parent.AddEdge(reactNode, verifyNode); err != nil {
		t.Fatalf("add edge react_loop->verify failed: %v", err)
	}

	if _, err = parent.Run(context.Background(), nil); err != nil {
		t.Fatalf("parent scheduler run failed: %v", err)
	}

	if initNode.State != NodeFinished {
		t.Fatalf("expected init node finished, got %d", initNode.State)
	}
	if reactNode.State != NodeFinished {
		t.Fatalf("expected react loop node finished, got %d", reactNode.State)
	}
	if verifyNode.State != NodeFinished {
		t.Fatalf("expected verify node finished, got %d", verifyNode.State)
	}

	verifiedAny, ok := parent.GraphState.Context.Get("react_verified")
	if !ok {
		t.Fatalf("react_verified not found")
	}
	verified, ok := verifiedAny.(bool)
	if !ok || !verified {
		t.Fatalf("expected react_verified=true, got %v (%T)", verifiedAny, verifiedAny)
	}
}

type DemoNode struct {
	Name     string
	Executed map[string]bool
	Mu       *sync.Mutex
}

func (d *DemoNode) Execute(ctx context.Context, state *GraphState) error {
	fmt.Printf("[%s] executed\n", d.Name)
	if d.Mu != nil && d.Executed != nil {
		d.Mu.Lock()
		d.Executed[d.Name] = true
		d.Mu.Unlock()
	}
	return nil
}

type FuncNode struct {
	Fn func(ctx context.Context, state *GraphState) error
}

func (f *FuncNode) Execute(ctx context.Context, state *GraphState) error {
	if f == nil || f.Fn == nil {
		return nil
	}
	return f.Fn(ctx, state)
}
