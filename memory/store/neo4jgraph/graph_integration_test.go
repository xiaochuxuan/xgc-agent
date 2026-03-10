package neo4jgraph

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestNeo4jGraphStore_Integration_BasicFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	uri := os.Getenv("NEO4J_URI")
	username := os.Getenv("NEO4J_USERNAME")
	password := os.Getenv("NEO4J_PASSWORD")
	database := os.Getenv("NEO4J_DATABASE")

	if uri == "" || username == "" || password == "" {
		t.Skip("set NEO4J_URI, NEO4J_USERNAME, NEO4J_PASSWORD to run integration tests")
	}
	if database == "" {
		database = "neo4j"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store, err := NewNeo4jGraphStore(
		ctx,
		WithURI(uri),
		WithUsername(username),
		WithPassword(password),
		WithDatabase(database),
		WithCreateIndexes(true),
	)
	if err != nil {
		t.Fatalf("NewNeo4jGraphStore: %v", err)
	}
	defer store.Close(context.Background())

	userID := "it-user-" + time.Now().UTC().Format("20060102150405.000000000")
	defer store.ClearAll(context.Background(), userID)

	if err := store.HealthCheck(ctx); err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}

	if err := store.AddEntity(ctx, userID, "e1", "Alice", "person", map[string]any{"role": "engineer"}); err != nil {
		t.Fatalf("AddEntity e1: %v", err)
	}
	if err := store.AddEntity(ctx, userID, "e2", "Bob", "person", map[string]any{"role": "manager"}); err != nil {
		t.Fatalf("AddEntity e2: %v", err)
	}

	if err := store.AddRelationship(ctx, userID, "e1", "e2", "KNOWS", map[string]any{"since": 2020}); err != nil {
		t.Fatalf("AddRelationship: %v", err)
	}

	related, err := store.FindRelatedEntities(ctx, userID, "e1", []string{"KNOWS"}, 2, 10)
	if err != nil {
		t.Fatalf("FindRelatedEntities: %v", err)
	}
	if len(related) == 0 {
		t.Fatalf("FindRelatedEntities returned empty result")
	}

	entities, err := store.SearchEntitiesByName(ctx, userID, "Alic", []string{"person"}, 10)
	if err != nil {
		t.Fatalf("SearchEntitiesByName: %v", err)
	}
	if len(entities) == 0 {
		t.Fatalf("SearchEntitiesByName returned empty result")
	}

	relationships, err := store.GetEntityRelationships(ctx, userID, "e1")
	if err != nil {
		t.Fatalf("GetEntityRelationships: %v", err)
	}
	if len(relationships) == 0 {
		t.Fatalf("GetEntityRelationships returned empty result")
	}

	stats, err := store.GetStats(ctx, userID)
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.EntityNodes < 2 {
		t.Fatalf("EntityNodes=%d, want >=2", stats.EntityNodes)
	}
	if stats.TotalRelationships < 1 {
		t.Fatalf("TotalRelationships=%d, want >=1", stats.TotalRelationships)
	}

	if err := store.DeleteEntity(ctx, userID, "e2"); err != nil {
		t.Fatalf("DeleteEntity: %v", err)
	}

	if err := store.ClearAll(ctx, userID); err != nil {
		t.Fatalf("ClearAll: %v", err)
	}
}
