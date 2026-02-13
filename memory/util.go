package memory

import (
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"
)

// GenerateMemoryID generates a unique ID for memory based on content and user context.
// Uses SHA256 hash of memory content, sorted topics, sorted metadata, and user ID for consistent ID generation.
// This ensures that:
// 1. Same content with different topic order produces the same ID.
// 2. Different users with same content produce different IDs.
func GenerateMemoryID(mem *Memory, userID string) string {
	var builder strings.Builder
	builder.WriteString("memory:")
	builder.WriteString(mem.Content)

	if len(mem.Topics) > 0 {
		// Sort topics to ensure consistent ordering.
		sortedTopics := make([]string, len(mem.Topics))
		copy(sortedTopics, mem.Topics)
		slices.Sort(sortedTopics)
		builder.WriteString("|topics:")
		builder.WriteString(strings.Join(sortedTopics, ","))
	}

	if len(mem.Metadata) > 0 {
		// Collect and sort metadata keys for consistent ordering.
		metaKeys := make([]string, 0, len(mem.Metadata))
		for k := range mem.Metadata {
			metaKeys = append(metaKeys, k)
		}
		slices.Sort(metaKeys)
		builder.WriteString("|metadata:")
		for _, k := range metaKeys {
			builder.WriteString(k)
			builder.WriteString("=")
			builder.WriteString(fmt.Sprintf("%v", mem.Metadata[k]))
			builder.WriteString(",")
		}
	}

	// Include user ID to prevent cross-user conflicts.
	builder.WriteString("|user:")
	builder.WriteString(userID)

	hash := sha256.Sum256([]byte(builder.String()))
	return fmt.Sprintf("%x", hash)
}
