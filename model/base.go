// base.go: Core model interfaces used by providers.
package model

import (
	"context"
	"xgc-agent/message"
)

// define basic model interface
type BaseModel interface {
	// Generater performs a single non-streaming generation.
	Generater(ctx context.Context, req *message.Request, opts ...BaseOption) (*message.Response, error)

	// Stream performs a streaming generation.
	// The returned event channel is closed when the stream ends.
	// If an error occurs mid-stream, it is sent on the error channel.
	Stream(ctx context.Context, req *message.Request, opts ...BaseOption) (<-chan *message.Response, <-chan error)
}

type ChatModel interface {
	BaseModel
}
