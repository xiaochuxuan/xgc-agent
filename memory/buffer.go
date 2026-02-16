package memory

import (
	"strings"
	"sync"
	"xgc-agent/v2/message"
)

// BufferMemory manages a sliding window of messages within a token budget.
// It is actually a message queue (FIFO + token trim)
type BufferMemory struct {
	mu        sync.RWMutex
	messages  []message.Messages
	maxTokens int
	tokenizer func(string) int
}

// NewBufferMemory creates a new BufferMemory with the specified token limit and tokenizer function.
func NewBufferMemory(maxTokens int, tokenizer func(string) int) *BufferMemory {
	if tokenizer == nil {
		tokenizer = defaultTokenizer
	}
	return &BufferMemory{maxTokens: maxTokens, tokenizer: tokenizer}
}

// Append adds new messages to the buffer and trims old messages if the token limit is exceeded.
func (b *BufferMemory) Append(msg message.Messages) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.messages = append(b.messages, msg)
	b.trim()
}

// Messages returns a copy of the current messages in the buffer.
func (b *BufferMemory) Messages() []message.Messages {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]message.Messages, len(b.messages))
	copy(out, b.messages)
	return out
}

// TokenCount returns the total token count of all messages in the buffer.
func (b *BufferMemory) TokenCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	total := 0
	for _, m := range b.messages {
		total += b.tokenizer(m.Content)
	}
	return total
}

// Clear removes all messages from the buffer.
func (b *BufferMemory) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.messages = nil
}

// trim removes oldest messages until the total token count is within the limit.
func (b *BufferMemory) trim() {
	for b.tokenCount() > b.maxTokens && len(b.messages) > 1 {
		b.messages = b.messages[1:]
	}
}

func (b *BufferMemory) tokenCount() int {
	total := 0
	for _, m := range b.messages {
		total += b.tokenizer(m.Content)
	}
	return total
}

// defaultTokenizer estimates tokens by splitting on whitespace (rough approximation).
func defaultTokenizer(s string) int {
	if s == "" {
		return 0
	}
	return len(strings.Fields(s)) + len(s)/4
}
