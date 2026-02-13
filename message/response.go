package message

import "time"

// Response is a provider-agnostic response.
type Response struct {
	// ID is the unique identifier for the response.
	ID string `json:"id"`
	// Model is the model used for the response.
	Model string `json:"model"`
	// ResponseChoices are the generated response choices
	ResponseChoices []ResponseChoice `json:"choices"`
	// Usage contains token usage information when available.
	Usage *Usage `json:"usage,omitempty"`
	// Error contains error information if the response indicates a failure.
	Error *ResponseError `json:"error,omitempty"`
	// Time is the timestamp of the response.
	Time time.Time `json:"time,omitempty"`
	// Done indicates if the llm flow is complete.
	Done bool `json:"done"`
	// IsPartial indicates if this is a partial response.
	IsPartial bool `json:"is_partial"`
}

// ResponseChoice represents a single response choice.
type ResponseChoice struct {
	// Index is the index of the choice.
	Index int `json:"index"`
	// Message is the generated message for this choice.
	Message Messages `json:"message,omitempty"`
	// FinishReason indicates why the generation finished.
	FinishReason *string `json:"finish_reason,omitempty"`
}

type ResponseError struct {
	Message string  `json:"message"`
	Type    string  `json:"type,omitempty"`
	Param   *string `json:"param,omitempty"`
	Code    *string `json:"code,omitempty"`
}

// ChatUsage contains token usage information when available.
type Usage struct {
	// Number of the prompt tokens used.
	PromptTokens int `json:"prompt_tokens"`
	// Number of the generated completion tokens used.
	CompletionTokens int `json:"completion_tokens"`
	// Total number of tokens used.(= PromptTokens + CompletionTokens)
	TotalTokens int `json:"total_tokens"`
}
