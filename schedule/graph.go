package schedule

import (
	"context"
	"fmt"
	"math"
	"sync/atomic"
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

type NodeType int32

const (
	NodeTypeNormal NodeType = iota
	NodeTypeConditional
)

type Scheduler struct {
	// Graph represents the execution graph where keys are node ids and values are pointers to Node structs.
	Graph map[int32]*Node
	// Entry is the starting node in the graph.
	Entry *Node
	// the
	idGen atomic.Int32

	GraphState *GraphState
}

type GraphState struct {
	Metadata map[string]any
	done     chan struct{}
	err      chan error
}

//type NodeFunc func(ctx context.Context, id int32) (interface{}, error)

type INode interface {
	Execute(ctx context.Context) (err error)
}

type Node struct {
	Name  string
	Type  NodeType
	Fn    INode
	State int32
	Prev  []*Node
	Next  []*Node // Next nodes to execute after this node

	id int32
}

func (n *Node) Execute(ctx context.Context, state *GraphState) error {
	if n.Type == NodeTypeConditional {
		return n.executeConditional(ctx, state)
	}

	countSkipped := 0
	// check if all prev nodes are finished & count nodes which is skipped
	for _, prev := range n.Prev {
		switch prev.State {
		case NodePending:
			// wait for pending nodes to finish
			return nil
		case NodeFailed:
			err := fmt.Errorf("node %s failed", prev.Name)
			state.err <- err
			return err
		case NodeRunning:
			return nil
		case NodeSkipped:
			countSkipped++
		case NodeFinished:
		default:
			state.err <- ErrUnknowNodeState
			return ErrUnknowNodeState
		}
	}
	// if all prev nodes are skipped, skip current node as well
	// countSkipped > 0 is to make sure the start node will not be skipped
	if countSkipped > 0 && countSkipped == len(n.Prev) {
		n.setState(NodeSkipped)
		goto startNext
	}

	// execute current node
	if atomic.CompareAndSwapInt32(&n.State, NodePending, NodeRunning) {
		err := n.Fn.Execute(ctx)
		if err != nil {
			n.setState(NodeFailed)
			return err
		}
		n.setState(NodeFinished)
	}

startNext:
	// execute next nodes in parallel
	for _, next := range n.Next {
		go func() { next.Execute(ctx, state) }()
	}
	if n.Name == EndNodeFlag && n.id == endNodeID {
		state.done <- struct{}{}
	}
	return nil
}

func (n *Node) executeConditional(ctx context.Context, state *GraphState) error { return nil }

func (n *Node) setState(state int32) {
	atomic.StoreInt32(&n.State, state)
}

func NewScheduler() *Scheduler {
	s := &Scheduler{
		Graph: make(map[int32]*Node),
		GraphState: &GraphState{
			done: make(chan struct{}, 1),
			err:  make(chan error, 1),
		},
	}
	s.Entry = s.AddNode(StartNodeFlag, &StartNode{})
	s.Graph[startNodeID] = s.Entry
	endNode := s.AddNode(EndNodeFlag, &EndNode{})
	endNode.id = endNodeID
	s.Graph[endNodeID] = endNode
	return s
}

func (s *Scheduler) AddNode(name string, inode INode) *Node {
	return &Node{
		Name: name,
		Fn:   inode,
		id:   s.generateNodeID(),
	}
}

// TODO: consider conditional node
func (s *Scheduler) SetEntry(node *Node) {
	s.Entry.Next = append(s.Entry.Next, node)
}

func (s *Scheduler) SetExit(node *Node) {
	endNode := s.Graph[endNodeID]
	node.Next = append(node.Next, endNode)
	endNode.Prev = append(endNode.Prev, node)
}

//func (s *Scheduler) AddConditionalNode(name string, fn NodeFunc) *ConditionalNode { return nil }

func (s *Scheduler) AddEdge(from, to *Node) *Scheduler {
	from.Next = append(from.Next, to)
	to.Prev = append(to.Prev, from)
	return s
}

func (s *Scheduler) BuildGraph() {}

func (s *Scheduler) Run(ctx context.Context) error {
	s.Entry.Execute(ctx, s.GraphState)
	select {
	case <-s.GraphState.done:
		return nil
	case err := <-s.GraphState.err:
		return err
	}
}

func (s *Scheduler) generateNodeID() int32 {
	return s.idGen.Add(1)
}
