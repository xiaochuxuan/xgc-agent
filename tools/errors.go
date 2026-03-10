package tools

import "errors"

var (
	ErrNilTool             = errors.New("tools: nil tool")
	ErrEmptyToolName       = errors.New("tools: empty tool name")
	ErrToolAlreadyRegister = errors.New("tools: tool already registered")
	ErrToolNotFound        = errors.New("tools: tool not found")
	ErrMaxToolsReached     = errors.New("tools: maximum number of registered tools reached")
)
