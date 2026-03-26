package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"xgc-agent/memory"
	"xgc-agent/message"
	"xgc-agent/model"
)

// PromotionAnalyzer classifies short-term messages and returns structured
// promotion data for episodic or semantic long-term memory.
type PromotionAnalyzer interface {
	Analyze(ctx context.Context, messages []message.Message) (PromotionDecision, error)
}

type PromotionDecision struct {
	Type      memory.MemoryType
	Summary   string
	Episodic  EpisodicPromotionData
	Semantic  SemanticPromotionData
	RawOutput string
}

type EpisodicPromotionData struct {
	EventText    string
	Participants []string
	OccurredAt   string
	Context      map[string]any
	Outcome      string
	Completed    bool
}

type SemanticPromotionData struct {
	Knowledge string
	Entities  []map[string]any
	Relations []map[string]any
}

// LLMPromotionAnalyzer prompts the model to classify and structure memory
// promotion output in JSON.
type LLMPromotionAnalyzer struct {
	model model.BaseModel
}

func NewLLMPromotionAnalyzer(m model.BaseModel) *LLMPromotionAnalyzer {
	return &LLMPromotionAnalyzer{model: m}
}

func (a *LLMPromotionAnalyzer) Analyze(ctx context.Context, messages []message.Message) (PromotionDecision, error) {
	if a == nil || a.model == nil {
		return PromotionDecision{}, nil
	}
	prompt := message.Message{Role: message.RoleSystem, Content: promotionSystemPrompt}
	userMsg := message.Message{Role: message.RoleUser, Content: buildConversationForPromotion(messages)}
	req := &message.Request{Messages: []message.Message{prompt, userMsg}}
	resp, err := a.model.Generater(ctx, req)
	if err != nil {
		return PromotionDecision{}, err
	}
	if len(resp.ResponseChoices) == 0 {
		return PromotionDecision{}, nil
	}
	raw := strings.TrimSpace(resp.ResponseChoices[0].Message.Content)
	decision, err := parsePromotionDecision(raw)
	if err != nil {
		return PromotionDecision{}, err
	}
	decision.RawOutput = raw
	return decision, nil
}

const promotionSystemPrompt = `You are a memory promotion classifier for an agent memory system.

<Role>
You classify and structure memory promotion from conversation snippets.

<Target>
Choose exactly one memory_type: episodic, semantic, none.

<Reference>
- episodic memory: concrete events and experiences with event text, participants, occurred time, context, outcome, completed status.
- semantic memory: abstract knowledge, preferences, rules, domain facts with entities and relations.

<Decision guidance>
- Prefer episodic when the snippet describes a specific event, action, result, or timeline.
- Prefer semantic when the snippet expresses stable knowledge, preference, policy, fact, or recurring rule.
- Use none for greetings, vague chatter, or content with no durable memory value.
- If both appear, choose the dominant signal. Event-centered snippets should be episodic; rule/fact-centered snippets should be semantic.
- Do not infer details that are not in the snippet. Use empty strings, empty arrays, or empty objects when unknown.

<Output requirements>
Return JSON only. No markdown.
Return exactly one JSON object. No prose before or after JSON.
Do not wrap output in code fences.
Always include all fields shown in the schema, even when memory_type is none.
JSON schema:
{
  "memory_type": "episodic|semantic|none",
  "summary": "short summary",
  "episodic": {
    "event_text": "",
    "participants": [""],
    "occurred_at": "RFC3339 or empty",
    "context": {},
    "outcome": "",
    "completed": false
  },
  "semantic": {
    "knowledge": "",
    "entities": [
      {"entity_id": "", "name": "", "type": "", "properties": {}}
    ],
    "relations": [
      {"from": "", "to": "", "type": "", "properties": {}}
    ]
  }
}

<Examples>
Example 1 (episodic)
Input snippet:
user: Yesterday I deployed the billing service with Bob and fixed a timeout in checkout.
assistant: Great, did the deployment succeed?
user: Yes, incident resolved and monitoring is green.

Output:
{
  "memory_type": "episodic",
  "summary": "Deployed billing service with Bob and resolved checkout timeout.",
  "episodic": {
	"event_text": "Deployed billing service and fixed checkout timeout",
	"participants": ["user", "Bob"],
	"occurred_at": "",
	"context": {"service": "billing", "issue": "checkout timeout"},
	"outcome": "Deployment succeeded and incident resolved",
	"completed": true
  },
  "semantic": {
	"knowledge": "",
	"entities": [],
	"relations": []
  }
}

Example 2 (semantic)
Input snippet:
user: I prefer concise answers with bullet points.
assistant: Noted. I will keep responses brief and structured.

Output:
{
  "memory_type": "semantic",
  "summary": "User prefers concise bullet-point responses.",
  "episodic": {
	"event_text": "",
	"participants": [],
	"occurred_at": "",
	"context": {},
	"outcome": "",
	"completed": false
  },
  "semantic": {
	"knowledge": "User prefers concise responses with bullet points",
	"entities": [
	  {"entity_id": "user", "name": "user", "type": "person", "properties": {}}
	],
	"relations": [
	  {"from": "user", "to": "concise_bullet_style", "type": "prefers", "properties": {}}
	]
  }
}

Example 3 (none)
Input snippet:
user: hi
assistant: hello

Output:
{
  "memory_type": "none",
  "summary": "No durable memory to promote.",
  "episodic": {
	"event_text": "",
	"participants": [],
	"occurred_at": "",
	"context": {},
	"outcome": "",
	"completed": false
  },
  "semantic": {
	"knowledge": "",
	"entities": [],
	"relations": []
  }
}`

type promotionPayload struct {
	MemoryType string `json:"memory_type"`
	Summary    string `json:"summary"`
	Episodic   struct {
		EventText    string         `json:"event_text"`
		Participants []string       `json:"participants"`
		OccurredAt   string         `json:"occurred_at"`
		Context      map[string]any `json:"context"`
		Outcome      string         `json:"outcome"`
		Completed    bool           `json:"completed"`
	} `json:"episodic"`
	Semantic struct {
		Knowledge string           `json:"knowledge"`
		Entities  []map[string]any `json:"entities"`
		Relations []map[string]any `json:"relations"`
	} `json:"semantic"`
}

func parsePromotionDecision(raw string) (PromotionDecision, error) {
	if strings.TrimSpace(raw) == "" {
		return PromotionDecision{}, nil
	}
	jsonText, ok := extractJSONObject(raw)
	if !ok {
		return PromotionDecision{}, fmt.Errorf("promotion output is not valid JSON")
	}
	var payload promotionPayload
	if err := json.Unmarshal([]byte(jsonText), &payload); err != nil {
		return PromotionDecision{}, err
	}

	decision := PromotionDecision{Summary: strings.TrimSpace(payload.Summary)}
	switch strings.ToLower(strings.TrimSpace(payload.MemoryType)) {
	case string(memory.MemoryEpisodic):
		decision.Type = memory.MemoryEpisodic
	case string(memory.MemorySemantic):
		decision.Type = memory.MemorySemantic
	default:
		decision.Type = ""
	}
	decision.Episodic = EpisodicPromotionData{
		EventText:    strings.TrimSpace(payload.Episodic.EventText),
		Participants: payload.Episodic.Participants,
		OccurredAt:   strings.TrimSpace(payload.Episodic.OccurredAt),
		Context:      payload.Episodic.Context,
		Outcome:      strings.TrimSpace(payload.Episodic.Outcome),
		Completed:    payload.Episodic.Completed,
	}
	decision.Semantic = SemanticPromotionData{
		Knowledge: strings.TrimSpace(payload.Semantic.Knowledge),
		Entities:  payload.Semantic.Entities,
		Relations: payload.Semantic.Relations,
	}
	return decision, nil
}

func extractJSONObject(raw string) (string, bool) {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return "", false
	}
	return raw[start : end+1], true
}

func buildConversationForPromotion(messages []message.Message) string {
	if len(messages) == 0 {
		return ""
	}
	var b strings.Builder
	for _, msg := range messages {
		role := strings.TrimSpace(string(msg.Role))
		if role == "" {
			role = "user"
		}
		b.WriteString(role)
		b.WriteString(": ")
		b.WriteString(strings.TrimSpace(msg.Content))
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

func parseOrNow(ts string) string {
	if strings.TrimSpace(ts) == "" {
		return time.Now().UTC().Format(time.RFC3339)
	}
	if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
		return parsed.UTC().Format(time.RFC3339)
	}
	if parsed, err := time.Parse("2006-01-02 15:04:05", ts); err == nil {
		return parsed.UTC().Format(time.RFC3339)
	}
	return time.Now().UTC().Format(time.RFC3339)
}
