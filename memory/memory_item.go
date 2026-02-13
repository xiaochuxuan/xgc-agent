// memory_item.go: Standardized memory item definition.
package memory

import "time"

// MemoryType represents a logical memory category.
type MemoryType string

const (
	MemoryWorking    MemoryType = "working"
	MemoryEpisodic   MemoryType = "episodic"
	MemorySemantic   MemoryType = "semantic"
	MemoryPerceptual MemoryType = "perceptual"
)

// MemoryItem is the standardized memory record.
type MemoryItem struct {
	ID        string
	Type      MemoryType
	Content   string
	Modality  string
	Metadata  map[string]any
	CreatedAt time.Time
	UpdatedAt time.Time
	ExpiresAt *time.Time
}

// Clone returns a shallow copy with metadata map copied.
func (m MemoryItem) Clone() MemoryItem {
	clone := m
	if m.Metadata != nil {
		clone.Metadata = make(map[string]any, len(m.Metadata))
		for k, v := range m.Metadata {
			clone.Metadata[k] = v
		}
	}
	return clone
}
