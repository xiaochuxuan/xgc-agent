package schedule

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestScheduler_Run(t *testing.T) {
	sb := NewScheduler()
	a := sb.AddNode("a", &DemoNode{Name: "a"})
	b := sb.AddNode("b", &DemoNode{Name: "b"})
	c := sb.AddNode("c", &DemoNode{Name: "c"})
	d := sb.AddNode("d", &DemoNode{Name: "d"})
	e := sb.AddNode("e", &DemoNode{Name: "e"})

	sb.SetEntry(a)
	sb.SetExit(e)

	sb.AddEdge(a, b)
	sb.AddEdge(a, c)
	sb.AddEdge(b, d)
	sb.AddEdge(c, e)
	sb.AddEdge(d, e)

	if err := sb.Run(context.Background()); err != nil {
		t.Fatalf("scheduler run failed: %v", err)
	}
}

type DemoNode struct {
	Name string
}

func (d *DemoNode) Execute(ctx context.Context) error {
	fmt.Printf("[%s] start at %d\n", d.Name, time.Now().Second())
	time.Sleep(1 * time.Second)
	fmt.Printf("[%s] end at %d\n", d.Name, time.Now().Second())
	return nil
}
