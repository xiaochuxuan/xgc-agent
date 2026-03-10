package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"xgc-agent/message"
	"xgc-agent/model/openai"
	"xgc-agent/tools"
)

func printToolCalls(toolCalls []message.ToolCall) {
	for _, tc := range toolCalls {
		args := strings.TrimSpace(string(tc.ToolDefinition.Parameters))
		fmt.Printf("tool_call: id=%s name=%s args=%s\n", tc.ID, tc.ToolDefinition.Name, args)
	}
}

type addArgs struct {
	A int `json:"a"`
	B int `json:"b"`
}

type addOut struct {
	Sum int `json:"sum"`
}

type countArgs struct {
	N int `json:"n"`
}

func newAddTool() tools.BaseTool {
	return tools.NewTool(func(ctx context.Context, in addArgs) (addOut, error) {
		_ = ctx
		return addOut{Sum: in.A + in.B}, nil
	}, tools.WithName("add"), tools.WithDescription("Add two integers and return {sum}."))
}

func newCounterStreamTool() tools.BaseTool {
	return tools.NewStreamTool[countArgs, tools.StreamChunk](func(in countArgs) *tools.StreamReader {
		st := tools.NewStream(8)
		go func() {
			defer st.Writer.Close()
			for i := 1; i <= in.N; i++ {
				// Send each number as a stream chunk.
				if closed := st.Writer.Send(tools.StreamChunk{Data: i}, nil); closed {
					return
				}
				// Slow down a bit so you can see streaming behavior.
				time.Sleep(50 * time.Millisecond)
			}
		}()
		return st.Reader
	}, tools.WithName("counter"), tools.WithDescription("Stream numbers from 1..n as chunks."))
}

type toolRuntime struct {
	tools map[string]tools.BaseTool
}

func (rt toolRuntime) executeToolCall(ctx context.Context, tc message.ToolCall) (message.Message, error) {
	t, ok := rt.tools[tc.ToolDefinition.Name]
	if !ok {
		return message.Message{}, fmt.Errorf("tool not found: %s", tc.ToolDefinition.Name)
	}

	switch t.Type() {
	case tools.ToolTypeNonStreaming:
		tool, err := tools.AsNonStreamingTool(t)
		if err != nil {
			return message.Message{}, err
		}
		out, err := tool.Execute(ctx, tc.ToolDefinition.Parameters)
		if err != nil {
			return message.Message{}, err
		}
		b, err := json.Marshal(out)
		if err != nil {
			return message.Message{}, err
		}
		return message.MessageTool(tc.ID, t.Name(), string(b)), nil

	case tools.ToolTypeStreaming:
		st, err := tools.AsStreamingTool(t)
		if err != nil {
			return message.Message{}, err
		}
		reader, err := st.StreamExecute(ctx, tc.ToolDefinition.Parameters)
		if err != nil {
			return message.Message{}, err
		}
		defer reader.Close()

		// Aggregate all streamed chunks into a JSON array.
		var items []any
		for {
			chunk, err := reader.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return message.Message{}, err
			}
			items = append(items, chunk.Data)
		}
		b, err := json.Marshal(map[string]any{"chunks": items})
		if err != nil {
			return message.Message{}, err
		}
		return message.MessageTool(tc.ID, t.Name(), string(b)), nil
	default:
		return message.Message{}, fmt.Errorf("unsupported tool type: %v", t.Type())
	}
}

func (rt toolRuntime) executeToolCalls(ctx context.Context, assistant message.Message) ([]message.Message, error) {
	out := make([]message.Message, 0, len(assistant.ToolCalls))
	for _, tc := range assistant.ToolCalls {
		m, err := rt.executeToolCall(ctx, tc)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func runNonStreamingModel(ctx context.Context, m *openai.OpenAIChatModel, msgs []message.Message, toolMap map[string]tools.BaseTool, maxTurns int) error {
	rt := toolRuntime{tools: toolMap}
	if maxTurns < 2 {
		// One tool-call round usually needs at least 2 model calls:
		// 1) model emits tool_calls, 2) model produces final answer after tool results.
		maxTurns = 2
	}

	for turn := 0; turn < maxTurns; turn++ {
		fmt.Printf("-- turn %d --\n", turn+1)
		req := &message.Request{Messages: msgs, Tools: toolMap}
		temp := 0.0
		req.Temperature = &temp

		resp, err := m.Generater(ctx, req)
		if err != nil {
			return err
		}
		if resp == nil || len(resp.ResponseChoices) == 0 {
			return fmt.Errorf("empty response")
		}
		choice := resp.ResponseChoices[0]
		assistant := choice.Message
		finish := ""
		if choice.FinishReason != nil {
			finish = *choice.FinishReason
		}
		fmt.Printf("assistant: %s\n", assistant.Content)
		if finish != "" {
			fmt.Printf("finish_reason: %s\n", finish)
		}
		if len(assistant.ToolCalls) > 0 {
			printToolCalls(assistant.ToolCalls)
		}

		// If no tool calls, we are done.
		if len(assistant.ToolCalls) == 0 {
			fmt.Printf("no required tool calls, finished.\n")
			return nil
		}
		fmt.Printf("executing %d tool call(s)...\n", len(assistant.ToolCalls))

		// Append assistant tool-call message, execute tools, append tool results.
		msgs = append(msgs, assistant)
		toolMsgs, err := rt.executeToolCalls(ctx, assistant)
		if err != nil {
			return err
		}
		for _, tm := range toolMsgs {
			fmt.Printf("tool[%s] => %s\n", tm.ToolName, tm.Content)
			msgs = append(msgs, tm)
		}
		fmt.Printf("tool results appended; asking model for final answer...\n")
	}

	return fmt.Errorf("exceeded maxTurns=%d", maxTurns)
}

func runStreamingModel(ctx context.Context, m *openai.OpenAIChatModel, msgs []message.Message, toolMap map[string]tools.BaseTool, maxTurns int) error {
	rt := toolRuntime{tools: toolMap}
	if maxTurns < 2 {
		// Same reason as non-streaming: tool_calls usually require a follow-up model call.
		maxTurns = 2
	}

	for turn := 0; turn < maxTurns; turn++ {
		fmt.Printf("-- turn %d --\n", turn+1)
		req := &message.Request{Messages: msgs, Tools: toolMap}
		temp := 0.0
		req.Temperature = &temp

		responses, errs := m.Stream(ctx, req)

		var final *message.Response
		for r := range responses {
			if r == nil {
				continue
			}
			if r.IsPartial {
				if len(r.ResponseChoices) > 0 {
					delta := r.ResponseChoices[0].Message.Content
					if delta != "" {
						fmt.Print(delta)
					}
				}
				continue
			}
			final = r
		}
		if err, ok := <-errs; ok && err != nil {
			return err
		}
		if final == nil || len(final.ResponseChoices) == 0 {
			return fmt.Errorf("missing final response")
		}
		if final.Error != nil {
			return fmt.Errorf("model error: %s", final.Error.Message)
		}
		fmt.Println()

		choice := final.ResponseChoices[0]
		assistant := choice.Message
		finish := ""
		if choice.FinishReason != nil {
			finish = *choice.FinishReason
		}
		if finish != "" {
			fmt.Printf("finish_reason: %s\n", finish)
		}
		if len(assistant.ToolCalls) > 0 {
			printToolCalls(assistant.ToolCalls)
		}

		// If no tool calls, done.
		if len(assistant.ToolCalls) == 0 {
			fmt.Printf("no required tool calls, finished.\n")
			return nil
		}
		fmt.Printf("executing %d tool call(s)...\n", len(assistant.ToolCalls))

		// Append assistant tool-call message, execute tools, append tool results, then loop.
		msgs = append(msgs, assistant)
		toolMsgs, err := rt.executeToolCalls(ctx, assistant)
		if err != nil {
			return err
		}
		for _, tm := range toolMsgs {
			fmt.Printf("tool[%s] => %s\n", tm.ToolName, tm.Content)
			msgs = append(msgs, tm)
		}
		fmt.Printf("tool results appended; asking model for final answer...\n")
	}
	return fmt.Errorf("exceeded maxTurns=%d", maxTurns)
}

func main() {
	_ = godotenv.Load("example/model/openai/.env")

	apiKey := strings.TrimSpace(os.Getenv("XGC_API_KEY"))
	if apiKey == "" {
		fmt.Println("missing XGC_API_KEY")
		return
	}

	modelName := strings.TrimSpace(os.Getenv("XGC_MODEL"))
	if modelName == "" {
		modelName = "gpt-4o-mini"
	}

	baseURL := strings.TrimSpace(os.Getenv("XGC_BASE_URL"))
	proxyURL := strings.TrimSpace(os.Getenv("XGC_HTTP_PROXY"))

	client := http.DefaultClient
	if proxyURL != "" {
		parsedProxy, err := url.Parse(proxyURL)
		if err != nil {
			fmt.Printf("invalid XGC_HTTP_PROXY: %v\n", err)
			return
		}
		client = &http.Client{
			Transport: &http.Transport{Proxy: http.ProxyURL(parsedProxy)},
		}
	}

	opts := []openai.Option{
		openai.WithAPIKey(apiKey),
		openai.WithHTTPClient(client),
		openai.WithModel(modelName),
	}
	if baseURL != "" {
		opts = append(opts, openai.WithBaseURL(baseURL))
	}

	chatModel, err := openai.NewOpenAIChatModel(opts...)
	if err != nil {
		fmt.Printf("init model error: %v\n", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// ------------------------------
	// Scenario 1: non-stream model + non-stream tool
	// ------------------------------
	fmt.Println("\n== Scenario 1: non-stream model + non-stream tool ==")
	addTool := newAddTool()
	tools1 := map[string]tools.BaseTool{addTool.Name(): addTool}
	msgs1 := []message.Message{
		message.MessageSystem("你是一个严谨的助手。遇到计算问题必须调用工具。"),
		message.MessageUser("请使用 add 工具计算 13 + 29，并只回复结果。"),
	}
	if err := runNonStreamingModel(ctx, chatModel, msgs1, tools1, 1); err != nil {
		fmt.Printf("scenario1 error: %v\n", err)
	}

	// ------------------------------
	// Scenario 2: non-stream model + stream tool
	// ------------------------------
	fmt.Println("\n== Scenario 2: non-stream model + stream tool ==")
	counterTool := newCounterStreamTool()
	tools2 := map[string]tools.BaseTool{counterTool.Name(): counterTool}
	msgs2 := []message.Message{
		message.MessageSystem("你是一个严谨的助手。必须调用工具获取数据。"),
		message.MessageUser("请调用 counter 工具，参数 n=5。工具会返回从 1 到 n 的流式结果。请根据工具结果告诉我最后一个数字是多少。"),
	}
	if err := runNonStreamingModel(ctx, chatModel, msgs2, tools2, 1); err != nil {
		fmt.Printf("scenario2 error: %v\n", err)
	}

	// ------------------------------
	// Scenario 3: stream model + non-stream tool
	// ------------------------------
	fmt.Println("\n== Scenario 3: stream model + non-stream tool ==")
	addTool2 := newAddTool()
	tools3 := map[string]tools.BaseTool{addTool2.Name(): addTool2}
	msgs3 := []message.Message{
		message.MessageSystem("你是一个严谨的助手。遇到计算问题必须调用工具。"),
		message.MessageUser("请使用 add 工具计算 7 + 8，并解释一下结果。"),
	}
	if err := runStreamingModel(ctx, chatModel, msgs3, tools3, 1); err != nil {
		fmt.Printf("scenario3 error: %v\n", err)
	}

	// ------------------------------
	// Scenario 4: stream model + stream tool
	// ------------------------------
	fmt.Println("\n== Scenario 4: stream model + stream tool ==")
	counterTool2 := newCounterStreamTool()
	tools4 := map[string]tools.BaseTool{counterTool2.Name(): counterTool2}
	msgs4 := []message.Message{
		message.MessageSystem("你是一个严谨的助手。必须调用工具获取数据。"),
		message.MessageUser("请调用 counter 工具，参数 n=3。根据工具返回的流式 chunks，总结一下返回了哪些数字。"),
	}
	if err := runStreamingModel(ctx, chatModel, msgs4, tools4, 1); err != nil {
		fmt.Printf("scenario4 error: %v\n", err)
	}
}
