package schedule

import (
	"context"
	"fmt"
)

type conditionalNode struct {
	Id   int32
	Cond func(ctx context.Context, state *GraphState) ([]string, error)
}

func (c *conditionalNode) Execute(ctx context.Context, state *GraphState) (err error) {
	tags, err := c.Cond(ctx, state)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("__condition_%d_result__", c.Id)
	state.Context.Set(key, tags)
	return nil
}

type conditionalVirtualNode struct {
}

func (c *conditionalVirtualNode) Execute(ctx context.Context, state *GraphState) (err error) {
	//TODO implement me
	return nil
}
