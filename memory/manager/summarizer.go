package manager

import (
	"context"
	"xgc-agent/message"
	"xgc-agent/model"
)

// Summarizer generates a summary from a list of messages.
type Summarizer interface {
	Summarize(ctx context.Context, messages []message.Message) (string, error)
}

// LLMSummarizer uses a BaseModel to generate conversation summaries.
type LLMSummarizer struct {
	model model.BaseModel
}

func NewLLMSummarizer(m model.BaseModel) *LLMSummarizer {
	return &LLMSummarizer{model: m}
}

func (s *LLMSummarizer) Summarize(ctx context.Context, messages []message.Message) (string, error) {
	prompt := message.Message{
		Role:    message.RoleSystem,
		Content: "请将以下对话内容总结为简洁的摘要，保留关键信息和决策要点。用中文回答。",
	}
	req := &message.Request{
		Messages: append([]message.Message{prompt}, messages...),
	}
	resp, err := s.model.Generater(ctx, req)
	if err != nil {
		return "", err
	}
	if len(resp.ResponseChoices) == 0 {
		return "", nil
	}
	return resp.ResponseChoices[0].Message.Content, nil
}

var _ Summarizer = (*LLMSummarizer)(nil)
