package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"xgc-agent/memory"
	"xgc-agent/memory/manager"
	"xgc-agent/message"
	"xgc-agent/model"
	openaimodel "xgc-agent/model/openai"

	"github.com/joho/godotenv"
)

func main() {
	cfg, ok := loadConfig()
	if !ok {
		return
	}

	httpClient := &http.Client{Timeout: 45 * time.Second}
	opts := []openaimodel.Option{
		openaimodel.WithAPIKey(cfg.apiKey),
		openaimodel.WithModel(cfg.model),
		openaimodel.WithHTTPClient(httpClient),
	}
	if cfg.baseURL != "" {
		opts = append(opts, openaimodel.WithBaseURL(cfg.baseURL))
	}

	chatModel, err := openaimodel.NewOpenAIChatModel(opts...)
	if err != nil {
		fmt.Printf("init model error: %v\n", err)
		return
	}

	analyzer := manager.NewLLMPromotionAnalyzer(&openAIAnalyzerModelAdapter{inner: chatModel})

	scenarios := []struct {
		name     string
		messages []message.Message
		wantType memory.MemoryType
	}{
		{
			name: "semantic_preference",
			messages: []message.Message{
				{Role: message.RoleUser, Content: "后续回复请用中文短句和项目符号，越简洁越好。"},
				{Role: message.RoleAssistant, Content: "收到，我会用中文简短要点回答。"},
			},
			wantType: memory.MemorySemantic,
		},
		{
			name: "episodic_specific_event",
			messages: []message.Message{
				{Role: message.RoleUser, Content: "昨天我和Bob上线了支付服务，之后修复了超时告警，最后监控恢复正常。"},
				{Role: message.RoleAssistant, Content: "明白，这次上线和故障处理已完成。"},
			},
			wantType: memory.MemoryEpisodic,
		},
	}

	passCount := 0
	for _, sc := range scenarios {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		decision, err := analyzer.Analyze(ctx, sc.messages)
		cancel()

		fmt.Printf("\n=== Scenario: %s ===\n", sc.name)
		fmt.Printf("Expected Type: %s\n", sc.wantType)
		if err != nil {
			fmt.Printf("Analyze error: %v\n", err)
			continue
		}

		fmt.Printf("Predicted Type: %s\n", decision.Type)
		fmt.Printf("Summary: %s\n", decision.Summary)
		fmt.Printf("Raw Output:\n%s\n", decision.RawOutput)

		matched := decision.Type == sc.wantType
		fmt.Printf("Matched Expected: %v\n", matched)
		if matched {
			passCount++
		}
	}

	fmt.Printf("\nResult: %d/%d scenarios matched expected memory type.\n", passCount, len(scenarios))
}

type exampleConfig struct {
	apiKey  string
	model   string
	baseURL string
}

func loadConfig() (exampleConfig, bool) {
	fileEnv := map[string]string{}
	for _, candidate := range []string{
		".env",
		filepath.Join("example", "model", "openai", ".env"),
	} {
		vals, err := godotenv.Read(candidate)
		if err == nil {
			for k, v := range vals {
				fileEnv[k] = v
			}
		}
	}

	cfg := exampleConfig{
		apiKey:  firstNonEmpty(strings.TrimSpace(os.Getenv("XGC_API_KEY")), strings.TrimSpace(fileEnv["XGC_API_KEY"])),
		model:   firstNonEmpty(strings.TrimSpace(os.Getenv("XGC_MODEL")), strings.TrimSpace(fileEnv["XGC_MODEL"]), "qwen-plus"),
		baseURL: firstNonEmpty(strings.TrimSpace(os.Getenv("XGC_BASE_URL")), strings.TrimSpace(fileEnv["XGC_BASE_URL"])),
	}
	if cfg.apiKey == "" {
		fmt.Println("missing XGC_API_KEY in environment or .env")
		return exampleConfig{}, false
	}
	return cfg, true
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

type openAIAnalyzerModelAdapter struct {
	inner *openaimodel.OpenAIChatModel
}

func (a *openAIAnalyzerModelAdapter) Generater(ctx context.Context, req *message.Request, opts ...model.BaseOption) (*message.Response, error) {
	return a.inner.Generater(ctx, req, opts...)
}

func (a *openAIAnalyzerModelAdapter) Stream(ctx context.Context, req *message.Request, opts ...model.BaseOption) (<-chan message.StreamEvent, <-chan error) {
	_ = ctx
	_ = req
	_ = opts
	events := make(chan message.StreamEvent)
	errs := make(chan error)
	close(events)
	close(errs)
	return events, errs
}
