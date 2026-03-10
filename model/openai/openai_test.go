package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"xgc-agent/message"
	agenttools "xgc-agent/tools"
)

type chatCompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int    `json:"index"`
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func newTestModel(t *testing.T, baseURL string) *OpenAIChatModel {
	t.Helper()

	m, err := NewOpenAIChatModel(
		WithAPIKey("test-key"),
		WithBaseURL(baseURL),
		WithHTTPClient(http.DefaultClient),
		WithModel("gpt-4o-mini"),
	)
	if err != nil {
		t.Fatalf("new model: %v", err)
	}

	// Ensure Stream has enough buffer to avoid blocking before returning.
	m.channelBufferSize = 8
	return m
}

func TestOpenAIChatModel_Generater(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		resp := chatCompletionResponse{
			ID:      "chatcmpl-test",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-4o-mini",
		}
		resp.Choices = append(resp.Choices, struct {
			Index        int    `json:"index"`
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
		}{
			Index:        0,
			FinishReason: "stop",
			Message: struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			}{Role: "assistant", Content: "hello"},
		})
		resp.Usage.PromptTokens = 5
		resp.Usage.CompletionTokens = 1
		resp.Usage.TotalTokens = 6

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	m := newTestModel(t, server.URL)

	req := &message.Request{
		Messages: []message.Message{message.MessageUser("hi")},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	resp, err := m.Generater(ctx, req)
	if err != nil {
		t.Fatalf("Generater error: %v", err)
	}
	if resp == nil {
		t.Fatal("Generater response is nil")
	}
	if !resp.Done {
		t.Fatalf("expected Done=true")
	}
	if len(resp.ResponseChoices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(resp.ResponseChoices))
	}
	if resp.ResponseChoices[0].Message.Content != "hello" {
		t.Fatalf("unexpected content: %q", resp.ResponseChoices[0].Message.Content)
	}
	if resp.Usage == nil {
		t.Fatalf("expected usage to be present")
	}
	if resp.Usage.TotalTokens != 6 {
		t.Fatalf("unexpected total tokens: %d", resp.Usage.TotalTokens)
	}
}

func TestOpenAIChatModel_Stream(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		chunks := []string{
			`{"id":"chatcmpl-stream","object":"chat.completion.chunk","created":123,"model":"gpt-4o-mini","choices":[{"index":0,"delta":{"content":"he"}}]}`,
			`{"id":"chatcmpl-stream","object":"chat.completion.chunk","created":123,"model":"gpt-4o-mini","choices":[{"index":0,"delta":{"content":"llo"}}]}`,
		}

		for _, chunk := range chunks {
			_, _ = w.Write([]byte("data: " + chunk + "\n\n"))
			flusher.Flush()
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	})

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	m := newTestModel(t, server.URL)

	req := &message.Request{
		Messages: []message.Message{message.MessageUser("hi")},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	responses, errs := m.Stream(ctx, req)

	readWithTimeout := func() *message.Response {
		select {
		case resp := <-responses:
			return resp
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for stream response")
			return nil
		}
	}

	var partialContent string
	partialCount := 0
	var final *message.Response

	for {
		resp := readWithTimeout()
		if resp == nil {
			t.Fatal("stream response is nil")
		}

		if resp.IsPartial {
			partialCount++
			if len(resp.ResponseChoices) > 0 {
				partialContent += resp.ResponseChoices[0].Message.Content
			}
			continue
		}

		final = resp
		break
	}

	if partialCount < 2 {
		t.Fatalf("expected at least 2 partial responses, got %d", partialCount)
	}
	if partialContent != "hello" {
		t.Fatalf("unexpected streamed partial content: %q", partialContent)
	}
	if final == nil {
		t.Fatal("missing final response")
	}
	if !final.Done {
		t.Fatalf("expected final Done=true")
	}
	if len(final.ResponseChoices) != 1 {
		t.Fatalf("expected 1 final choice, got %d", len(final.ResponseChoices))
	}
	if final.ResponseChoices[0].Message.Content != "hello" {
		t.Fatalf("unexpected final content: %q", final.ResponseChoices[0].Message.Content)
	}

	select {
	case err := <-errs:
		if err != nil {
			t.Fatalf("stream error: %v", err)
		}
	default:
	}
}

func TestOpenAIChatModel_ToolCallRoundTrip(t *testing.T) {
	type addArgs struct {
		A int `json:"a"`
		B int `json:"b"`
	}
	type addOut struct {
		Sum int `json:"sum"`
	}

	addTool := agenttools.NewTool(func(ctx context.Context, in addArgs) (addOut, error) {
		_ = ctx
		return addOut{Sum: in.A + in.B}, nil
	}, agenttools.WithName("add"), agenttools.WithDescription("add two integers"))

	toolsMap := map[string]agenttools.BaseTool{addTool.Name(): addTool}

	requestCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		requestCount++

		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()

		var req map[string]any
		if err := json.Unmarshal(body, &req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(err.Error()))
			return
		}

		// First request: model should receive tool schemas.
		if requestCount == 1 {
			toolsVal, ok := req["tools"].([]any)
			if !ok || len(toolsVal) == 0 {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("missing tools in request"))
				return
			}

			// Respond with a tool call.
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"id":"chatcmpl-toolcall",
				"object":"chat.completion",
				"created":123,
				"model":"gpt-4o-mini",
				"choices":[{
					"index":0,
					"message":{
						"role":"assistant",
						"content":"",
						"tool_calls":[{
							"id":"call_1",
							"type":"function",
							"function":{"name":"add","arguments":"{\"a\":1,\"b\":2}"}
						}]
					},
					"finish_reason":"tool_calls"
				}],
				"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}
			}`))
			return
		}

		// Second request: should include a tool message with the result.
		messagesVal, ok := req["messages"].([]any)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("missing messages"))
			return
		}
		foundToolMsg := false
		for _, m := range messagesVal {
			mm, ok := m.(map[string]any)
			if !ok {
				continue
			}
			role, _ := mm["role"].(string)
			if role != "tool" {
				continue
			}
			toolCallID, _ := mm["tool_call_id"].(string)
			content, _ := mm["content"].(string)
			if toolCallID == "call_1" && content != "" {
				foundToolMsg = true
				break
			}
		}
		if !foundToolMsg {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("missing tool result message"))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl-toolfinal",
			"object":"chat.completion",
			"created":123,
			"model":"gpt-4o-mini",
			"choices":[{
				"index":0,
				"message":{"role":"assistant","content":"sum is 3"},
				"finish_reason":"stop"
			}],
			"usage":{"prompt_tokens":12,"completion_tokens":3,"total_tokens":15}
		}`))
	})

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	m := newTestModel(t, server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	// 1) Ask the model a question with tools available.
	req1 := &message.Request{
		Messages: []message.Message{message.MessageUser("what is 1+2?")},
		Tools:    toolsMap,
	}

	resp1, err := m.Generater(ctx, req1)
	if err != nil {
		t.Fatalf("Generater (tool call) error: %v", err)
	}
	if resp1 == nil || len(resp1.ResponseChoices) != 1 {
		t.Fatalf("unexpected resp1: %#v", resp1)
	}
	if len(resp1.ResponseChoices[0].Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp1.ResponseChoices[0].Message.ToolCalls))
	}

	tc := resp1.ResponseChoices[0].Message.ToolCalls[0]
	if tc.ID != "call_1" {
		t.Fatalf("unexpected tool call id: %q", tc.ID)
	}
	if tc.ToolDefinition.Name != "add" {
		t.Fatalf("unexpected tool name: %q", tc.ToolDefinition.Name)
	}

	// 2) Execute the tool locally.
	toolBase, ok := toolsMap[tc.ToolDefinition.Name]
	if !ok {
		t.Fatalf("tool not found: %s", tc.ToolDefinition.Name)
	}
	tool, err := agenttools.AsNonStreamingTool(toolBase)
	if err != nil {
		t.Fatalf("tool is not non-streaming: %v", err)
	}
	out, err := tool.Execute(ctx, tc.ToolDefinition.Parameters)
	if err != nil {
		t.Fatalf("tool execute error: %v", err)
	}
	outJSON, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal tool output: %v", err)
	}

	// 3) Send the tool result back to the model.
	assistantToolCallMsg := message.Message{
		Role:      message.RoleAssistant,
		Content:   "",
		ToolCalls: []message.ToolCall{tc},
	}
	toolResultMsg := message.MessageTool(tc.ID, tc.ToolDefinition.Name, string(bytes.TrimSpace(outJSON)))

	req2 := &message.Request{
		Messages: []message.Message{
			message.MessageUser("what is 1+2?"),
			assistantToolCallMsg,
			toolResultMsg,
		},
		Tools: toolsMap,
	}

	resp2, err := m.Generater(ctx, req2)
	if err != nil {
		t.Fatalf("Generater (final) error: %v", err)
	}
	if resp2 == nil || len(resp2.ResponseChoices) != 1 {
		t.Fatalf("unexpected resp2: %#v", resp2)
	}
	if resp2.ResponseChoices[0].Message.Content != "sum is 3" {
		t.Fatalf("unexpected final content: %q", resp2.ResponseChoices[0].Message.Content)
	}
}
