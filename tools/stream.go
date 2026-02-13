package tools

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"xgc-agent/utils"
)

// the streaming tool implementation.
type StreamingTool interface {
	// StreamExecute runs the tool with the given input and returns a stream for iterating output.
	StreamExecute(ctx context.Context, args []byte) (*StreamReader, error)
	BaseTool
}

// StreamableTool is a generic implementation of a streaming tool.
// T is the item type of the stream.
// user should provide a function that takes input I and returns Stream[T].
type StreamableTool[I any, O any] struct {
	name          string
	description   string
	inputSchema   *Schema
	outputSchema  *Schema
	streamExecute func(I) *StreamReader
	unmarshal     func([]byte, any) error
}

// implements BaseTool interface
func (t *StreamableTool[I, O]) Name() string {
	return t.name
}

// implements BaseTool interface
func (t *StreamableTool[I, O]) Description() string {
	return t.description
}

// implements BaseTool interface
func (t *StreamableTool[I, O]) Schema() ToolSchema {
	return ToolSchema{
		Input:  t.inputSchema,
		Output: t.outputSchema,
	}
}

func (t *StreamableTool[I, O]) Type() ToolType {
	return ToolTypeStreaming
}

// NewStreamTool is a generic implementation of a streaming tool.
// it wraps a function with specific input and output types.
// it uses JSON Schema for input and output descriptions.
func NewStreamTool[I any, O any](fn func(I) *StreamReader, opts ...Option) *StreamableTool[I, O] {
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

	iSchema := GenerateJSONSchema(reflect.TypeOf(emptyI))
	oSchema := GenerateJSONSchema(reflect.TypeOf(emptyO))
	return &StreamableTool[I, O]{
		name:          options.name,
		description:   options.description,
		inputSchema:   iSchema,
		outputSchema:  oSchema,
		streamExecute: fn,
		unmarshal:     options.unmarshal,
	}
}

// StreamExecute implements StreamingTool.
// when you new a StreamableTool, you must provide the streamExecute function
// other properties are optional
func (t *StreamableTool[I, O]) StreamExecute(ctx context.Context, args []byte) (*StreamReader, error) {
	var input I

	if err := t.unmarshal(args, &input); err != nil {
		return nil, err
	}

	if t.streamExecute == nil {
		return nil, fmt.Errorf("streamExecute function is nil")
	}

	return t.streamExecute(input), nil
}

type Stream struct {
	Reader *StreamReader
	Writer *StreamWriter
}

// StreamReader is the reader side of a stream.
type StreamReader struct {
	s *stream[StreamChunk]
}

// StreamWriter is the writer side of a stream.
type StreamWriter struct {
	s *stream[StreamChunk]
}

// stream is the internal implementation of a stream.
type stream[T any] struct {
	items    chan streamItem[T]
	closed   chan struct{}
	isClosed bool
}

// streamItem represents an item in the stream.
type streamItem[T any] struct {
	chunk T
	err   error
}

// To specify the chunk structure for streaming data.
type StreamChunk struct {
	Data any `json:"data"`
}

// NewStream creates a new Stream with the given buffer size.
func NewStream(sz int) *Stream {
	s := &stream[StreamChunk]{
		items:  make(chan streamItem[StreamChunk], sz),
		closed: make(chan struct{}),
	}
	return &Stream{
		Reader: &StreamReader{s: s},
		Writer: &StreamWriter{s: s},
	}
}

// The user api for StreamWriter to send an item.
// usage(eg):
//
//	closed := writer.Send(item, nil)
//
//	if closed {
//		// the stream is closed, stop sending
//		break
//	}
//	 writer.Close()
func (w *StreamWriter) Send(item StreamChunk, err error) (closed bool) {
	return w.s.send(item, err)
}

// The user api for StreamWriter to close the sending side of the stream.
// when the sending side is closed, the receiving side will receive io.EOF when all items are read.
// so the user should call Close on the StreamReader to release resources.
func (w *StreamWriter) Close() {
	w.s.closeSend()
}

// The user api for StreamReader to receive an item.
// usage(eg):
//
//		 for {
//			item, err := reader.Recv()
//			if err == io.EOF {
//			    // the stream is closed, stop receiving
//			    break
//			}
//		 }
//	 	 reader.Close()
func (r *StreamReader) Recv() (StreamChunk, error) {
	return r.s.recv()
}

// The user api for StreamReader to close the receiving side of the stream.
// when the receiving side is closed, the sending side will be notified to stop sending.
// so the user should call Close on the StreamWriter to release resources.
func (r *StreamReader) Close() {
	r.s.closeRecv()
}

func (s *stream[T]) send(item T, err error) (closed bool) {
	// if the stream is closed, return immediately
	select {
	case <-s.closed:
		return true
	default:
	}

	i := streamItem[T]{item, err}

	select {
	case <-s.closed:
		return true
	case s.items <- i:
		return false
	}
}

// the internal recv method
func (s *stream[T]) recv() (T, error) {
	it, ok := <-s.items
	if !ok {
		it.err = io.EOF
	}
	return it.chunk, it.err
}

func (s *stream[T]) closeSend() {
	close(s.items)
}

func (s *stream[T]) closeRecv() {
	close(s.closed)
}
