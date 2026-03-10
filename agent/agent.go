package agent

import (
	"context"
	"fmt"
	"xgc-agent/memory/manager"
	"xgc-agent/message"
	"xgc-agent/model"
	"xgc-agent/sandbox"
	"xgc-agent/session"
	"xgc-agent/tools"
)

// BaseAgent defines the minimal interface for an agent, which can be used in a scheduler or directly in code.
type BaseAgent interface {
	Run(ctx context.Context, input string) (string, error)
}

// AgentHooks provides lifecycle callbacks.
// TODO: implement it
type AgentHooks struct {
	BeforeCall func(ctx context.Context, toolName string, args []byte) error
	AfterCall  func(ctx context.Context, toolName string, result any) error
	OnError    func(ctx context.Context, err error) error
}

// Agent orchestrates model, tools, memory, and sandbox into a unified assistant.
// It can be set in a scheduler node as a task handler, or used directly in code.
type Agent struct {
	Name    string
	model   model.BaseModel
	tools   *tools.Registry
	memory  *manager.MemoryManager
	sandbox sandbox.Sandbox
	session *session.Session
	hooks   AgentHooks
	config  Config
}

type AgentOption func(*Agent)

func WithMemory(m *manager.MemoryManager) AgentOption {
	return func(a *Agent) { a.memory = m }
}

func WithSandbox(s sandbox.Sandbox) AgentOption {
	return func(a *Agent) { a.sandbox = s }
}

func WithSession(s *session.Session) AgentOption {
	return func(a *Agent) { a.session = s }
}

func WithHooks(h AgentHooks) AgentOption {
	return func(a *Agent) { a.hooks = h }
}

func NewAgent(cfg Config, opts ...AgentOption) *Agent {
	a := &Agent{
		Name:   cfg.Name,
		model:  cfg.Model,
		tools:  cfg.Tools,
		config: cfg,
	}
	for _, o := range opts {
		o(a)
	}
	return a
}

func (a *Agent) executeTool(ctx context.Context, tc message.ToolCall) (any, error) {
	name := tc.ToolDefinition.Name
	if a.hooks.BeforeCall != nil {
		if err := a.hooks.BeforeCall(ctx, name, tc.ToolDefinition.Parameters); err != nil {
			return nil, err
		}
	}

	t, ok := a.tools.Get(name)
	if !ok {
		return nil, fmt.Errorf("tool %q not found", name)
	}
	tool, err := tools.AsNonStreamingTool(t)
	if err != nil {
		return nil, err
	}

	result, err := tool.Execute(ctx, tc.ToolDefinition.Parameters)
	if err != nil {
		if a.hooks.OnError != nil {
			_ = a.hooks.OnError(ctx, err)
		}
		return nil, err
	}

	if a.hooks.AfterCall != nil {
		if err := a.hooks.AfterCall(ctx, name, result); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (a *Agent) sessionID() string {
	if a.session != nil {
		return a.session.ID
	}
	return ""
}
