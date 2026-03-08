package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"xgc-agent/message"
	"xgc-agent/prompt"
	"xgc-agent/tools"
)

// ReActAgent implements the ReAct (Reasoning + Acting) paradigm.
// A Simple implementation of ReAct agent that supports tool calls and iterative reasoning.
// Think → Act → Observe loop until done or max iterations reached.
type ReActAgent struct {
	*Agent
	maxIterations int
}

func NewReActAgent(cfg Config, opts ...AgentOption) *ReActAgent {
	maxIter := cfg.MaxIterations
	if maxIter <= 0 {
		maxIter = 10
	}
	return &ReActAgent{
		Agent:         NewAgent(cfg, opts...),
		maxIterations: maxIter,
	}
}

func (a *ReActAgent) Run(ctx context.Context, input string) (string, error) {
	systemPrompt := prompt.ReActTemplateZH
	if a.config.Locale == "en" {
		systemPrompt = prompt.ReActTemplateEN
	}

	msgs := []message.Message{
		{Role: message.RoleSystem, Content: systemPrompt},
		{Role: message.RoleUser, Content: input},
	}

	toolMap := make(map[string]tools.BaseTool)
	if a.tools != nil {
		for _, t := range a.tools.List() {
			toolMap[t.Name()] = t
		}
	}

	for i := 0; i < a.maxIterations; i++ {
		req := &message.Request{Messages: msgs, Tools: toolMap}
		resp, err := a.model.Generater(ctx, req)
		if err != nil {
			return "", fmt.Errorf("react generate: %w", err)
		}
		if len(resp.ResponseChoices) == 0 {
			return "", nil
		}

		choice := resp.ResponseChoices[0]
		if len(choice.Message.ToolCalls) == 0 {
			return choice.Message.Content, nil
		}

		msgs = append(msgs, choice.Message)
		for _, tc := range choice.Message.ToolCalls {
			result, execErr := a.executeTool(ctx, tc)
			toolMsg := message.Message{
				Role:     message.RoleTool,
				ToolID:   tc.ID,
				ToolName: tc.ToolDefinition.Name,
			}
			if execErr != nil {
				toolMsg.Content = fmt.Sprintf("error: %v", execErr)
			} else {
				b, _ := json.Marshal(result)
				toolMsg.Content = string(b)
			}
			msgs = append(msgs, toolMsg)
		}
	}
	return "", fmt.Errorf("react: max iterations (%d) exceeded", a.maxIterations)
}
