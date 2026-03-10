package agent

import (
	"time"
	"xgc-agent/model"
	"xgc-agent/sandbox"
	"xgc-agent/tools"
)

// Config holds the configuration for creating an Agent.
type Config struct {
	Name          string
	Model         model.BaseModel
	Tools         *tools.Registry
	Sandbox       sandbox.Sandbox
	MaxIterations int
	Timeout       time.Duration
	Locale        string // "zh" or "en"
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig(m model.BaseModel) Config {
	return Config{
		Name:          "xgc-agent",
		Model:         m,
		Tools:         tools.NewRegistry(tools.DefaultMaxTools),
		MaxIterations: 10,
		Locale:        "zh",
	}
}
