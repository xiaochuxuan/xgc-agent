package schedule

import "context"

type EndNode struct{}

func (e *EndNode) Execute(ctx context.Context, state *GraphState) (err error) {
	//TODO implement me
	return nil
}
