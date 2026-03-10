package prompt

import "strings"

// PromptBuilder provides a chainable API for constructing system prompts.
type PromptBuilder struct {
	sections []string
}

func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{}
}

// System adds a system-level instruction section.
func (b *PromptBuilder) System(s string) *PromptBuilder {
	if s != "" {
		b.sections = append(b.sections, s)
	}
	return b
}

// ToolSpec describes a tool for prompt rendering.
type ToolSpecItem struct {
	Name   string
	Desc   string
	Schema string
}

// Tools adds a tool description section.
func (b *PromptBuilder) Tools(tools []ToolSpecItem) *PromptBuilder {
	if len(tools) == 0 {
		return b
	}
	var sb strings.Builder
	sb.WriteString("【可用工具】\n")
	for _, t := range tools {
		sb.WriteString("- ")
		sb.WriteString(t.Name)
		sb.WriteString("：")
		sb.WriteString(t.Desc)
		if t.Schema != "" {
			sb.WriteString("\n  参数：")
			sb.WriteString(t.Schema)
		}
		sb.WriteString("\n")
	}
	b.sections = append(b.sections, sb.String())
	return b
}

// Context adds RAG context documents.
func (b *PromptBuilder) Context(docs []string) *PromptBuilder {
	if len(docs) == 0 {
		return b
	}
	var sb strings.Builder
	sb.WriteString("【参考资料】\n")
	for i, d := range docs {
		sb.WriteString("[")
		sb.WriteString(strings.Repeat("", 0))
		sb.WriteString(itoa(i + 1))
		sb.WriteString("] ")
		sb.WriteString(d)
		sb.WriteString("\n")
	}
	b.sections = append(b.sections, sb.String())
	return b
}

// Memory adds memory context.
func (b *PromptBuilder) Memory(items []string) *PromptBuilder {
	if len(items) == 0 {
		return b
	}
	var sb strings.Builder
	sb.WriteString("【记忆上下文】\n")
	for _, m := range items {
		sb.WriteString("- ")
		sb.WriteString(m)
		sb.WriteString("\n")
	}
	b.sections = append(b.sections, sb.String())
	return b
}

// Build concatenates all sections into a final prompt string.
func (b *PromptBuilder) Build() string {
	return strings.Join(b.sections, "\n\n")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
