package schedule

import "context"

type EndNode struct{}

func (e *EndNode) Execute(ctx context.Context) error {
	//TODO implement me
	return nil
}
