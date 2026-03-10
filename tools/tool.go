package tools

import (
	"encoding/json"
)

// ToolType represents the type of the tool.
// represents by an unsigned integer.
type ToolType uint

const (
	// ToolTypeNonStreaming represents a non-streaming tool.
	ToolTypeNonStreaming ToolType = iota
	// ToolTypeStreaming represents a streaming tool.
	ToolTypeStreaming
	// ToolTypeMCP represents a tool that wraps an MCP server tool.
	ToolTypeMCP
)

// BaseTool is an interface for tools that can be used by the agent.
// all tools must implement it.
type BaseTool interface {
	// Name returns the name of the tool.
	Name() string
	// Description returns a brief description of the tool.
	Description() string
	// Schema returns the JSON Schema definition used for model tool calling.
	Schema() ToolSchema
	// Type returns the type of the tool: "streaming" or "non-streaming"(or others in future).
	Type() ToolType
}

// toolOptions holds configuration for CallableTool.
// it supports user-defined unmarshaling functions and other options.
type toolOptions struct {
	// add configuration fields as needed
	name        string
	description string
	unmarshal   func([]byte, any) error
}

type Option func(*toolOptions)

func WithName(name string) Option {
	return func(opts *toolOptions) {
		opts.name = name
	}
}

func WithDescription(description string) Option {
	return func(opts *toolOptions) {
		opts.description = description
	}
}

func WithUnmarshalFunc(fn func([]byte, any) error) Option {
	return func(opts *toolOptions) {
		opts.unmarshal = fn
	}
}

func Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
