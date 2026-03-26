package type_

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
	"xgc-agent/embedding"
	"xgc-agent/memory"
	"xgc-agent/memory/store"
	"xgc-agent/memory/store/neo4jgraph"
)

// SemanticMemoryImpl implements SemanticMemory with embedding-based search.
type SemanticMemoryImpl struct {
	userID   string
	store    store.StoreManager
	embedder embedding.EmbeddingService
}

func NewSemanticMemory(userID string, s store.StoreManager, embedder embedding.EmbeddingService) *SemanticMemoryImpl {
	return &SemanticMemoryImpl{userID: userID, store: s, embedder: embedder}
}

func (m *SemanticMemoryImpl) Add(ctx context.Context, items ...memory.MemoryItem) error {
	now := time.Now()
	for i := range items {
		items[i].Type = memory.MemorySemantic
		if items[i].UserID == "" {
			items[i].UserID = m.userID
		}
		if items[i].Memory == nil {
			items[i].Memory = &memory.Memory{}
		}
		if items[i].MemoryID == "" {
			items[i].MemoryID = memory.GenerateMemoryID(items[i].Memory, items[i].UserID)
		}
		if items[i].CreatedAt.IsZero() {
			items[i].CreatedAt = now
		}
		items[i].UpdatedAt = now
		if items[i].Memory.Metadata == nil {
			items[i].Memory.Metadata = map[string]any{}
		}
		items[i].Memory.Metadata["memory_type"] = string(memory.MemorySemantic)
		if m.embedder != nil && items[i].Embedding == nil && items[i].Memory != nil {
			vecs, err := m.embedder.Embed(ctx, []string{items[i].Memory.Content})
			if err == nil && len(vecs) > 0 {
				items[i].Embedding = vecs[0]
				items[i].Memory.Metadata[semanticEmbeddingMetadataKey] = vecs[0]
			}
		}
		if len(items[i].Embedding) > 0 {
			items[i].Memory.Metadata[semanticEmbeddingMetadataKey] = items[i].Embedding
		}
		if err := ensureSemanticGraphMetadata(&items[i]); err != nil {
			return err
		}
		if err := m.store.Add(ctx, items[i]); err != nil {
			return err
		}
		if err := m.syncGraph(ctx, items[i]); err != nil {
			return err
		}
	}
	return nil
}

func (m *SemanticMemoryImpl) AddKnowledge(ctx context.Context, items ...memory.MemoryItem) error {
	return m.Add(ctx, items...)
}

func (m *SemanticMemoryImpl) Update(ctx context.Context, items ...memory.MemoryItem) error {
	for _, it := range items {
		if it.Memory == nil {
			continue
		}
		if it.UserID == "" {
			it.UserID = m.userID
		}
		if it.Memory.Metadata == nil {
			it.Memory.Metadata = map[string]any{}
		}
		it.Memory.Metadata["memory_type"] = string(memory.MemorySemantic)
		if m.embedder != nil && strings.TrimSpace(it.Memory.Content) != "" {
			vecs, err := m.embedder.Embed(ctx, []string{it.Memory.Content})
			if err == nil && len(vecs) > 0 {
				it.Embedding = vecs[0]
				it.Memory.Metadata[semanticEmbeddingMetadataKey] = vecs[0]
			}
		}
		if len(it.Embedding) > 0 {
			it.Memory.Metadata[semanticEmbeddingMetadataKey] = it.Embedding
		}
		if err := ensureSemanticGraphMetadata(&it); err != nil {
			return err
		}
		if err := m.store.Update(ctx, m.userID, it.MemoryID, it.Memory.Content, it.Memory.Topics, it.Memory.Metadata); err != nil {
			return err
		}
		if err := m.syncGraph(ctx, it); err != nil {
			return err
		}
	}
	return nil
}

func (m *SemanticMemoryImpl) Get(ctx context.Context, id string) (memory.MemoryItem, error) {
	item, err := m.store.Get(ctx, m.userID, id)
	if err != nil {
		return memory.MemoryItem{}, err
	}
	return *item, nil
}

func (m *SemanticMemoryImpl) List(ctx context.Context, opts memory.ListOptions) ([]memory.MemoryItem, error) {
	items, err := m.store.List(ctx, m.userID, opts.Limit)
	if err != nil {
		return nil, err
	}
	out := make([]memory.MemoryItem, 0, len(items))
	now := time.Now()
	for _, it := range items {
		if it == nil || !isSemanticType(*it) {
			continue
		}
		if !opts.IncludeExpired && it.IsExpired(now) {
			continue
		}
		if opts.Since != nil && it.CreatedAt.Before(*opts.Since) {
			continue
		}
		if opts.Until != nil && it.CreatedAt.After(*opts.Until) {
			continue
		}
		out = append(out, it.Clone())
	}
	if opts.Limit > 0 && len(out) > opts.Limit {
		out = out[:opts.Limit]
	}
	return out, nil
}

func (m *SemanticMemoryImpl) Delete(ctx context.Context, id string) error {
	return m.store.Delete(ctx, m.userID, id)
}

func (m *SemanticMemoryImpl) Clear(ctx context.Context) error {
	return m.store.Clear(ctx, m.userID)
}

func (m *SemanticMemoryImpl) Search(ctx context.Context, query string, limit int) ([]memory.MemoryItem, error) {
	if graphStore, ok := m.store.(neo4jgraph.GraphStore); ok {
		items, err := m.searchByGraph(ctx, graphStore, query, limit)
		if err == nil && len(items) > 0 {
			return items, nil
		}
	}
	return m.searchByVector(ctx, query, limit)
}

func (m *SemanticMemoryImpl) SearchByTopic(ctx context.Context, topic string, limit int) ([]memory.MemoryItem, error) {
	all, err := m.store.List(ctx, m.userID, 0)
	if err != nil {
		return nil, err
	}
	var out []memory.MemoryItem
	for _, it := range all {
		if it == nil || !isSemanticType(*it) || it.Memory == nil {
			continue
		}
		for _, t := range it.Memory.Topics {
			if strings.EqualFold(t, topic) {
				out = append(out, it.Clone())
				break
			}
		}
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (m *SemanticMemoryImpl) searchByVector(ctx context.Context, query string, limit int) ([]memory.MemoryItem, error) {
	if m.embedder == nil {
		return nil, memory.ErrEmbeddingUnavailable
	}
	vecs, err := m.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, nil
	}

	all, err := m.store.List(ctx, m.userID, 0)
	if err != nil {
		return nil, err
	}
	queryVec := vecs[0]
	type scored struct {
		item  memory.MemoryItem
		score float64
	}
	results := make([]scored, 0, len(all))
	for _, raw := range all {
		if raw == nil || !isSemanticType(*raw) {
			continue
		}
		vec := raw.Embedding
		if len(vec) == 0 && raw.Memory != nil {
			vec = embeddingFromMetadata(raw.Memory.Metadata)
		}
		if len(vec) == 0 {
			continue
		}
		score := cosineSimilarity(queryVec, vec)
		item := raw.Clone()
		item.Score = score
		results = append(results, scored{item: item, score: score})
	}
	if len(results) == 0 {
		return nil, nil
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	out := make([]memory.MemoryItem, 0, len(results))
	for _, r := range results {
		out = append(out, r.item)
	}
	return out, nil
}

type semanticGraphCandidate struct {
	memoryID      string
	entityMatches int
	entityHits    int
	relationHits  int
}

func (m *SemanticMemoryImpl) searchByGraph(ctx context.Context, graphStore neo4jgraph.GraphStore, query string, limit int) ([]memory.MemoryItem, error) {
	queryEntities := extractQueryEntities(query)
	if len(queryEntities) == 0 {
		queryEntities = []string{strings.TrimSpace(query)}
	}

	matchedEntities := make([]neo4jgraph.Entity, 0)
	seenEntityIDs := map[string]struct{}{}
	for _, name := range queryEntities {
		if strings.TrimSpace(name) == "" {
			continue
		}
		entities, err := graphStore.SearchEntitiesByName(ctx, m.userID, name, nil, 20)
		if err != nil {
			continue
		}
		for _, entity := range entities {
			entityID := stringValue(entity.Props["id"])
			if entityID == "" {
				continue
			}
			if _, ok := seenEntityIDs[entityID]; ok {
				continue
			}
			seenEntityIDs[entityID] = struct{}{}
			matchedEntities = append(matchedEntities, entity)
		}
	}

	if len(matchedEntities) == 0 {
		entities, err := graphStore.SearchEntitiesByName(ctx, m.userID, query, nil, 20)
		if err != nil {
			return nil, err
		}
		for _, entity := range entities {
			entityID := stringValue(entity.Props["id"])
			if entityID == "" {
				continue
			}
			if _, ok := seenEntityIDs[entityID]; ok {
				continue
			}
			seenEntityIDs[entityID] = struct{}{}
			matchedEntities = append(matchedEntities, entity)
		}
	}

	if len(matchedEntities) == 0 {
		return nil, nil
	}

	candidates := map[string]*semanticGraphCandidate{}
	for _, entity := range matchedEntities {
		entityID := stringValue(entity.Props["id"])
		if entityID == "" {
			continue
		}

		if memoryID := stringValue(entity.Props["memory_id"]); memoryID != "" {
			candidate := upsertSemanticCandidate(candidates, memoryID)
			candidate.entityMatches++
			candidate.entityHits++
		}

		rels, err := graphStore.GetEntityRelationships(ctx, m.userID, entityID)
		if err == nil {
			for _, rel := range rels {
				if memoryID := stringValue(rel.Relationship["memory_id"]); memoryID != "" {
					candidate := upsertSemanticCandidate(candidates, memoryID)
					candidate.relationHits++
				}
				if memoryID := stringValue(rel.OtherEntity.Props["memory_id"]); memoryID != "" {
					candidate := upsertSemanticCandidate(candidates, memoryID)
					candidate.entityHits++
				}
			}
		}

		related, err := graphStore.FindRelatedEntities(ctx, m.userID, entityID, nil, 2, 50)
		if err == nil {
			for _, relatedEntity := range related {
				if memoryID := stringValue(relatedEntity.Entity.Props["memory_id"]); memoryID != "" {
					candidate := upsertSemanticCandidate(candidates, memoryID)
					candidate.entityHits++
					candidate.relationHits += len(relatedEntity.RelationshipPath)
				}
			}
		}
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	maxEntityHits := 1
	maxRelationHits := 1
	for _, candidate := range candidates {
		if candidate.entityHits > maxEntityHits {
			maxEntityHits = candidate.entityHits
		}
		if candidate.relationHits > maxRelationHits {
			maxRelationHits = candidate.relationHits
		}
	}

	results := make([]memory.MemoryItem, 0, len(candidates))
	for _, candidate := range candidates {
		item, err := m.store.Get(ctx, m.userID, candidate.memoryID)
		if err != nil || item == nil || !isSemanticType(*item) {
			continue
		}
		entityScore := 0.0
		if len(queryEntities) > 0 {
			entityScore = float64(candidate.entityMatches) / float64(len(queryEntities))
			if entityScore > 1 {
				entityScore = 1
			}
		}
		entityDensity := float64(candidate.entityHits) / float64(maxEntityHits)
		relationDensity := float64(candidate.relationHits) / float64(maxRelationHits)

		score := entityScore*semanticEntityScoreWeight +
			entityDensity*semanticEntityDensityWeight +
			relationDensity*semanticRelationDensityWeight

		clone := item.Clone()
		clone.Score = score
		results = append(results, clone)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func upsertSemanticCandidate(candidates map[string]*semanticGraphCandidate, memoryID string) *semanticGraphCandidate {
	if candidate, ok := candidates[memoryID]; ok {
		return candidate
	}
	candidate := &semanticGraphCandidate{memoryID: memoryID}
	candidates[memoryID] = candidate
	return candidate
}

var semanticEntityTokenRegex = regexp.MustCompile(`[\p{Han}]{2,}|[A-Za-z0-9_-]{2,}`)

func extractQueryEntities(query string) []string {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	tokens := semanticEntityTokenRegex.FindAllString(query, -1)
	if len(tokens) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(tokens))
	for _, raw := range tokens {
		token := strings.TrimSpace(raw)
		if token == "" {
			continue
		}
		if utf8.RuneCountInString(token) < 2 {
			continue
		}
		key := strings.ToLower(token)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, token)
	}
	return out
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (sqrt(normA) * sqrt(normB))
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 20; i++ {
		z = (z + x/z) / 2
	}
	return z
}

func (m *SemanticMemoryImpl) syncGraph(ctx context.Context, item memory.MemoryItem) error {
	graphStore, ok := m.store.(neo4jgraph.GraphStore)
	if !ok || item.Memory == nil {
		return nil
	}
	entities, err := extractGraphEntities(item)
	if err != nil {
		return err
	}
	for _, entity := range entities {
		if err := graphStore.AddEntity(ctx, item.UserID, entity.ID, entity.Name, entity.Type, entity.Properties); err != nil {
			return err
		}
		if err := graphStore.LinkEntityToMemory(ctx, item.UserID, entity.ID, item.MemoryID, semanticEntityMemoryRelationshipType, map[string]any{
			"memory_id": item.MemoryID,
			"user_id":   item.UserID,
			"source":    "semantic_memory",
		}); err != nil {
			return err
		}
	}

	relations, err := extractGraphRelations(item)
	if err != nil {
		return err
	}
	for _, rel := range relations {
		if err := graphStore.AddRelationship(ctx, item.UserID, rel.FromID, rel.ToID, rel.Type, rel.Properties); err != nil {
			return err
		}
	}
	return nil
}

const (
	semanticEntitiesMetadataKey           = "entities"
	semanticRelationsMetadataKey          = "relations"
	semanticEmbeddingMetadataKey          = "embedding"
	semanticEntityMemoryRelationshipType  = "MENTIONED_IN"
	semanticDefaultEntityType             = "CONCEPT"
	semanticDefaultRelationType           = "RELATED_TO"
	semanticEntityIDMetadataKey           = "entity_id"
	semanticEntityNameMetadataKey         = "name"
	semanticEntityTypeMetadataKey         = "type"
	semanticEntityPropertiesMetadataKey   = "properties"
	semanticRelationFromMetadataKey       = "from"
	semanticRelationToMetadataKey         = "to"
	semanticRelationTypeMetadataKey       = "type"
	semanticRelationPropertiesMetadataKey = "properties"
	semanticEntityScoreWeight             = 0.6
	semanticEntityDensityWeight           = 0.2
	semanticRelationDensityWeight         = 0.2
)

type semanticGraphEntity struct {
	ID         string
	Name       string
	Type       string
	Properties map[string]any
}

type semanticGraphRelation struct {
	FromID     string
	ToID       string
	Type       string
	Properties map[string]any
}

func ensureSemanticGraphMetadata(item *memory.MemoryItem) error {
	if item == nil || item.Memory == nil {
		return nil
	}
	if item.UserID == "" {
		return memory.ErrUserIDRequired
	}
	if item.MemoryID == "" {
		return memory.ErrMemoryIDRequired
	}
	if item.Memory.Metadata == nil {
		item.Memory.Metadata = map[string]any{}
	}
	entities, err := extractGraphEntities(*item)
	if err != nil {
		return err
	}
	relations, err := extractGraphRelations(*item)
	if err != nil {
		return err
	}
	item.Memory.Metadata[semanticEntitiesMetadataKey] = encodeGraphEntities(entities)
	item.Memory.Metadata[semanticRelationsMetadataKey] = encodeGraphRelations(relations)
	item.Memory.Metadata["entity_count"] = len(entities)
	item.Memory.Metadata["relation_count"] = len(relations)
	return nil
}

func extractGraphEntities(item memory.MemoryItem) ([]semanticGraphEntity, error) {
	if item.Memory == nil || item.Memory.Metadata == nil {
		return nil, nil
	}
	raw, ok := item.Memory.Metadata[semanticEntitiesMetadataKey]
	if !ok || raw == nil {
		return deriveGraphEntitiesFromTopics(item), nil
	}
	list, ok := raw.([]any)
	if !ok {
		if typed, ok := raw.([]map[string]any); ok {
			list = make([]any, 0, len(typed))
			for _, v := range typed {
				list = append(list, v)
			}
		} else {
			return nil, fmt.Errorf("semantic memory metadata %q must be an array", semanticEntitiesMetadataKey)
		}
	}
	out := make([]semanticGraphEntity, 0, len(list))
	for idx, rawEntity := range list {
		entityMap, ok := rawEntity.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("semantic entity at index %d must be an object", idx)
		}
		id := stringValue(entityMap[semanticEntityIDMetadataKey])
		name := stringValue(entityMap[semanticEntityNameMetadataKey])
		if id == "" {
			id = memory.GenerateMemoryID(&memory.Memory{Content: name}, item.UserID)
		}
		if name == "" {
			name = id
		}
		if id == "" || name == "" {
			return nil, fmt.Errorf("semantic entity at index %d requires id or name", idx)
		}
		entityType := strings.ToUpper(strings.TrimSpace(stringValue(entityMap[semanticEntityTypeMetadataKey])))
		if entityType == "" {
			entityType = semanticDefaultEntityType
		}
		props := copyStringAnyMap(asMap(entityMap[semanticEntityPropertiesMetadataKey]))
		if props == nil {
			props = map[string]any{}
		}
		props["memory_id"] = item.MemoryID
		props["user_id"] = item.UserID
		out = append(out, semanticGraphEntity{ID: id, Name: name, Type: entityType, Properties: props})
	}
	return out, nil
}

func extractGraphRelations(item memory.MemoryItem) ([]semanticGraphRelation, error) {
	if item.Memory == nil || item.Memory.Metadata == nil {
		return nil, nil
	}
	raw, ok := item.Memory.Metadata[semanticRelationsMetadataKey]
	if !ok || raw == nil {
		return deriveGraphRelationsFromEntities(item)
	}
	list, ok := raw.([]any)
	if !ok {
		if typed, ok := raw.([]map[string]any); ok {
			list = make([]any, 0, len(typed))
			for _, v := range typed {
				list = append(list, v)
			}
		} else {
			return nil, fmt.Errorf("semantic memory metadata %q must be an array", semanticRelationsMetadataKey)
		}
	}
	out := make([]semanticGraphRelation, 0, len(list))
	for idx, rawRelation := range list {
		relationMap, ok := rawRelation.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("semantic relation at index %d must be an object", idx)
		}
		fromID := stringValue(relationMap[semanticRelationFromMetadataKey])
		toID := stringValue(relationMap[semanticRelationToMetadataKey])
		if fromID == "" || toID == "" {
			return nil, fmt.Errorf("semantic relation at index %d requires from and to", idx)
		}
		relationType := sanitizeRelationshipType(stringValue(relationMap[semanticRelationTypeMetadataKey]))
		if relationType == "" {
			relationType = semanticDefaultRelationType
		}
		props := copyStringAnyMap(asMap(relationMap[semanticRelationPropertiesMetadataKey]))
		if props == nil {
			props = map[string]any{}
		}
		props["memory_id"] = item.MemoryID
		props["user_id"] = item.UserID
		out = append(out, semanticGraphRelation{FromID: fromID, ToID: toID, Type: relationType, Properties: props})
	}
	return out, nil
}

func deriveGraphEntitiesFromTopics(item memory.MemoryItem) []semanticGraphEntity {
	if item.Memory == nil {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]semanticGraphEntity, 0, len(item.Memory.Topics))
	for _, topic := range item.Memory.Topics {
		name := strings.TrimSpace(topic)
		if name == "" {
			continue
		}
		id := memory.GenerateMemoryID(&memory.Memory{Content: name}, item.UserID)
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, semanticGraphEntity{
			ID:   id,
			Name: name,
			Type: semanticDefaultEntityType,
			Properties: map[string]any{
				"source":    "topics",
				"memory_id": item.MemoryID,
				"user_id":   item.UserID,
			},
		})
	}
	return out
}

func deriveGraphRelationsFromEntities(item memory.MemoryItem) ([]semanticGraphRelation, error) {
	entities, err := extractGraphEntities(item)
	if err != nil {
		return nil, err
	}
	if len(entities) < 2 {
		return nil, nil
	}
	out := make([]semanticGraphRelation, 0, len(entities)*(len(entities)-1)/2)
	for i := 0; i < len(entities); i++ {
		for j := i + 1; j < len(entities); j++ {
			out = append(out, semanticGraphRelation{
				FromID: entities[i].ID,
				ToID:   entities[j].ID,
				Type:   semanticDefaultRelationType,
				Properties: map[string]any{
					"source":    "co_occurrence",
					"memory_id": item.MemoryID,
					"user_id":   item.UserID,
				},
			})
		}
	}
	return out, nil
}

func encodeGraphEntities(entities []semanticGraphEntity) []map[string]any {
	out := make([]map[string]any, 0, len(entities))
	for _, entity := range entities {
		out = append(out, map[string]any{
			semanticEntityIDMetadataKey:         entity.ID,
			semanticEntityNameMetadataKey:       entity.Name,
			semanticEntityTypeMetadataKey:       entity.Type,
			semanticEntityPropertiesMetadataKey: copyStringAnyMap(entity.Properties),
		})
	}
	return out
}

func encodeGraphRelations(relations []semanticGraphRelation) []map[string]any {
	out := make([]map[string]any, 0, len(relations))
	for _, relation := range relations {
		out = append(out, map[string]any{
			semanticRelationFromMetadataKey:       relation.FromID,
			semanticRelationToMetadataKey:         relation.ToID,
			semanticRelationTypeMetadataKey:       relation.Type,
			semanticRelationPropertiesMetadataKey: copyStringAnyMap(relation.Properties),
		})
	}
	return out
}

func sanitizeRelationshipType(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-' || r == ' ':
			b.WriteByte('_')
		}
	}
	return strings.Trim(b.String(), "_")
}

func stringValue(v any) string {
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

func copyStringAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func isSemanticType(item memory.MemoryItem) bool {
	if item.Type == memory.MemorySemantic {
		return true
	}
	if item.Memory == nil || item.Memory.Metadata == nil {
		return false
	}
	if t, ok := item.Memory.Metadata["memory_type"].(string); ok {
		return strings.EqualFold(strings.TrimSpace(t), string(memory.MemorySemantic))
	}
	return false
}

func embeddingFromMetadata(metadata map[string]any) []float32 {
	if metadata == nil {
		return nil
	}
	raw, ok := metadata[semanticEmbeddingMetadataKey]
	if !ok || raw == nil {
		return nil
	}
	if vec, ok := raw.([]float32); ok {
		return vec
	}
	if values, ok := raw.([]any); ok {
		out := make([]float32, 0, len(values))
		for _, v := range values {
			switch n := v.(type) {
			case float64:
				out = append(out, float32(n))
			case float32:
				out = append(out, n)
			case int:
				out = append(out, float32(n))
			case int64:
				out = append(out, float32(n))
			}
		}
		return out
	}
	return nil
}

var _ SemanticMemory = (*SemanticMemoryImpl)(nil)
