package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// MCPTool represents a tool that executes by calling an MCP server.
type MCPTool interface {
	Execute(ctx context.Context, args []byte) (any, error)
	BaseTool
}

// MCPToolCli wraps an MCP server tool as a local Tool implementation.
type MCPToolCli struct {
	client *MCPClient
	def    MCPToolDef
}

func NewMCPTool(client *MCPClient, def MCPToolDef) *MCPToolCli {
	return &MCPToolCli{client: client, def: def}
}

func (t *MCPToolCli) Name() string        { return t.def.Name }
func (t *MCPToolCli) Description() string { return t.def.Description }
func (t *MCPToolCli) Type() ToolType      { return ToolTypeMCP }

func (t *MCPToolCli) Schema() ToolSchema {
	var input Schema
	if len(t.def.InputSchema) > 0 {
		_ = json.Unmarshal(t.def.InputSchema, &input)
	}
	return ToolSchema{Input: &input}
}

func (t *MCPToolCli) Execute(ctx context.Context, args []byte) (any, error) {
	result, err := t.client.CallTool(ctx, t.def.Name, json.RawMessage(args))
	if err != nil {
		return nil, err
	}
	if result.IsError {
		if text := extractMCPTextResult(result); text != "" {
			return nil, fmt.Errorf("mcp tool %s: %s", t.def.Name, text)
		}
		return nil, fmt.Errorf("mcp tool %s failed", t.def.Name)
	}
	if result.StructuredContent != nil {
		return result.StructuredContent, nil
	}
	if text := extractMCPTextResult(result); text != "" {
		return text, nil
	}
	data, err := json.Marshal(result)
	if err != nil {
		return result, nil
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		return string(data), nil
	}
	return out, nil
}

func extractMCPTextResult(result *mcp.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	parts := make([]string, 0, len(result.Content))
	for _, item := range result.Content {
		if text, ok := item.(mcp.TextContent); ok {
			trimmed := strings.TrimSpace(text.Text)
			if trimmed != "" {
				parts = append(parts, trimmed)
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}
