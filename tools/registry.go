package tools

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

const (
	DefaultMaxTools = 100
)

// Registry provides unified tool management with a registration mechanism.
type Registry struct {
	mu       sync.RWMutex
	tools    map[string]BaseTool
	maxTools int
}

// NewRegistry creates a new tool registry.
func NewRegistry(maxTools int) *Registry {
	if maxTools <= 0 {
		maxTools = DefaultMaxTools
	}
	return &Registry{
		tools:    make(map[string]BaseTool),
		maxTools: maxTools,
	}
}

// Register registers a tool by its name.
func (r *Registry) Register(t BaseTool) error {
	if t == nil {
		return ErrNilTool
	}
	if r.Count() >= r.maxTools {
		return ErrMaxToolsReached
	}
	name := strings.TrimSpace(t.Name())
	if name == "" {
		return ErrEmptyToolName
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("%w: %s", ErrToolAlreadyRegister, name)
	}
	r.tools[name] = t
	return nil
}

// MustRegister registers a tool and panics on error.
func (r *Registry) MustRegister(t BaseTool) {
	if err := r.Register(t); err != nil {
		panic(err)
	}
}

// Unregister removes a tool by name and reports whether it existed.
func (r *Registry) Unregister(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[name]; !exists {
		return false
	}
	delete(r.tools, name)
	return true
}

// Get returns a tool by name.
// ok is false if the tool does not exist.
func (r *Registry) Get(name string) (BaseTool, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, false
	}

	// set read lock
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// List returns all registered tools sorted by name.
func (r *Registry) List() []BaseTool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]BaseTool, 0, len(r.tools))
	for _, t := range r.tools {
		items = append(items, t)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name() < items[j].Name()
	})
	return items
}

// Count returns the number of registered tools.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// Schemas returns JSON Schemas for all registered tools sorted by name.
// TODO
func (r *Registry) Schemas() []ToolSchema {
	tools := r.List()
	schemas := make([]ToolSchema, 0, len(tools))
	for _, t := range tools {
		schemas = append(schemas, t.Schema())
	}
	return schemas
}

// DefaultRegistry is the global registry for tools.
var DefaultRegistry = NewRegistry(DefaultMaxTools)

// Register registers a tool in the default registry.
func Register(t BaseTool) error { return DefaultRegistry.Register(t) }

// MustRegister registers a tool in the default registry and panics on error.
func MustRegister(t BaseTool) { DefaultRegistry.MustRegister(t) }

// Unregister removes a tool from the default registry.
func Unregister(name string) bool { return DefaultRegistry.Unregister(name) }

// Get returns a tool from the default registry.
func Get(name string) (BaseTool, bool) { return DefaultRegistry.Get(name) }

// List returns all tools from the default registry.
func List() []BaseTool { return DefaultRegistry.List() }

// Schemas returns JSON Schemas for all tools from the default registry.
func Schemas() []ToolSchema { return DefaultRegistry.Schemas() }

// The below are helper functions to convert BaseTool to specific Tool Type.
// You can add more functions here if you have more tool types in future.
// when to use, you can use the Type() function to check the tool type first.
// usage(eg):
// t = Get("my_tool")
// if t.Type() == ToolTypeNonStreaming {
//     tool, err := AsNonStreamingTool(t)
//     ...
// }

// NonStreamingTool converts a BaseTool to a non-streaming Tool.
func AsNonStreamingTool(t BaseTool) (Tool, error) {
	if t.Type() != ToolTypeNonStreaming {
		return nil, fmt.Errorf("tools: tool %s is not a non-streaming tool", t.Name())
	}
	return t.(Tool), nil
}

// StreamingTool converts a BaseTool to a streaming StreamableTool.
func AsStreamingTool(t BaseTool) (StreamingTool, error) {
	if t.Type() != ToolTypeStreaming {
		return nil, fmt.Errorf("tools: tool %s is not a streaming tool", t.Name())
	}
	return t.(StreamingTool), nil
}
