package message

import "xgc-agent/tools"

// ChatRequest is a provider-agnostic request.
type Request struct {
	Messages []Messages `json:"messages"`
	GenerationConfig
	// Tools lists available tools for the model to call.
	Tools map[string]tools.BaseTool `json:"tools,omitempty"`
}

// GenerationConfig contains configuration for text generation.
type GenerationConfig struct {
	// maximum tokens for response generation
	// Optional. Default: model's maximum tokens
	MaxTokens *int `json:"max_tokens,omitempty"`
	// temperature for response randomness
	// Optional. Default: 1.0. Range: 0.0 to 2.0
	Temperature *float64 `json:"temperature,omitempty"`
	// top_p for nucleus sampling
	// Optional. Default: 1.0. Range: 0.0 to 1.0
	TopP *float64 `json:"top_p,omitempty"`
	// stop sequences to end generation
	// Optional.
	Stop []string `json:"stop,omitempty"`
	// // PresencePenalty penalizes new tokens based on their existing frequency.
	// PresencePenalty *float64 `json:"presence_penalty,omitempty"`
	// // FrequencyPenalty penalizes new tokens based on their frequency in the text so far.
	// FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`
	// // ReasoningEffort limits the reasoning effort for reasoning models.
	// // Supported values: "low", "medium", "high".
	// // Only effective for OpenAI o-series models.
	// ReasoningEffort *string `json:"reasoning_effort,omitempty"`
	// // ThinkingEnabled enables thinking mode for Claude and Gemini models via OpenAI API.
	// ThinkingEnabled *bool `json:"thinking_enabled,omitempty"`
	// // ThinkingTokens controls the length of thinking for Claude and Gemini models via OpenAI API.
	// ThinkingTokens *int `json:"thinking_tokens,omitempty"`
}
