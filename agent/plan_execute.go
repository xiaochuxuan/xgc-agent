package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"xgc-agent/message"
	"xgc-agent/prompt"
	"xgc-agent/tools"
)

// PlanExecuteAgent implements the Plan-and-Execute paradigm.
// First generates a plan, then executes each step with tool calls.
type PlanExecuteAgent struct {
	*Agent
	maxSteps int
}

func NewPlanExecuteAgent(cfg Config, opts ...AgentOption) *PlanExecuteAgent {
	maxSteps := cfg.MaxIterations
	if maxSteps <= 0 {
		maxSteps = 10
	}
	return &PlanExecuteAgent{
		Agent:    NewAgent(cfg, opts...),
		maxSteps: maxSteps,
	}
}

func (a *PlanExecuteAgent) Run(ctx context.Context, input string) (string, error) {
	systemPrompt := prompt.PlanExecuteTemplateZH
	if a.config.Locale == "en" {
		systemPrompt = prompt.PlanExecuteTemplateEN
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

	for i := 0; i < a.maxSteps; i++ {
		req := &message.Request{Messages: msgs, Tools: toolMap}
		resp, err := a.model.Generater(ctx, req)
		if err != nil {
			return "", fmt.Errorf("plan-execute generate: %w", err)
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
	return "", fmt.Errorf("plan-execute: max steps (%d) exceeded", a.maxSteps)
}
