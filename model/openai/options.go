package openai

import (
	"net/http"

	openaiopt "github.com/openai/openai-go/v3/option"
)

type options struct {
	// authentication key
	// Required.
	APIKey string `json:"api_key"`

	// optional base URL for OpenAI-compatible providers
	BaseURL string `json:"base_url"`

	// used to send HTTP requests
	// Optional. Default: http.DefaultClient
	HTTPClient *http.Client `json:"-"`

	// Options for the OpenAI client.
	OpenAIOptions []openaiopt.RequestOption

	// specified model to use
	Model string `json:"model"`
}

// A function that configures options for the OpenAI chat model.
type Option func(*options)

// WithAPIKey sets the API key for authentication.
func WithAPIKey(key string) Option {
	return func(o *options) {
		o.APIKey = key
	}
}

// WithBaseURL sets the base URL for OpenAI-compatible providers.
func WithBaseURL(url string) Option {
	return func(o *options) {
		o.BaseURL = url
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(o *options) {
		o.HTTPClient = client
	}
}

// WithOpenAIOptions sets additional OpenAI client options.
// Example:
//
//	opt := WithOpenAIOptions(
//	    openaiopt.WithHTTPHeader("X-Custom-Header", "value"),
//	    openaiopt.WithRequestTimeout(30*time.Second),
//	)
func WithOpenAIOptions(opts ...openaiopt.RequestOption) Option {
	return func(o *options) {
		o.OpenAIOptions = append(o.OpenAIOptions, opts...)
	}
}

// WithModel sets the model name to use.
func WithModel(model string) Option {
	return func(o *options) {
		o.Model = model
	}
}

// applyOptions applies the given options to the default options.
func applyOptions(opts ...Option) *options {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}
