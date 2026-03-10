package inmem

import "xgc-agent/tools"

type storeOptions struct {
	// the max memory stores per user
	memoryLimit int
	// the registered tools to be added to the manager's tool registry
	toolRegistry *tools.Registry
	// the enabled tools
	enableTools map[string]bool
}

type StoreOptions func(*storeOptions)

// WithMemoryLimit sets the maximum number of memory stores per user.
func WithMemoryLimit(limit int) StoreOptions {
	return func(o *storeOptions) {
		o.memoryLimit = limit
	}
}

func WithToolRegistry(reg *tools.Registry) StoreOptions {
	return func(o *storeOptions) {
		o.toolRegistry = reg
	}
}

// WithEnabledTools sets the enabled tools in the manager.
func WithEnabledTools(tools map[string]bool) StoreOptions {
	return func(o *storeOptions) {
		o.enableTools = tools
	}
}
