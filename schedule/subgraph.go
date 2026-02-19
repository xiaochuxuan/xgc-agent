package schedule

import (
	"context"
	"fmt"
)

// SubGraphNode represents a node that contains a subgraph.
// When you want to create a cycle in the graph, you can use a SubGraphNode to execute a subgraph.
type subGraphNode struct {
	// scheduler is the scheduler that executes the subgraph.
	scheduler *Scheduler
	// inputKeys is the keys mapping.
	inputKeys map[string]string //map[subGraphKey]parentGraphKey
	// outputKeys is the keys mapping.
	outputKeys map[string]string //map[parentGraphKey]subGraphKey

	// continueCond is the condition that determines whether to continue executing the subgraph.
	// it is used for cycle graph, when the condition is false, the subgraph will stop executing and return to the parent graph.
	cond func(ctx context.Context, state *GraphState) bool
}

// Execute executes the subgraph until the continue condition is false.
// The state parameter is the parent graph state.
func (s *subGraphNode) Execute(ctx context.Context, state *GraphState) (err error) {
	for s.cond(ctx, state) {
		if err := s.executeOnce(ctx, state); err != nil {
			return err
		}
	}
	return nil
}

func (s *subGraphNode) executeOnce(ctx context.Context, state *GraphState) (err error) {
	// execute the subgraph once
	// execute the subgraph until the continue condition is false
	var ok bool
	input := make(map[string]any)
	for k, v := range s.inputKeys {
		if input[k], ok = state.Context.Get(v); !ok {
			return fmt.Errorf("key %s not found in state", v)
		}
	}

	subGraphContext, err := s.scheduler.Run(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to execute subgraph: %w", err)
	}

	// map the output keys from subgraph context to parent graph context
	for parentKey, subKey := range s.outputKeys {
		if value, ok := subGraphContext.Get(subKey); ok {
			state.Context.Set(parentKey, value)
		}
	}
	return nil
}
