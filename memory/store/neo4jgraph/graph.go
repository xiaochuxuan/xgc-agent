package neo4jgraph

import (
	"context"
	"fmt"
	"strings"
	"time"

	"xgc-agent/memory"
	"xgc-agent/memory/store"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	neo4jconfig "github.com/neo4j/neo4j-go-driver/v5/neo4j/config"
)

type GraphStore interface {
	store.StoreManager
	AddEntity(ctx context.Context, userID string, entityID string, name string, entityType string, properties map[string]any) error
	AddRelationship(ctx context.Context, userID string, fromEntityID string, toEntityID string, relationshipType string, properties map[string]any) error
	FindRelatedEntities(ctx context.Context, userID string, entityID string, relationshipTypes []string, maxDepth int, limit int) ([]RelatedEntity, error)
	SearchEntitiesByName(ctx context.Context, userID string, namePattern string, entityTypes []string, limit int) ([]Entity, error)
	GetEntityRelationships(ctx context.Context, userID string, entityID string) ([]EntityRelationship, error)
	DeleteEntity(ctx context.Context, userID string, entityID string) error
	ClearAll(ctx context.Context, userID string) error
	GetStats(ctx context.Context, userID string) (Stats, error)
	HealthCheck(ctx context.Context) error
	Close(ctx context.Context) error
}

// Entity represents a graph entity node.
type Entity struct {
	Props map[string]any
}

// RelatedEntity represents a related entity with path details.
type RelatedEntity struct {
	Entity Entity
	// Distance represents the number of hops between the original entity and the related entity.
	Distance int
	// RelationshipPath is a list of relationship types that connect the original entity to the related entity
	// ordered from the original entity to the related entity.
	RelationshipPath []string
}

// EntityRelationship represents a relationship between entities.
type EntityRelationship struct {
	Relationship map[string]any
	OtherEntity  Entity
	Direction    string
}

// Stats provides graph database statistics.
type Stats struct {
	TotalNodes         int64
	TotalRelationships int64
	EntityNodes        int64
	MemoryNodes        int64
}

// Neo4jGraphStore implements a Neo4j-backed graph store.
type Neo4jGraphStore struct {
	// driver is the Neo4j driver instance for database interactions.
	driver neo4j.DriverWithContext
	// options holds the configuration settings for the graph store.
	options neo4jOptions
}

func (s *Neo4jGraphStore) Add(ctx context.Context, userID string, content string, topics []string, metadata map[string]any) error {
	//TODO implement me
	panic("implement me")
}

func (s *Neo4jGraphStore) Get(ctx context.Context, userID string, memoryID string) (*memory.MemoryItem, error) {
	//TODO implement me
	panic("implement me")
}

func (s *Neo4jGraphStore) Update(ctx context.Context, userID string, memoryID string, content string, topic []string, metadata map[string]any) error {
	//TODO implement me
	panic("implement me")
}

func (s *Neo4jGraphStore) Delete(ctx context.Context, userID string, memoryID string) error {
	//TODO implement me
	panic("implement me")
}

func (s *Neo4jGraphStore) Clear(ctx context.Context, userID string) error {
	//TODO implement me
	panic("implement me")
}

func (s *Neo4jGraphStore) List(ctx context.Context, userID string, limit int) ([]*memory.MemoryItem, error) {
	//TODO implement me
	panic("implement me")
}

// NewNeo4jGraphStore creates a new Neo4j graph store.
func NewNeo4jGraphStore(ctx context.Context, opts ...Neo4jOption) (*Neo4jGraphStore, error) {
	cfg := defaultOptions()
	for _, opt := range opts {
		opt(&cfg)
	}
	if strings.TrimSpace(cfg.uri) == "" {
		return nil, fmt.Errorf("invalid URI")
	}

	// Initialize Neo4j driver with context and custom configuration
	driver, err := neo4j.NewDriverWithContext(
		cfg.uri,
		neo4j.BasicAuth(cfg.username, cfg.password, ""),
		func(conf *neo4jconfig.Config) {
			conf.MaxConnectionLifetime = cfg.maxConnectionLifetime
			conf.MaxConnectionPoolSize = cfg.maxConnectionPoolSize
			conf.ConnectionAcquisitionTimeout = cfg.connectionAcquisitionTimeout
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Neo4j driver: %w", err)
	}

	// Verify connectivity before returning the store instance
	if err := driver.VerifyConnectivity(ctx); err != nil {
		_ = driver.Close(ctx)
		return nil, fmt.Errorf("failed to verify Neo4j connectivity: %w", err)
	}

	store := &Neo4jGraphStore{
		driver:  driver,
		options: cfg,
	}
	if cfg.createIndexes {
		if err := store.createIndexes(ctx); err != nil {
			_ = driver.Close(ctx)
			return nil, fmt.Errorf("failed to create indexes: %w", err)
		}
	}

	return store, nil
}

// Close closes the Neo4j driver.
func (s *Neo4jGraphStore) Close(ctx context.Context) error {
	if s == nil || s.driver == nil {
		return nil
	}
	return s.driver.Close(ctx)
}

// createIndexes creates necessary indexes for efficient querying.
func (s *Neo4jGraphStore) createIndexes(ctx context.Context) error {
	queries := []string{
		// entity indexes
		"CREATE INDEX entity_id_index IF NOT EXISTS FOR (e:Entity) ON (e.id)",
		"CREATE INDEX entity_user_id_index IF NOT EXISTS FOR (e:Entity) ON (e.user_id)",
		"CREATE INDEX entity_name_index IF NOT EXISTS FOR (e:Entity) ON (e.name)",
		"CREATE INDEX entity_type_index IF NOT EXISTS FOR (e:Entity) ON (e.type)",
		// memory indexes
		"CREATE INDEX memory_id_index IF NOT EXISTS FOR (m:Memory) ON (m.id)",
		"CREATE INDEX memory_type_index IF NOT EXISTS FOR (m:Memory) ON (m.memory_type)",
		"CREATE INDEX memory_timestamp_index IF NOT EXISTS FOR (m:Memory) ON (m.timestamp)",
	}

	// Execute index creation queries in a single transaction for efficiency
	session := s.driver.NewSession(ctx,
		neo4j.SessionConfig{
			DatabaseName: s.options.database,
			AccessMode:   neo4j.AccessModeWrite},
	)
	defer session.Close(ctx)

	for _, query := range queries {
		_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			_, err := tx.Run(ctx, query, nil)
			return nil, err
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// AddEntity upserts an entity node.
func (s *Neo4jGraphStore) AddEntity(ctx context.Context, userID string, entityID string, name string,
	entityType string, properties map[string]any) error {
	if err := checkEntity(userID, entityID); err != nil {
		return err
	}
	if strings.TrimSpace(name) == "" {
		return ErrNameRequired
	}
	if strings.TrimSpace(entityType) == "" {
		return ErrTypeRequired
	}

	// Prepare properties for upsert
	props := map[string]any{}
	for k, v := range properties {
		props[k] = v
	}
	now := time.Now()
	props["id"] = entityID
	props["user_id"] = userID
	props["name"] = name
	props["type"] = entityType
	props["updated_at"] = now
	if _, ok := props["created_at"]; !ok {
		props["created_at"] = now
	}

	// Use MERGE to create or update the entity node based on the unique combination of user_id and id
	query := "MERGE (e:Entity {id: $entity_id, user_id: $user_id}) SET e += $props RETURN e"
	params := map[string]any{"entity_id": entityID, "user_id": userID, "props": props}

	// Execute the query in a write transaction
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: s.options.database,
		AccessMode:   neo4j.AccessModeWrite},
	)
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}
		_, err = result.Single(ctx)
		return nil, err
	})
	if err != nil {
		return fmt.Errorf("failed to add entity: %w", err)
	}

	return err
}

// AddRelationship creates or updates a relationship between entities.
func (s *Neo4jGraphStore) AddRelationship(ctx context.Context, userID string,
	fromEntityID string, toEntityID string, relationshipType string, properties map[string]any) error {
	if strings.TrimSpace(userID) == "" {
		return ErrUserIDRequired
	}
	if strings.TrimSpace(fromEntityID) == "" {
		return ErrFromIDRequired
	}
	if strings.TrimSpace(toEntityID) == "" {
		return ErrToIDRequired
	}
	if !validRelationshipType(relationshipType) {
		return ErrRelationshipType
	}

	// Prepare properties for upsert
	props := map[string]any{}
	for k, v := range properties {
		props[k] = v
	}
	now := time.Now()
	props["type"] = relationshipType
	props["updated_at"] = now.UTC()
	if _, ok := props["created_at"]; !ok {
		props["created_at"] = now.UTC()
	}

	// Use MERGE to create or update the relationship based on the unique combination of from_id, to_id, and type
	query := fmt.Sprintf(
		"MATCH (from:Entity {id: $from_id, user_id: $user_id}) "+
			"MATCH (to:Entity {id: $to_id, user_id: $user_id}) "+
			"MERGE (from)-[r:%s]->(to) SET r += $props RETURN r",
		relationshipType,
	)

	// Execute the query in a write transaction
	params := map[string]any{"from_id": fromEntityID, "to_id": toEntityID, "user_id": userID, "props": props}

	session := s.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: s.options.database,
		AccessMode:   neo4j.AccessModeWrite},
	)
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}
		_, err = result.Single(ctx)
		return nil, err
	})
	if err != nil {
		return fmt.Errorf("failed to add relationship: %w", err)
	}

	return err
}

// FindRelatedEntities returns related entities for a given entity.
// maxDepth specifies how many levels of relationships to traverse
func (s *Neo4jGraphStore) FindRelatedEntities(ctx context.Context, userID string, entityID string,
	relationshipTypes []string, maxDepth int, limit int) ([]RelatedEntity, error) {
	if err := checkEntity(userID, entityID); err != nil {
		return nil, err
	}
	if maxDepth <= 0 {
		maxDepth = 2
	}
	if limit <= 0 {
		limit = 50
	}

	// Build relationship type filter for Cypher query
	relFilter := ""
	if len(relationshipTypes) > 0 {
		validTypes := make([]string, 0, len(relationshipTypes))
		for _, relType := range relationshipTypes {
			if !validRelationshipType(relType) {
				return nil, ErrRelationshipType
			}
			validTypes = append(validTypes, relType)
		}
		relFilter = ":" + strings.Join(validTypes, "|")
	}

	query := fmt.Sprintf(
		"MATCH path = (start:Entity {id: $entity_id, user_id: $user_id})-[r%s*1..%d]-(related:Entity {user_id: $user_id}) "+
			"WHERE start.id <> related.id "+
			"RETURN DISTINCT related, length(path) AS distance, "+
			"[rel in relationships(path) | type(rel)] AS relationship_path "+
			"ORDER BY distance, related.name LIMIT $limit",
		relFilter,
		maxDepth,
	)
	params := map[string]any{"entity_id": entityID, "user_id": userID, "limit": limit}

	// Execute the query in a read transaction
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: s.options.database,
		AccessMode:   neo4j.AccessModeRead},
	)
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, fmt.Errorf("failed to execute query: %w", err)
		}
		var out []RelatedEntity
		for res.Next(ctx) {
			record := res.Record()
			relatedVal, _ := record.Get("related")
			distanceVal, _ := record.Get("distance")
			pathVal, _ := record.Get("relationship_path")

			node, ok := relatedVal.(neo4j.Node)
			if !ok {
				return nil, fmt.Errorf("unexpected related entity type")
			}
			distance := int64(0)
			switch v := distanceVal.(type) {
			case int64:
				distance = v
			case int:
				distance = int64(v)
			}

			out = append(out, RelatedEntity{
				Entity:           Entity{Props: node.Props},
				Distance:         int(distance),
				RelationshipPath: toStringSlice(pathVal),
			})
		}
		if err := res.Err(); err != nil {
			return nil, fmt.Errorf("failed to process query results: %w", err)
		}
		return out, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute read transaction: %w", err)
	}
	if result == nil {
		return []RelatedEntity{}, nil
	}
	return result.([]RelatedEntity), nil
}

// SearchEntitiesByName searches entities by name pattern.
func (s *Neo4jGraphStore) SearchEntitiesByName(ctx context.Context, userID string, namePattern string,
	entityTypes []string, limit int) ([]Entity, error) {
	if err := checkEntity(userID, ""); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}

	filter := ""
	params := map[string]any{"user_id": userID, "pattern": ".*" + namePattern + ".*", "limit": limit}
	if len(entityTypes) > 0 {
		filter = "AND e.type IN $types"
		params["types"] = entityTypes
	}

	query := fmt.Sprintf(
		"MATCH (e:Entity) WHERE e.user_id = $user_id AND e.name =~ $pattern %s RETURN e ORDER BY e.name LIMIT $limit",
		filter,
	)

	session := s.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: s.options.database,
		AccessMode:   neo4j.AccessModeRead},
	)
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, fmt.Errorf("failed to execute query: %w", err)
		}
		entities := make([]Entity, 0)
		for res.Next(ctx) {
			record := res.Record()
			val, _ := record.Get("e")
			node, ok := val.(neo4j.Node)
			if !ok {
				return nil, fmt.Errorf("unexpected entity type")
			}
			entities = append(entities, Entity{Props: node.Props})
		}
		if err := res.Err(); err != nil {
			return nil, fmt.Errorf("failed to process query results: %w", err)
		}
		return entities, nil
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return []Entity{}, nil
	}
	return result.([]Entity), nil
}

// GetEntityRelationships returns all relationships for an entity.
func (s *Neo4jGraphStore) GetEntityRelationships(ctx context.Context, userID string, entityID string) ([]EntityRelationship, error) {
	if err := checkEntity(userID, entityID); err != nil {
		return nil, err
	}

	query := "MATCH (e:Entity {id: $entity_id, user_id: $user_id})-[r]-(other:Entity {user_id: $user_id}) " +
		"RETURN r, other, CASE WHEN startNode(r).id = $entity_id THEN 'outgoing' ELSE 'incoming' END AS direction"
	params := map[string]any{"entity_id": entityID, "user_id": userID}

	session := s.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: s.options.database,
		AccessMode:   neo4j.AccessModeRead},
	)
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}
		relationships := make([]EntityRelationship, 0)
		for res.Next(ctx) {
			record := res.Record()
			relVal, _ := record.Get("r")
			otherVal, _ := record.Get("other")
			dirVal, _ := record.Get("direction")

			rel, ok := relVal.(neo4j.Relationship)
			if !ok {
				return nil, fmt.Errorf("unexpected relationship type")
			}
			node, ok := otherVal.(neo4j.Node)
			if !ok {
				return nil, fmt.Errorf("unexpected other entity type")
			}
			direction, _ := dirVal.(string)
			relationships = append(relationships, EntityRelationship{
				Relationship: rel.Props,
				OtherEntity:  Entity{Props: node.Props},
				Direction:    direction,
			})
		}
		if err := res.Err(); err != nil {
			return nil, err
		}
		return relationships, nil
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return []EntityRelationship{}, nil
	}
	return result.([]EntityRelationship), nil
}

// DeleteEntity deletes an entity and all its relationships.
func (s *Neo4jGraphStore) DeleteEntity(ctx context.Context, userID string, entityID string) error {
	if err := checkEntity(userID, entityID); err != nil {
		return err
	}

	query := "MATCH (e:Entity {id: $entity_id, user_id: $user_id}) DETACH DELETE e"
	params := map[string]any{"entity_id": entityID, "user_id": userID}

	session := s.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: s.options.database,
		AccessMode:   neo4j.AccessModeWrite},
	)
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, fmt.Errorf("failed to execute query: %w", err)
		}
		summary, err := res.Consume(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to consume query results: %w", err)
		}
		if summary.Counters().NodesDeleted() == 0 {
			return nil, fmt.Errorf("no nodes deleted")
		}
		return nil, nil
	})

	return err
}

// ClearAll clears all nodes and relationships.
func (s *Neo4jGraphStore) ClearAll(ctx context.Context, userID string) error {
	if strings.TrimSpace(userID) == "" {
		return ErrUserIDRequired
	}
	query := "MATCH (n {user_id: $user_id}) DETACH DELETE n"

	session := s.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: s.options.database,
		AccessMode:   neo4j.AccessModeWrite},
	)
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, query, map[string]any{"user_id": userID})
		return nil, err
	})

	return err
}

// GetStats returns graph statistics.
func (s *Neo4jGraphStore) GetStats(ctx context.Context, userID string) (Stats, error) {
	if strings.TrimSpace(userID) == "" {
		return Stats{}, ErrUserIDRequired
	}

	queries := map[string]string{
		"total_nodes":         "MATCH (n {user_id: $user_id}) RETURN count(n) as count",
		"total_relationships": "MATCH (a {user_id: $user_id})-[r]->(b {user_id: $user_id}) RETURN count(r) as count",
		"entity_nodes":        "MATCH (n:Entity {user_id: $user_id}) RETURN count(n) as count",
		"memory_nodes":        "MATCH (n:Memory {user_id: $user_id}) RETURN count(n) as count",
	}
	results := map[string]int64{}

	session := s.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: s.options.database,
		AccessMode:   neo4j.AccessModeRead,
	})
	defer session.Close(ctx)

	for key, query := range queries {
		value, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			res, err := tx.Run(ctx, query, map[string]any{"user_id": userID})
			if err != nil {
				return nil, err
			}
			record, err := res.Single(ctx)
			if err != nil {
				return nil, err
			}
			countVal, _ := record.Get("count")
			switch v := countVal.(type) {
			case int64:
				return v, nil
			case int:
				return int64(v), nil
			default:
				return int64(0), nil
			}
		})
		if err != nil {
			return Stats{}, err
		}
		results[key] = value.(int64)
	}

	return Stats{
		TotalNodes:         results["total_nodes"],
		TotalRelationships: results["total_relationships"],
		EntityNodes:        results["entity_nodes"],
		MemoryNodes:        results["memory_nodes"],
	}, nil
}

// HealthCheck validates driver connectivity.
func (s *Neo4jGraphStore) HealthCheck(ctx context.Context) error {
	if err := s.driver.VerifyConnectivity(ctx); err != nil {
		return fmt.Errorf("failed to verify connectivity: %w", err)
	}

	session := s.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: s.options.database,
		AccessMode:   neo4j.AccessModeRead},
	)
	defer session.Close(ctx)

	_, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, "RETURN 1 as health", nil)
		if err != nil {
			return nil, err
		}
		record, err := res.Single(ctx)
		if err != nil {
			return nil, err
		}
		val, _ := record.Get("health")
		if v, ok := val.(int64); ok && v == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("health check failed")
	})

	return err
}

func validRelationshipType(relType string) bool {
	if strings.TrimSpace(relType) == "" {
		return false
	}
	for _, r := range relType {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}

func toStringSlice(value any) []string {
	if value == nil {
		return nil
	}
	if list, ok := value.([]string); ok {
		return list
	}
	if list, ok := value.([]any); ok {
		out := make([]string, 0, len(list))
		for _, item := range list {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func checkEntity(userID string, entityID string) error {
	if strings.TrimSpace(userID) == "" {
		return ErrUserIDRequired
	}
	if strings.TrimSpace(entityID) == "" {
		return ErrEntityIDRequired
	}
	return nil
}
