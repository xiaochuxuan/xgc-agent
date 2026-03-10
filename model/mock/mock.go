// // mock.go: Offline mock chat model implementation.
package mock

// import (
// 	"context"
// 	"strings"
// 	"time"

// 	"xgc-agent/model"
// )

// func init() {
// 	model.RegisterProvider(
// 		"mock",
// 		func(cfg model.Config) (model.ChatModel, error) { return NewChatModel(cfg), nil },
// 	)
// }

// type ChatModel struct {
// 	defaultModel string
// 	Delay        time.Duration
// }

// func NewChatModel(cfg model.Configs) *ChatModel {
// 	m := &ChatModel{defaultModel: strings.TrimSpace(cfg.Model)}
// 	if m.defaultModel == "" {
// 		m.defaultModel = "mock-001"
// 	}
// 	m.Delay = 10 * time.Millisecond
// 	return m
// }

// func (m *ChatModel) Generater(ctx context.Context, req *model.ChatRequest) (*model.ChatResponse, error) {
// 	_ = ctx
// 	if req == nil {
// 		return nil, model.ErrNilRequest
// 	}
// 	content := "mock: " + lastUserContent(req.Messages)
// 	return &model.ChatResponse{Model: m.defaultModel, Content: content}, nil
// }

// func (m *ChatModel) Stream(ctx context.Context, req *model.ChatRequest) (<-chan model.StreamEvent, <-chan error) {
// 	events := make(chan model.StreamEvent, 32)
// 	errs := make(chan error, 1)

// 	go func() {
// 		defer close(events)
// 		defer close(errs)

// 		if req == nil {
// 			errs <- model.ErrNilRequest
// 			return
// 		}

// 		text := "mock: " + lastUserContent(req.Messages)
// 		for _, r := range []rune(text) {
// 			select {
// 			case <-ctx.Done():
// 				errs <- ctx.Err()
// 				return
// 			case events <- model.StreamEvent{Delta: string(r)}:
// 				if m.Delay > 0 {
// 					time.Sleep(m.Delay)
// 				}
// 			}
// 		}

// 		events <- model.StreamEvent{Done: true}
// 	}()

// 	return events, errs
// }

// func lastUserContent(messages []model.Messages) string {
// 	for i := len(messages) - 1; i >= 0; i-- {
// 		if strings.ToLower(strings.TrimSpace(messages[i].Role)) == "user" {
// 			return messages[i].Content
// 		}
// 	}
// 	if len(messages) == 0 {
// 		return ""
// 	}
// 	return messages[len(messages)-1].Content
// }

// var _ model.ChatModel = (*ChatModel)(nil)
