package session

import (
	"strings"
	"sync"
	"xgc-agent/message"
)

// Buffer manages a sliding window of messages within a token budget.
// It is actually a message queue (FIFO + token trim)
type Buffer struct {
	mu        sync.RWMutex
	messages  []message.Message
	maxTokens int
	tokenizer func(string) int
}

// NewBuffer creates a new BufferMemory with the specified token limit and tokenizer function.
func NewBuffer(maxTokens int, tokenizer func(string) int) *Buffer {
	if tokenizer == nil {
		tokenizer = defaultTokenizer
	}
	return &Buffer{maxTokens: maxTokens, tokenizer: tokenizer}
}

// Append adds new messages to the buffer and trims old messages if the token limit is exceeded.
func (b *Buffer) Append(msg message.Message) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.messages = append(b.messages, msg)
	b.trim()
}

// Messages returns a copy of the current messages in the buffer.
func (b *Buffer) Messages() []message.Message {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]message.Message, len(b.messages))
	copy(out, b.messages)
	return out
}

// TokenCount returns the total token count of all messages in the buffer.
func (b *Buffer) TokenCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	total := 0
	for _, m := range b.messages {
		total += b.tokenizer(m.Content)
	}
	return total
}

// Clear removes all messages from the buffer.
func (b *Buffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.messages = nil
}

// trim removes oldest messages until the total token count is within the limit.
func (b *Buffer) trim() {
	for b.tokenCount() > b.maxTokens && len(b.messages) > 1 {
		b.messages = b.messages[1:]
	}
}

func (b *Buffer) tokenCount() int {
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

// MaxTokens returns the configured token limit.
func (b *Buffer) MaxTokens() int {
	return b.maxTokens
}
