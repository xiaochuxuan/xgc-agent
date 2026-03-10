package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func newTestMCPClient(t *testing.T) *MCPClient {
	t.Helper()

	srv := server.NewMCPServer("test-mcp", "1.0.0")
	srv.AddTool(
		mcp.NewToolWithRawSchema(
			"echo",
			"echo input text",
			json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}`),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			text, err := request.RequireString("text")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultStructured(map[string]any{"echo": text}, text), nil
		},
	)
	srv.AddTool(
		mcp.NewToolWithRawSchema(
			"text_only",
			"return text only",
			json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}`),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			text, err := request.RequireString("text")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText("  first line  \n" + text), nil
		},
	)
	srv.AddTool(
		mcp.NewToolWithRawSchema(
			"optional",
			"accept empty arguments",
			json.RawMessage(`{"type":"object","properties":{}}`),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return mcp.NewToolResultStructured(map[string]any{"ok": true}, "optional ok"), nil
		},
	)
	srv.AddTool(
		mcp.NewToolWithRawSchema(
			"fail",
			"always fail",
			json.RawMessage(`{"type":"object","properties":{}}`),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return mcp.NewToolResultError("boom"), nil
		},
	)
	srv.AddTool(
		mcp.NewToolWithRawSchema(
			"fail_empty",
			"fail without text",
			json.RawMessage(`{"type":"object","properties":{}}`),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{IsError: true}, nil
		},
	)

	raw, err := mcpclient.NewInProcessClient(srv)
	if err != nil {
		t.Fatalf("NewInProcessClient: %v", err)
	}
	client, err := NewMCPClientFromRaw(raw, "test-client", "test-version")
	if err != nil {
		t.Fatalf("NewMCPClientFromRaw: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func defsByName(defs []MCPToolDef) map[string]MCPToolDef {
	out := make(map[string]MCPToolDef, len(defs))
	for _, def := range defs {
		out[def.Name] = def
	}
	return out
}

func TestNewMCPClientWithConfig_RejectsEmptyEndpoint(t *testing.T) {
	_, err := NewMCPClientWithConfig(MCPClientConfig{})
	if err == nil {
		t.Fatalf("NewMCPClientWithConfig expected error")
	}
	if !strings.Contains(err.Error(), "empty endpoint") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestNewMCPClientFromRaw_RejectsNilClient(t *testing.T) {
	_, err := NewMCPClientFromRaw(nil, "name", "version")
	if err == nil {
		t.Fatalf("NewMCPClientFromRaw(nil) expected error")
	}
	if !strings.Contains(err.Error(), "nil client") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestNewMCPClientFromRaw_PreservesMetadata(t *testing.T) {
	client := newTestMCPClient(t)
	if client.ClientName != "test-client" {
		t.Fatalf("ClientName=%q, want test-client", client.ClientName)
	}
	if client.Version != "test-version" {
		t.Fatalf("Version=%q, want test-version", client.Version)
	}
}

func TestMCPClient_DiscoverTools_ReturnsDefinitions(t *testing.T) {
	client := newTestMCPClient(t)

	defs, err := client.DiscoverTools(context.Background())
	if err != nil {
		t.Fatalf("DiscoverTools: %v", err)
	}
	if len(defs) != 5 {
		t.Fatalf("DiscoverTools len=%d, want 5", len(defs))
	}

	byName := defsByName(defs)
	for _, name := range []string{"echo", "text_only", "optional", "fail", "fail_empty"} {
		if _, ok := byName[name]; !ok {
			t.Fatalf("DiscoverTools missing %q", name)
		}
	}
	if byName["echo"].Description != "echo input text" {
		t.Fatalf("echo description=%q", byName["echo"].Description)
	}
	if !strings.Contains(string(byName["echo"].InputSchema), `"text"`) {
		t.Fatalf("echo schema=%s, want property text", byName["echo"].InputSchema)
	}
}

func TestMCPClient_CallTool_DecodesArgsAndReturnsResult(t *testing.T) {
	client := newTestMCPClient(t)

	result, err := client.CallTool(context.Background(), "echo", json.RawMessage(`{"text":"hi"}`))
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result == nil {
		t.Fatalf("CallTool result is nil")
	}
	if result.IsError {
		t.Fatalf("CallTool result IsError=true")
	}
	obj, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("StructuredContent type=%T, want map[string]any", result.StructuredContent)
	}
	if obj["echo"] != "hi" {
		t.Fatalf("StructuredContent echo=%v, want hi", obj["echo"])
	}
}

func TestMCPClient_CallTool_AllowsNilEmptyOrNullArgs(t *testing.T) {
	client := newTestMCPClient(t)

	cases := []struct {
		name string
		args json.RawMessage
	}{
		{name: "nil", args: nil},
		{name: "empty", args: json.RawMessage{}},
		{name: "null", args: json.RawMessage("null")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := client.CallTool(context.Background(), "optional", tc.args)
			if err != nil {
				t.Fatalf("CallTool(%s): %v", tc.name, err)
			}
			obj, ok := result.StructuredContent.(map[string]any)
			if !ok {
				t.Fatalf("StructuredContent type=%T, want map[string]any", result.StructuredContent)
			}
			if obj["ok"] != true {
				t.Fatalf("StructuredContent ok=%v, want true", obj["ok"])
			}
		})
	}
}

func TestMCPClient_CallTool_RejectsInvalidJSONArgs(t *testing.T) {
	client := newTestMCPClient(t)

	_, err := client.CallTool(context.Background(), "echo", json.RawMessage(`{"text":}`))
	if err == nil {
		t.Fatalf("CallTool expected error")
	}
	if !strings.Contains(err.Error(), "mcp call decode args") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestMCPTool_MetadataAndSchema(t *testing.T) {
	tool := NewMCPTool(newTestMCPClient(t), MCPToolDef{
		Name:        "echo",
		Description: "echo input text",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}`),
	})

	if tool.Name() != "echo" {
		t.Fatalf("Name=%q, want echo", tool.Name())
	}
	if tool.Description() != "echo input text" {
		t.Fatalf("Description=%q, want echo input text", tool.Description())
	}
	if tool.Type() != ToolTypeMCP {
		t.Fatalf("Type=%v, want %v", tool.Type(), ToolTypeMCP)
	}
	if tool.Schema().Input == nil {
		t.Fatalf("Schema().Input is nil")
	}
	if tool.Schema().Input.Type != "object" {
		t.Fatalf("Schema().Input.Type=%q, want object", tool.Schema().Input.Type)
	}
	if tool.Schema().Input.Properties["text"] == nil || tool.Schema().Input.Properties["text"].Type != "string" {
		t.Fatalf("Schema().Input.Properties[text]=%v, want string property", tool.Schema().Input.Properties["text"])
	}
}

func TestMCPTool_Schema_InvalidJSONFallsBackToEmptySchema(t *testing.T) {
	tool := NewMCPTool(newTestMCPClient(t), MCPToolDef{Name: "bad", InputSchema: json.RawMessage(`{"type":`)})
	if tool.Schema().Input == nil {
		t.Fatalf("Schema().Input is nil")
	}
	if tool.Schema().Input.Type != "" {
		t.Fatalf("Schema().Input.Type=%q, want empty string", tool.Schema().Input.Type)
	}
}

func TestMCPTool_Execute_ReturnsStructuredContent(t *testing.T) {
	client := newTestMCPClient(t)
	echo := NewMCPTool(client, MCPToolDef{
		Name:        "echo",
		Description: "echo input text",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}`),
	})

	out, err := echo.Execute(context.Background(), []byte(`{"text":"hi"}`))
	if err != nil {
		t.Fatalf("echo.Execute: %v", err)
	}
	obj, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("echo.Execute type=%T, want map[string]any", out)
	}
	if obj["echo"] != "hi" {
		t.Fatalf("echo.Execute echo=%v, want hi", obj["echo"])
	}
}

func TestMCPTool_Execute_ReturnsJoinedTextContent(t *testing.T) {
	client := newTestMCPClient(t)
	tool := NewMCPTool(client, MCPToolDef{
		Name:        "text_only",
		Description: "return text only",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}`),
	})

	out, err := tool.Execute(context.Background(), []byte(`{"text":"second line"}`))
	if err != nil {
		t.Fatalf("text_only.Execute: %v", err)
	}
	text, ok := out.(string)
	if !ok {
		t.Fatalf("text_only.Execute type=%T, want string", out)
	}
	if text != "first line  \nsecond line" {
		t.Fatalf("text_only.Execute=%q, want returned text content", text)
	}
}

func TestExtractMCPTextResult_TrimsAndJoinsTextContent(t *testing.T) {
	result := &mcp.CallToolResult{Content: []mcp.Content{
		mcp.NewTextContent("  first line  "),
		mcp.NewTextContent(""),
		mcp.NewTextContent(" second line "),
	}}

	if got := extractMCPTextResult(result); got != "first line\nsecond line" {
		t.Fatalf("extractMCPTextResult()=%q, want joined trimmed text", got)
	}
}

func TestMCPTool_Execute_ReturnsToolErrorWithText(t *testing.T) {
	client := newTestMCPClient(t)
	fail := NewMCPTool(client, MCPToolDef{Name: "fail", Description: "always fail", InputSchema: json.RawMessage(`{"type":"object"}`)})

	_, err := fail.Execute(context.Background(), []byte(`{}`))
	if err == nil {
		t.Fatalf("fail.Execute expected error, got nil")
	}
	if !strings.Contains(err.Error(), "mcp tool fail: boom") {
		t.Fatalf("fail.Execute err=%q, want contain mcp tool fail: boom", err.Error())
	}
}

func TestMCPTool_Execute_ReturnsGenericErrorWithoutText(t *testing.T) {
	client := newTestMCPClient(t)
	fail := NewMCPTool(client, MCPToolDef{Name: "fail_empty", Description: "fail without text", InputSchema: json.RawMessage(`{"type":"object"}`)})

	_, err := fail.Execute(context.Background(), []byte(`{}`))
	if err == nil {
		t.Fatalf("fail_empty.Execute expected error, got nil")
	}
	if !strings.Contains(err.Error(), "mcp tool fail_empty failed") {
		t.Fatalf("fail_empty.Execute err=%q, want generic tool failure", err.Error())
	}
}

func TestRegistry_WithMCPTools_RegisterGetSchemasAndAsMCPTool(t *testing.T) {
	client := newTestMCPClient(t)
	defs, err := client.DiscoverTools(context.Background())
	if err != nil {
		t.Fatalf("DiscoverTools: %v", err)
	}

	reg := NewRegistry(DefaultMaxTools)
	for _, def := range defs {
		if err := reg.Register(NewMCPTool(client, def)); err != nil {
			t.Fatalf("Register discovered tool %s: %v", def.Name, err)
		}
	}

	tool, ok := reg.Get("echo")
	if !ok {
		t.Fatalf("Get(echo) ok=false")
	}
	mcpTool, err := AsMCPTool(tool)
	if err != nil {
		t.Fatalf("AsMCPTool: %v", err)
	}
	out, err := mcpTool.Execute(context.Background(), []byte(`{"text":"registry"}`))
	if err != nil {
		t.Fatalf("registered Execute: %v", err)
	}
	obj, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("registered Execute type=%T, want map[string]any", out)
	}
	if obj["echo"] != "registry" {
		t.Fatalf("registered Execute echo=%v, want registry", obj["echo"])
	}

	if _, err := AsNonStreamingTool(tool); err == nil {
		t.Fatalf("AsNonStreamingTool expected error for MCP tool")
	}

	schemas := reg.Schemas()
	if len(schemas) != len(defs) {
		t.Fatalf("Schemas len=%d, want %d", len(schemas), len(defs))
	}
	foundObjectSchema := false
	for _, schema := range schemas {
		if schema.Input != nil && schema.Input.Type == "object" {
			foundObjectSchema = true
			break
		}
	}
	if !foundObjectSchema {
		t.Fatalf("Schemas did not include object input schema")
	}
}
