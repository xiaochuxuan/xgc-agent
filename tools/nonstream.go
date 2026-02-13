package tools

import (
	"context"
	"reflect"
	"xgc-agent/utils"
)

// the non-streaming tool implementation.
type Tool interface {
	// Execute runs the tool with the given input and returns the output or an error.
	Execute(ctx context.Context, args []byte) (any, error)
	BaseTool
}

// CallableTool is a generic implementation of a non-streaming tool.
// it wraps a function with specific input and output types.
// it uses JSON Schema for input and output descriptions.
type CallableTool[I any, O any] struct {
	name         string
	description  string
	inputSchema  *Schema
	outputSchema *Schema
	execute      func(context.Context, I) (O, error)
	unmarshal    func([]byte, any) error
}

// imolements BaseTool interface
func (t *CallableTool[I, O]) Name() string {
	return t.name
}

// implements BaseTool interface
func (t *CallableTool[I, O]) Description() string {
	return t.description
}

// implements BaseTool interface
func (t *CallableTool[I, O]) Schema() ToolSchema {
	return ToolSchema{
		Input:  t.inputSchema,
		Output: t.outputSchema,
	}
}

// implements BaseTool interface
func (t *CallableTool[I, O]) Type() ToolType {
	return ToolTypeNonStreaming
}

// implements Tool interface
// when you new a CallableTool, you must provide the execute function
// other properties are optional
func NewTool[I any, O any](fn func(context.Context, I) (O, error), opts ...Option) *CallableTool[I, O] {
	functionName, err := utils.FunctionName(fn)

	// set default options
	options := &toolOptions{
		// use the function name as the tool name by default
		name: func() string {
			if err != nil {
				return "unknown"
			}
			return utils.ShortFunctionName(functionName)
		}(),
		// use the default unmarshal function
		unmarshal: Unmarshal,
	}

	// apply user-defined options
	for _, opt := range opts {
		opt(options)
	}

	var (
		emptyI I
		emptyO O
	)

	inputSchema := GenerateJSONSchema(reflect.TypeOf(emptyI))
	outputSchema := GenerateJSONSchema(reflect.TypeOf(emptyO))

	return &CallableTool[I, O]{
		name:         options.name,
		description:  options.description,
		inputSchema:  inputSchema,
		outputSchema: outputSchema,
		execute:      fn,
		unmarshal:    options.unmarshal,
	}
}

// implements Tool interface
func (t *CallableTool[I, O]) Execute(ctx context.Context, args []byte) (any, error) {
	var input I
	if err := t.unmarshal(args, &input); err != nil {
		return nil, err
	}
	return t.execute(ctx, input)
}
