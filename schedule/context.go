package schedule

import "sync"

const (
	DefaultChanMaxBuffer      = 100
	DefaultChanMaxSubscribers = 100
)

type GraphContext interface {
	Get(key string) (value any, exists bool)
	Set(key string, value any)

	// Register allows a node to register a channel for a specific key.
	// it also starts a goroutine to listen to the producer channel and broadcast messages to all consumers.
	Register(key string, ch chan any)
	// Subscribe allows a node to subscribe to a specific key and receive messages sent to that key.
	// it also sends historical messages to the new consumer channel during the subscription process, so that producer won't miss any messages sent to the channel during the subscription.
	Subscribe(key string) <-chan any
}

type graphContext struct {
	mu   sync.RWMutex
	data map[string]any
	// queues serves as a message bus for nodes to communicate with each other streamingly.
	queues map[string]*MessageQueue
}

type MessageQueue struct {
	mu        sync.RWMutex
	Producer  chan any
	Consumers []chan any

	// history can be used to store historical messages for a specific key
	// it is used for consumers that subscribe after the producer has sent messages
	// so they can receive historical messages.
	history []any
}

type ContextChunk struct {
	Key   string
	Value any
}

func (c *graphContext) Get(key string) (value any, exists bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, exists = c.data[key]
	return value, exists
}

func (c *graphContext) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.data == nil {
		c.data = make(map[string]any)
	}
	c.data[key] = value
}

func (c *graphContext) Register(key string, ch chan any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.queues == nil {
		c.queues = make(map[string]*MessageQueue)
	}
	var queue *MessageQueue
	if _, exists := c.queues[key]; !exists {
		queue = &MessageQueue{
			Producer:  ch,
			Consumers: []chan any{},
		}
		c.queues[key] = queue
	} else {
		return
	}

	go func() {
		// listen to the producer channel and broadcast messages to all consumers
		for msg := range ch {
			queue.history = append(queue.history, msg)
			queue.mu.RLock()
			for _, consumer := range queue.Consumers {
				consumer <- msg
			}
			queue.mu.RUnlock()
		}

		// recycle the queue after producer channel is closed
		for _, consumer := range queue.Consumers {
			close(consumer)
		}
	}()
}

func (c *graphContext) Subscribe(key string) <-chan any {
	if c.queues == nil || c.queues[key] == nil || c.queues[key].Producer == nil {
		return nil
	}

	queue := c.queues[key]
	// lock to finish the subscription process
	// including adding the new consumer channel to the queue and sending historical messages
	// so that producer won't miss any messages sent to the channel during the subscription.
	queue.mu.Lock()
	defer queue.mu.Unlock()
	ch := make(chan any, DefaultChanMaxBuffer)
	if queue.Consumers == nil {
		queue.Consumers = make([]chan any, DefaultChanMaxSubscribers)
	}
	queue.Consumers = append(queue.Consumers, ch)

	// send historical messages to the new consumer
	for _, msg := range queue.history {
		ch <- msg
	}

	return ch
}

/*

master d,e,f

sub a,b,c

a:f, b:d, c:0.7
a:state["f"]

*/
