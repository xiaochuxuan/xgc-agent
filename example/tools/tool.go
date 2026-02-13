package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"xgc-agent/tools"
)

type addInput struct {
	A int `json:"a"`
	B int `json:"b"`
}

type addOutput struct {
	Sum int `json:"sum"`
}

type countInput struct {
	N int `json:"n"`
}

func main() {
	ctx := context.Background()

	// ---------------------------
	// Non-streaming tool example
	// ---------------------------
	adder := tools.NewTool(func(ctx context.Context, in addInput) (addOutput, error) {
		_ = ctx
		return addOutput{Sum: in.A + in.B}, nil
	}, tools.WithName("adder"), tools.WithDescription("add two integers"))

	result, err := adder.Execute(ctx, []byte(`{"a":2,"b":3}`))
	if err != nil {
		panic(err)
	}
	fmt.Printf("[non-stream] adder => %+v\n", result)

	// ------------------------
	// Streaming tool example
	// ------------------------
	counter := tools.NewStreamTool[countInput, tools.StreamChunk](func(in countInput) *tools.StreamReader {
		st := tools.NewStream(8)
		go func() {
			defer st.Writer.Close()
			for i := 1; i <= in.N; i++ {
				item := tools.StreamChunk{Data: i}
				if closed := st.Writer.Send(item, nil); closed {
					return
				}
				time.Sleep(20 * time.Millisecond)
			}
		}()
		return st.Reader
	}, tools.WithName("counter"), tools.WithDescription("emit numbers 1..N"))

	reader, err := counter.StreamExecute(ctx, []byte(`{"n":5}`))
	if err != nil {
		panic(err)
	}
	defer reader.Close()

	fmt.Print("[stream] counter =>")
	for {
		chunk, err := reader.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
		v, ok := chunk.Data.(int)
		if !ok {
			panic(fmt.Sprintf("unexpected chunk type: %T", chunk.Data))
		}
		fmt.Printf(" %d", v)
	}
	fmt.Println()
}
