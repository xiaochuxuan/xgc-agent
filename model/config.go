// config.go: Model provider configuration.
package model

import "time"

// Config selects the provider and basic connection settings.
//
// Provider values:
// - "openai_compat": OpenAI-compatible /v1/chat/completions endpoint
// - "mock": offline deterministic mock model
//
// If Model is empty, the provider default will be used.
type Configs struct {
	Provider string

	// BaseURL is the provider base URL, e.g. https://api.openai.com
	// For OpenAI-compatible providers you can point it to your vendor, e.g. https://api.deepseek.com
	BaseURL string

	// APIKey is used for authenticated providers.
	APIKey string

	Model string

	Timeout time.Duration
}

type Config struct {
	apply func(cfg *Configs)
}

// WithProvider sets the model provider.
func WithProvider(provider string) Config {
	return Config{
		apply: func(cfg *Configs) {
			cfg.Provider = provider
		},
	}
}

// WithBaseURL sets the base URL for the model provider.
func WithBaseURL(url string) Config {
	return Config{
		apply: func(cfg *Configs) {
			cfg.BaseURL = url
		},
	}
}

// WithAPIKey sets the API key for authenticated model providers.
func WithAPIKey(key string) Config {
	return Config{
		apply: func(cfg *Configs) {
			cfg.APIKey = key
		},
	}
}

// WithTimeout sets the request timeout for the model provider.
func WithTimeout(timeout time.Duration) Config {
	return Config{
		apply: func(cfg *Configs) {
			cfg.Timeout = timeout
		},
	}
}

func MakeAllConfigs(opts ...Config) Configs {
	cfg := Configs{}
	for _, opt := range opts {
		if opt.apply != nil {
			opt.apply(&cfg)
		}
	}
	return cfg
}
