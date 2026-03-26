package type_

import (
	"context"
	"testing"
	"time"
	"xgc-agent/memory"
	"xgc-agent/memory/store/neo4jgraph"
)

type semanticTestStore struct {
	items         map[string]memory.MemoryItem
	addedEntities []graphEntityCall
	addedRels     []graphRelationCall
	memoryLinks   []graphMemoryLinkCall
}

type graphEntityCall struct {
	userID     string
	entityID   string
	name       string
	entityType string
	properties map[string]any
}

type graphRelationCall struct {
	userID           string
	fromEntityID     string
	toEntityID       string
	relationshipType string
	properties       map[string]any
}

type graphMemoryLinkCall struct {
	userID           string
	entityID         string
	memoryID         string
	relationshipType string
	properties       map[string]any
}

func newSemanticTestStore() *semanticTestStore {
	return &semanticTestStore{items: map[string]memory.MemoryItem{}}
}

func (s *semanticTestStore) Add(ctx context.Context, item memory.MemoryItem) error {
	s.items[item.MemoryID] = item.Clone()
	return nil
}

func (s *semanticTestStore) Get(ctx context.Context, userID string, memoryID string) (*memory.MemoryItem, error) {
	item, ok := s.items[memoryID]
	if !ok {
		return nil, memory.ErrNotFound
	}
	clone := item.Clone()
	return &clone, nil
}

func (s *semanticTestStore) Update(ctx context.Context, userID string, memoryID string, content string, topic []string, metadata map[string]any) error {
	item, ok := s.items[memoryID]
	if !ok {
		return memory.ErrNotFound
	}
	item.Memory.Content = content
	item.Memory.Topics = topic
	item.Memory.Metadata = metadata
	item.UpdatedAt = time.Now()
	s.items[memoryID] = item
	return nil
}

func (s *semanticTestStore) Delete(ctx context.Context, userID string, memoryID string) error {
	delete(s.items, memoryID)
	return nil
}

func (s *semanticTestStore) Clear(ctx context.Context, userID string) error {
	s.items = map[string]memory.MemoryItem{}
	return nil
}

func (s *semanticTestStore) List(ctx context.Context, userID string, limit int) ([]*memory.MemoryItem, error) {
	out := make([]*memory.MemoryItem, 0, len(s.items))
	for _, item := range s.items {
		clone := item.Clone()
		out = append(out, &clone)
	}
	return out, nil
}

func (s *semanticTestStore) Search(ctx context.Context, userID string, queryEmbedding []float32, limit int) ([]*memory.MemoryItem, error) {
	_ = ctx
	_ = queryEmbedding
	_ = limit
	if userID == "" {
		return nil, memory.ErrUserIDRequired
	}
	return nil, memory.ErrSearchNotSupported
}

func (s *semanticTestStore) AddEntity(ctx context.Context, userID string, entityID string, name string, entityType string, properties map[string]any) error {
	s.addedEntities = append(s.addedEntities, graphEntityCall{userID: userID, entityID: entityID, name: name, entityType: entityType, properties: properties})
	return nil
}

func (s *semanticTestStore) AddRelationship(ctx context.Context, userID string, fromEntityID string, toEntityID string, relationshipType string, properties map[string]any) error {
	s.addedRels = append(s.addedRels, graphRelationCall{userID: userID, fromEntityID: fromEntityID, toEntityID: toEntityID, relationshipType: relationshipType, properties: properties})
	return nil
}

func (s *semanticTestStore) LinkEntityToMemory(ctx context.Context, userID string, entityID string, memoryID string, relationshipType string, properties map[string]any) error {
	s.memoryLinks = append(s.memoryLinks, graphMemoryLinkCall{userID: userID, entityID: entityID, memoryID: memoryID, relationshipType: relationshipType, properties: properties})
	return nil
}

func (s *semanticTestStore) FindRelatedEntities(ctx context.Context, userID string, entityID string, relationshipTypes []string, maxDepth int, limit int) ([]neo4jgraph.RelatedEntity, error) {
	return nil, nil
}

func (s *semanticTestStore) SearchEntitiesByName(ctx context.Context, userID string, namePattern string, entityTypes []string, limit int) ([]neo4jgraph.Entity, error) {
	return nil, nil
}

func (s *semanticTestStore) GetEntityRelationships(ctx context.Context, userID string, entityID string) ([]neo4jgraph.EntityRelationship, error) {
	return nil, nil
}

func (s *semanticTestStore) DeleteEntity(ctx context.Context, userID string, entityID string) error {
	return nil
}

func (s *semanticTestStore) ClearAll(ctx context.Context, userID string) error {
	return nil
}

func (s *semanticTestStore) GetStats(ctx context.Context, userID string) (neo4jgraph.Stats, error) {
	return neo4jgraph.Stats{}, nil
}

func (s *semanticTestStore) HealthCheck(ctx context.Context) error {
	return nil
}

func (s *semanticTestStore) Close(ctx context.Context) error {
	return nil
}

func TestSemanticMemoryAddSyncsGraphMetadataAndRelationships(t *testing.T) {
	ctx := context.Background()
	store := newSemanticTestStore()
	mem := NewSemanticMemory("u1", store, nil)
	item := memory.MemoryItem{
		MemoryID: "mem-1",
		UserID:   "u1",
		Memory: &memory.Memory{
			Content: "Alice works at Acme",
			Topics:  []string{"graph", "memory"},
			Metadata: map[string]any{
				"entities": []any{
					map[string]any{"entity_id": "alice", "name": "Alice", "type": "person"},
					map[string]any{"entity_id": "acme", "name": "Acme", "type": "org"},
				},
				"relations": []any{
					map[string]any{"from": "alice", "to": "acme", "type": "works at", "properties": map[string]any{"confidence": 0.9}},
				},
			},
		},
	}

	if err := mem.Add(ctx, item); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	stored := store.items["mem-1"]
	if got := stored.Memory.Metadata["entity_count"]; got != 2 {
		t.Fatalf("entity_count = %v, want 2", got)
	}
	if got := stored.Memory.Metadata["relation_count"]; got != 1 {
		t.Fatalf("relation_count = %v, want 1", got)
	}
	if len(store.addedEntities) != 2 {
		t.Fatalf("AddEntity calls = %d, want 2", len(store.addedEntities))
	}
	if len(store.addedRels) != 1 {
		t.Fatalf("AddRelationship calls = %d, want 1", len(store.addedRels))
	}
	if len(store.memoryLinks) != 2 {
		t.Fatalf("LinkEntityToMemory calls = %d, want 2", len(store.memoryLinks))
	}
	if store.memoryLinks[0].relationshipType != semanticEntityMemoryRelationshipType {
		t.Fatalf("memory link type = %s, want %s", store.memoryLinks[0].relationshipType, semanticEntityMemoryRelationshipType)
	}
	if store.memoryLinks[0].memoryID != "mem-1" || store.memoryLinks[1].memoryID != "mem-1" {
		t.Fatalf("memory link targets = %+v, want all mem-1", store.memoryLinks)
	}
	if store.addedRels[0].relationshipType != "WORKS_AT" {
		t.Fatalf("semantic relationship type = %s, want WORKS_AT", store.addedRels[0].relationshipType)
	}
}

func TestSemanticMemoryAddDerivesEntitiesAndRelationsFromTopics(t *testing.T) {
	ctx := context.Background()
	store := newSemanticTestStore()
	mem := NewSemanticMemory("u1", store, nil)
	item := memory.MemoryItem{
		MemoryID: "mem-2",
		UserID:   "u1",
		Memory: &memory.Memory{
			Content:  "semantic topics",
			Topics:   []string{"Go", "Neo4j", "Go"},
			Metadata: map[string]any{},
		},
	}

	if err := mem.Add(ctx, item); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	stored := store.items["mem-2"]
	if got := stored.Memory.Metadata["entity_count"]; got != 2 {
		t.Fatalf("entity_count = %v, want 2", got)
	}
	if got := stored.Memory.Metadata["relation_count"]; got != 1 {
		t.Fatalf("relation_count = %v, want 1", got)
	}
	if len(store.addedEntities) != 2 {
		t.Fatalf("AddEntity calls = %d, want 2", len(store.addedEntities))
	}
	if len(store.addedRels) != 1 {
		t.Fatalf("AddRelationship calls = %d, want 1", len(store.addedRels))
	}
	if len(store.memoryLinks) != 2 {
		t.Fatalf("LinkEntityToMemory calls = %d, want 2", len(store.memoryLinks))
	}
}

func TestSemanticMemoryUpdateRefreshesEntityMemoryLinks(t *testing.T) {
	ctx := context.Background()
	store := newSemanticTestStore()
	mem := NewSemanticMemory("u1", store, nil)
	item := memory.MemoryItem{
		MemoryID: "mem-4",
		UserID:   "u1",
		Memory: &memory.Memory{
			Content: "Alice knows Bob",
			Metadata: map[string]any{
				"entities": []any{
					map[string]any{"entity_id": "alice", "name": "Alice"},
					map[string]any{"entity_id": "bob", "name": "Bob"},
				},
			},
		},
	}
	if err := mem.Add(ctx, item); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	store.memoryLinks = nil
	store.addedRels = nil

	updated := memory.MemoryItem{
		MemoryID: "mem-4",
		UserID:   "u1",
		Memory: &memory.Memory{
			Content: "Alice mentors Bob",
			Metadata: map[string]any{
				"entities": []any{
					map[string]any{"entity_id": "alice", "name": "Alice"},
					map[string]any{"entity_id": "bob", "name": "Bob"},
				},
				"relations": []any{
					map[string]any{"from": "alice", "to": "bob", "type": "mentors"},
				},
			},
		},
	}
	if err := mem.Update(ctx, updated); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if len(store.memoryLinks) != 2 {
		t.Fatalf("LinkEntityToMemory calls = %d, want 2", len(store.memoryLinks))
	}
	if len(store.addedRels) != 1 || store.addedRels[0].relationshipType != "MENTORS" {
		t.Fatalf("updated relationships = %+v, want one MENTORS", store.addedRels)
	}
}

func TestEnsureSemanticGraphMetadataRejectsInvalidRelationShape(t *testing.T) {
	item := &memory.MemoryItem{
		MemoryID: "mem-3",
		UserID:   "u1",
		Memory: &memory.Memory{
			Content: "bad relation",
			Metadata: map[string]any{
				"relations": []any{map[string]any{"from": "a"}},
			},
		},
	}

	if err := ensureSemanticGraphMetadata(item); err == nil {
		t.Fatal("ensureSemanticGraphMetadata() error = nil, want error")
	}
}
