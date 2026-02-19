package schedule

import (
	"context"
	"fmt"
	"math"
	"sync/atomic"
	"xgc-agent/utils"
)

const (
	StartNodeFlag = "__start__"
	EndNodeFlag   = "__end__"

	startNodeID = int32(0)
	endNodeID   = math.MaxInt32
)

const (
	NodePending int32 = iota
	NodeRunning
	NodeSkipped // conditional node can be skipped if condition not met
	NodeFinished
	NodeFailed
)

type Scheduler struct {
	// Graph represents the execution graph where keys are node ids and values are pointers to Node structs.
	Graph map[int32]*Node
	// Entry is the starting node in the graph.
	Entry *Node
	// idGen is an atomic counter used to generate unique node IDs.
	idGen atomic.Int32
	// GraphState holds the state of the graph execution, including channels for signaling completion and errors.
	GraphState *GraphState
}

type GraphState struct {
	Context  *graphContext
	done     chan *Node
	finished chan struct{}
	err      chan error
}

func newGraphState() *GraphState {
	return &GraphState{
		Context: &graphContext{
			data:   make(map[string]any),
			queues: make(map[string]*MessageQueue),
		},
		done:     make(chan *Node, 100),
		finished: make(chan struct{}, 1),
		err:      make(chan error, 1),
	}
}

func NewScheduler() *Scheduler {
	s := &Scheduler{
		Graph:      make(map[int32]*Node),
		GraphState: newGraphState(),
	}
	s.Entry, _ = s.AddNode(StartNodeFlag, NodeTypeNormal, &StartNode{})
	s.Graph[startNodeID] = s.Entry
	endNode, _ := s.AddNode(EndNodeFlag, NodeTypeNormal, &EndNode{})
	endNode.id = endNodeID
	s.Graph[endNodeID] = endNode
	return s
}

func (s *Scheduler) resetForRun() {
	s.GraphState = newGraphState()

	if s.Entry != nil {
		s.Entry.setState(NodePending)
	}
	for _, node := range s.Graph {
		if node != nil {
			node.setState(NodePending)
		}
	}
}

func (s *Scheduler) AddNode(name string, type_ NodeType, inode INode) (*Node, error) {
	if inode == nil {
		return nil, ErrInodeRequired
	}
	if !utils.SliceContains(NodeTypeSlice, type_) {
		return nil, fmt.Errorf("invalid node type: %d", type_)
	}

	var id int32
	if name == StartNodeFlag || name == EndNodeFlag {
		id = 0
	} else {
		id = s.generateNodeID()
	}
	node := &Node{
		Name: name,
		Type: type_,
		Fn:   inode,
		id:   id,
	}
	s.Graph[id] = node
	return node, nil
}

func (s *Scheduler) AddConditionalNode(name string, cond func(ctx context.Context, state *GraphState) ([]string, error)) (*Node, error) {
	id := s.generateNodeID()
	return &Node{
		id:   id,
		Type: NodeTypeConditional,
		Name: name,
		Fn: &conditionalNode{
			Id:   id,
			Cond: cond,
		},
	}, nil
}

// AddSubGraphNode adds a sub-graph node to the scheduler.
// The sub-graph will be executed as a part of the main graph.
// The inputKeys are the keys mappings from the main graph context to the sub-graph context. The sub-graph can access the input data through the inputKeys.
// The fn is a function that will be executed before the sub-graph
// to determine whether to execute the sub-graph or not.
// If fn returns false, the sub-graph will be skipped.
func (s *Scheduler) AddSubGraphNode(name string, subScheduler *Scheduler,
	inputKeys map[string]string, fn func(ctx context.Context, state *GraphState) bool) (*Node, error) {
	id := s.generateNodeID()
	return &Node{
		id:   id,
		Type: NodeTypeSubGraph,
		Name: name,
		Fn: &subGraphNode{
			inputKeys: inputKeys,
			scheduler: subScheduler,
			cond:      fn,
		},
	}, nil
}

func (s *Scheduler) AddEdge(from, to *Node) error {
	if from == nil || to == nil {
		return ErrNodeRequired
	}
	if from.Type == NodeTypeConditional {
		return ErrFromNodeTypeInvalid
	}
	from.Next = append(from.Next, to)
	to.Prev = append(to.Prev, from)
	return nil
}

func (s *Scheduler) AddConditionalEdge(from, to *Node, tag string) error {
	if from == nil || to == nil {
		return ErrNodeRequired
	}
	if from.Type != NodeTypeConditional {
		return ErrFromNodeTypeInvalid
	}
	// insert a virtual node between from and to
	// the virtual node is used to store the condition tag and determine whether to execute to node
	virtualNode := &Node{
		Name: tag,
		Type: _NodeTypeConditionalVirtual,
		Fn:   &conditionalVirtualNode{},
		id:   s.generateNodeID(),
	}
	s.Graph[virtualNode.id] = virtualNode
	from.Next = append(from.Next, virtualNode)
	virtualNode.Prev = append(virtualNode.Prev, from)
	virtualNode.Next = append(virtualNode.Next, to)
	to.Prev = append(to.Prev, virtualNode)
	return nil
}

func (s *Scheduler) SetEntry(node *Node) {
	s.Entry.Next = append(s.Entry.Next, node)
}

func (s *Scheduler) SetExit(node *Node) {
	endNode := s.Graph[endNodeID]
	node.Next = append(node.Next, endNode)
	endNode.Prev = append(endNode.Prev, node)
}

func (s *Scheduler) BuildGraph() {}

// Run is the main entry point for executing the graph.
// It starts a goroutine to listen for completed nodes and schedule their next nodes accordingly. The main goroutine executes the entry node and waits for the graph execution to finish or fail.
// Only setState in the main goroutine to avoid race condition
func (s *Scheduler) Run(ctx context.Context, input map[string]any) (GraphContext, error) {
	s.resetForRun()

	// set the input data to the graph context
	for k, v := range input {
		s.GraphState.Context.Set(k, v)
	}

	// the main control procession
	go func() {
		for node := range s.GraphState.done {
			if node.id == endNodeID {
				s.GraphState.finished <- struct{}{}
				return
			}
			// when a node is finished, schedule its next nodes to run
			for _, next := range node.Next {
				if err := s.scheduleNode(ctx, next); err != nil {
					s.GraphState.err <- err
					return
				}
			}
		}
	}()

	// start from the entry node
	go func() {
		s.Entry.Execute(ctx, s.GraphState)
	}()

	// wait for the graph execution to finish or fail
	select {
	case <-s.GraphState.finished:
		fmt.Println("graph execution finished")
		return s.GraphState.Context, nil
	case err := <-s.GraphState.err:
		fmt.Printf("graph execution failed: %v\n", err)
		return s.GraphState.Context, err
	}
}

func (s *Scheduler) scheduleNode(ctx context.Context, node *Node) error {
	countSkipped := 0
	// check if all prev nodes are finished & count nodes which is skipped
	for _, prev := range node.Prev {
		switch prev.State {
		case NodePending, NodeRunning:
			// wait for pending or running nodes to finish
			return nil
		case NodeFailed:
			// if any of the prev node failed, current node should be marked as failed
			// and return error immediately
			return fmt.Errorf("node %s failed", prev.Name)
		case NodeSkipped:
			// set skipped count
			countSkipped++
		case NodeFinished:
			// do nothing, just continue to check other prev nodes
		default:
			return ErrUnknowNodeState
		}
	}
	// if all prev nodes are skipped, skip current node as well
	// countSkipped > 0 is to make sure the start node will not be skipped
	if countSkipped > 0 && countSkipped == len(node.Prev) {
		node.setState(NodeSkipped)
		s.GraphState.done <- node
		return nil
	}

	if !atomic.CompareAndSwapInt32(&node.State, NodePending, NodeRunning) {
		return nil
	}
	go func() {
		node.Execute(ctx, s.GraphState)
	}()

	return nil
}

func (s *Scheduler) generateNodeID() int32 {
	return s.idGen.Add(1)
}
