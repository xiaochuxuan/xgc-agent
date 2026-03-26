package manager

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"xgc-agent/memory"
	"xgc-agent/message"
	"xgc-agent/model"
	openaimodel "xgc-agent/model/openai"

	"github.com/joho/godotenv"
)

func TestParsePromotionDecision_ValidJSON(t *testing.T) {
	raw := `prefix text {"memory_type":"semantic","summary":"User prefers concise answers","episodic":{"event_text":"","participants":[],"occurred_at":"","context":{},"outcome":"","completed":false},"semantic":{"knowledge":"User prefers concise answers","entities":[],"relations":[]}} suffix text`
	decision, err := parsePromotionDecision(raw)
	if err != nil {
		t.Fatalf("parsePromotionDecision() error = %v", err)
	}
	if decision.Type != memory.MemorySemantic {
		t.Fatalf("decision.Type = %q, want %q", decision.Type, memory.MemorySemantic)
	}
	if decision.Summary == "" {
		t.Fatal("decision.Summary is empty")
	}
}

func TestLLMPromotionAnalyzer_Analyze_FromEnv(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live promotion test in short mode")
	}
	// if strings.TrimSpace(os.Getenv("RUN_PROMOTION_LIVE_TEST")) != "1" {
	// 	t.Skip("set RUN_PROMOTION_LIVE_TEST=1 to run live promotion analyzer test")
	// }

	cfg, ok := loadPromotionTestConfigFromDotEnv(t)
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
		t.Fatalf("NewOpenAIChatModel() error = %v", err)
	}

	analyzer := NewLLMPromotionAnalyzer(&openAIAnalyzerModelAdapter{inner: chatModel})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	messages := []message.Message{
		{Role: message.RoleUser, Content: "For future answers, please use concise bullet points in Chinese."},
		{Role: message.RoleAssistant, Content: "Got it, I will answer in concise Chinese bullet points."},
	}

	decision, err := analyzer.Analyze(ctx, messages)
	t.Logf("decision: %+v", decision)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if decision.Type != memory.MemorySemantic {
		t.Fatalf("decision.Type = %q, want %q", decision.Type, memory.MemorySemantic)
	}
	if strings.TrimSpace(decision.Summary) == "" {
		t.Fatal("decision.Summary is empty")
	}
	if strings.TrimSpace(decision.RawOutput) == "" {
		t.Fatal("decision.RawOutput is empty")
	}
}

type promotionTestConfig struct {
	apiKey  string
	model   string
	baseURL string
}

func loadPromotionTestConfigFromDotEnv(t *testing.T) (promotionTestConfig, bool) {
	t.Helper()
	fileEnv := map[string]string{}
	for _, candidate := range []string{".env", filepath.Join("..", "..", ".env")} {
		vals, err := godotenv.Read(candidate)
		if err == nil {
			for k, v := range vals {
				fileEnv[k] = v
			}
		}
	}

	cfg := promotionTestConfig{
		apiKey:  firstNonEmpty(strings.TrimSpace(os.Getenv("XGC_API_KEY")), strings.TrimSpace(fileEnv["XGC_API_KEY"])),
		model:   firstNonEmpty(strings.TrimSpace(os.Getenv("XGC_MODEL")), strings.TrimSpace(fileEnv["XGC_MODEL"]), "qwen-plus"),
		baseURL: firstNonEmpty(strings.TrimSpace(os.Getenv("XGC_BASE_URL")), strings.TrimSpace(fileEnv["XGC_BASE_URL"])),
	}
	if cfg.apiKey == "" {
		t.Skip("missing XGC_API_KEY in process environment or .env file")
		return promotionTestConfig{}, false
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

// openAIAnalyzerModelAdapter bridges Stream signature differences for tests.
// LLMPromotionAnalyzer only calls Generater, so Stream is a no-op.
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
