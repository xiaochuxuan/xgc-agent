package neo4jgraph

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"xgc-agent/tools"
)

func TestValidRelationshipType(t *testing.T) {
	tests := []struct {
		name    string
		relType string
		want    bool
	}{
		{name: "simple uppercase", relType: "FRIEND_OF", want: true},
		{name: "letters and numbers", relType: "REL_123", want: true},
		{name: "empty", relType: "", want: false},
		{name: "spaces only", relType: "   ", want: false},
		{name: "contains dash", relType: "FRIEND-OF", want: false},
		{name: "contains space", relType: "FRIEND OF", want: false},
		{name: "contains punctuation", relType: "FRIEND!", want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := validRelationshipType(tt.relType)
			if got != tt.want {
				t.Fatalf("validRelationshipType(%q)=%v, want %v", tt.relType, got, tt.want)
			}
		})
	}
}

func TestToStringSlice(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  []string
	}{
		{name: "nil", input: nil, want: nil},
		{name: "string slice", input: []string{"a", "b"}, want: []string{"a", "b"}},
		{name: "any slice with strings", input: []any{"x", "y"}, want: []string{"x", "y"}},
		{name: "any slice mixed types", input: []any{"x", 1, "z", true}, want: []string{"x", "z"}},
		{name: "unsupported type", input: 123, want: nil},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := toStringSlice(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("toStringSlice(%v) len=%d, want %d", tt.input, len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("toStringSlice(%v)[%d]=%q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestCheckEntity(t *testing.T) {
	tests := []struct {
		name     string
		userID   string
		entityID string
		wantErr  error
	}{
		{name: "missing user", userID: "", entityID: "e1", wantErr: ErrUserIDRequired},
		{name: "missing entity", userID: "u1", entityID: "", wantErr: ErrEntityIDRequired},
		{name: "both missing", userID: "", entityID: "", wantErr: ErrUserIDRequired},
		{name: "valid", userID: "u1", entityID: "e1", wantErr: nil},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := checkEntity(tt.userID, tt.entityID)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("checkEntity(%q, %q) err=%v, want %v", tt.userID, tt.entityID, err, tt.wantErr)
			}
		})
	}
}

func TestNeo4jGraphStoreCloseNilReceiver(t *testing.T) {
	var s *Neo4jGraphStore
	if err := s.Close(context.Background()); err != nil {
		t.Fatalf("Close on nil receiver err=%v, want nil", err)
	}
}

func TestNeo4jGraphStoreAddEntityValidation(t *testing.T) {
	ctx := context.Background()
	s := &Neo4jGraphStore{}

	if err := s.AddEntity(ctx, "", "e1", "name", "type", nil); !errors.Is(err, ErrUserIDRequired) {
		t.Fatalf("AddEntity empty user err=%v, want %v", err, ErrUserIDRequired)
	}
	if err := s.AddEntity(ctx, "u1", "", "name", "type", nil); !errors.Is(err, ErrEntityIDRequired) {
		t.Fatalf("AddEntity empty entity err=%v, want %v", err, ErrEntityIDRequired)
	}
	if err := s.AddEntity(ctx, "u1", "e1", "", "type", nil); !errors.Is(err, ErrNameRequired) {
		t.Fatalf("AddEntity empty name err=%v, want %v", err, ErrNameRequired)
	}
	if err := s.AddEntity(ctx, "u1", "e1", "name", "", nil); !errors.Is(err, ErrTypeRequired) {
		t.Fatalf("AddEntity empty type err=%v, want %v", err, ErrTypeRequired)
	}
}

func TestNeo4jGraphStoreAddRelationshipValidation(t *testing.T) {
	ctx := context.Background()
	s := &Neo4jGraphStore{}

	if err := s.AddRelationship(ctx, "", "a", "b", "REL", nil); !errors.Is(err, ErrUserIDRequired) {
		t.Fatalf("AddRelationship empty user err=%v, want %v", err, ErrUserIDRequired)
	}
	if err := s.AddRelationship(ctx, "u1", "", "b", "REL", nil); !errors.Is(err, ErrFromIDRequired) {
		t.Fatalf("AddRelationship empty from err=%v, want %v", err, ErrFromIDRequired)
	}
	if err := s.AddRelationship(ctx, "u1", "a", "", "REL", nil); !errors.Is(err, ErrToIDRequired) {
		t.Fatalf("AddRelationship empty to err=%v, want %v", err, ErrToIDRequired)
	}
	if err := s.AddRelationship(ctx, "u1", "a", "b", "REL-INVALID", nil); !errors.Is(err, ErrRelationshipType) {
		t.Fatalf("AddRelationship invalid type err=%v, want %v", err, ErrRelationshipType)
	}
}

func TestNeo4jGraphStoreFindRelatedEntitiesValidation(t *testing.T) {
	ctx := context.Background()
	s := &Neo4jGraphStore{}

	if _, err := s.FindRelatedEntities(ctx, "", "e1", nil, 1, 10); !errors.Is(err, ErrUserIDRequired) {
		t.Fatalf("FindRelatedEntities empty user err=%v, want %v", err, ErrUserIDRequired)
	}
	if _, err := s.FindRelatedEntities(ctx, "u1", "", nil, 1, 10); !errors.Is(err, ErrEntityIDRequired) {
		t.Fatalf("FindRelatedEntities empty entity err=%v, want %v", err, ErrEntityIDRequired)
	}
	if _, err := s.FindRelatedEntities(ctx, "u1", "e1", []string{"BAD-TYPE"}, 1, 10); !errors.Is(err, ErrRelationshipType) {
		t.Fatalf("FindRelatedEntities invalid type err=%v, want %v", err, ErrRelationshipType)
	}
}

func TestNeo4jGraphStoreDeleteEntityValidation(t *testing.T) {
	ctx := context.Background()
	s := &Neo4jGraphStore{}

	if err := s.DeleteEntity(ctx, "", "e1"); !errors.Is(err, ErrUserIDRequired) {
		t.Fatalf("DeleteEntity empty user err=%v, want %v", err, ErrUserIDRequired)
	}
	if err := s.DeleteEntity(ctx, "u1", ""); !errors.Is(err, ErrEntityIDRequired) {
		t.Fatalf("DeleteEntity empty entity err=%v, want %v", err, ErrEntityIDRequired)
	}
}

func TestNeo4jGraphStoreClearAllValidation(t *testing.T) {
	ctx := context.Background()
	s := &Neo4jGraphStore{}

	if err := s.ClearAll(ctx, ""); !errors.Is(err, ErrUserIDRequired) {
		t.Fatalf("ClearAll empty user err=%v, want %v", err, ErrUserIDRequired)
	}
}

func TestNeo4jGraphStoreGetStatsValidation(t *testing.T) {
	ctx := context.Background()
	s := &Neo4jGraphStore{}

	if _, err := s.GetStats(ctx, ""); !errors.Is(err, ErrUserIDRequired) {
		t.Fatalf("GetStats empty user err=%v, want %v", err, ErrUserIDRequired)
	}
}

func TestNeo4jGraphStoreSearchEntitiesByNameValidation(t *testing.T) {
	ctx := context.Background()
	s := &Neo4jGraphStore{}

	if _, err := s.SearchEntitiesByName(ctx, "", "name", nil, 10); !errors.Is(err, ErrUserIDRequired) {
		t.Fatalf("SearchEntitiesByName empty user err=%v, want %v", err, ErrUserIDRequired)
	}
}

func TestNeo4jGraphStoreGetEntityRelationshipsValidation(t *testing.T) {
	ctx := context.Background()
	s := &Neo4jGraphStore{}

	if _, err := s.GetEntityRelationships(ctx, "", "e1"); !errors.Is(err, ErrUserIDRequired) {
		t.Fatalf("GetEntityRelationships empty user err=%v, want %v", err, ErrUserIDRequired)
	}
	if _, err := s.GetEntityRelationships(ctx, "u1", ""); !errors.Is(err, ErrEntityIDRequired) {
		t.Fatalf("GetEntityRelationships empty entity err=%v, want %v", err, ErrEntityIDRequired)
	}
}

func TestNeo4jGraphStoreDefaultOptions(t *testing.T) {
	cfg := defaultOptions()

	if cfg.uri != defaultURI {
		t.Fatalf("uri=%q, want %q", cfg.uri, defaultURI)
	}
	if cfg.username != defaultUsername {
		t.Fatalf("username=%q, want %q", cfg.username, defaultUsername)
	}
	if cfg.password != defaultPassword {
		t.Fatalf("password=%q, want %q", cfg.password, defaultPassword)
	}
	if cfg.database != defaultDatabase {
		t.Fatalf("database=%q, want %q", cfg.database, defaultDatabase)
	}
	if cfg.maxConnectionLifetime != defaultMaxConnectionLifetime {
		t.Fatalf("maxConnectionLifetime=%v, want %v", cfg.maxConnectionLifetime, defaultMaxConnectionLifetime)
	}
	if cfg.maxConnectionPoolSize != defaultMaxConnectionPoolSize {
		t.Fatalf("maxConnectionPoolSize=%d, want %d", cfg.maxConnectionPoolSize, defaultMaxConnectionPoolSize)
	}
	if cfg.connectionAcquisitionTimeout != defaultConnectionAcquisitionTimeout {
		t.Fatalf("connectionAcquisitionTimeout=%v, want %v", cfg.connectionAcquisitionTimeout, defaultConnectionAcquisitionTimeout)
	}
	if cfg.createIndexes != defaultCreateIndexes {
		t.Fatalf("createIndexes=%v, want %v", cfg.createIndexes, defaultCreateIndexes)
	}
	if cfg.toolRegistry == nil {
		t.Fatalf("toolRegistry=nil, want non-nil")
	}
	if cfg.enableTools == nil {
		t.Fatalf("enableTools=nil, want non-nil")
	}
}

func TestNeo4jGraphStoreOptionsApplied(t *testing.T) {
	registry := tools.NewRegistry(100) // Create a new registry with a different count to distinguish it from the default
	enabledTools := map[string]bool{"a": true}

	cfg := defaultOptions()
	WithURI("neo4j://example:7687")(&cfg)
	WithUsername("user")(&cfg)
	WithPassword("pass")(&cfg)
	WithDatabase("db")(&cfg)
	WithMaxConnectionLifetime(2 * time.Hour)(&cfg)
	WithMaxConnectionPoolSize(100)(&cfg)
	WithConnectionAcquisitionTimeout(30 * time.Second)(&cfg)
	WithCreateIndexes(false)(&cfg)
	WithToolRegistry(registry)(&cfg)
	WithEnabledTools(enabledTools)(&cfg)

	if cfg.uri != "neo4j://example:7687" {
		t.Fatalf("uri=%q, want neo4j://example:7687", cfg.uri)
	}
	if cfg.username != "user" {
		t.Fatalf("username=%q, want user", cfg.username)
	}
	if cfg.password != "pass" {
		t.Fatalf("password=%q, want pass", cfg.password)
	}
	if cfg.database != "db" {
		t.Fatalf("database=%q, want db", cfg.database)
	}
	if cfg.maxConnectionLifetime != 2*time.Hour {
		t.Fatalf("maxConnectionLifetime=%v, want %v", cfg.maxConnectionLifetime, 2*time.Hour)
	}
	if cfg.maxConnectionPoolSize != 100 {
		t.Fatalf("maxConnectionPoolSize=%d, want 100", cfg.maxConnectionPoolSize)
	}
	if cfg.connectionAcquisitionTimeout != 30*time.Second {
		t.Fatalf("connectionAcquisitionTimeout=%v, want %v", cfg.connectionAcquisitionTimeout, 30*time.Second)
	}
	if cfg.createIndexes {
		t.Fatalf("createIndexes=%v, want false", cfg.createIndexes)
	}
	if cfg.toolRegistry != registry {
		t.Fatalf("toolRegistry=%p, want %p", cfg.toolRegistry, registry)
	}
	if cfg.enableTools["a"] != true {
		t.Fatalf("enableTools=%v, want map with a=true", cfg.enableTools)
	}
}

func TestNeo4jGraphStoreNewInvalidURI(t *testing.T) {
	ctx := context.Background()
	store, err := NewNeo4jGraphStore(ctx, WithURI("   "))
	if err == nil {
		t.Fatalf("NewNeo4jGraphStore expected error for empty uri")
	}
	if store != nil {
		t.Fatalf("NewNeo4jGraphStore store=%v, want nil", store)
	}
	if !strings.Contains(err.Error(), "invalid URI") {
		t.Fatalf("NewNeo4jGraphStore err=%v, want contains %q", err, "invalid URI")
	}
}

func TestNeo4jGraphStorePackageLevelNotImplementedFunctions(t *testing.T) {
	ctx := context.Background()

	if err := Add(ctx, "u1", "content", nil, nil); !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("Add err=%v, want %v", err, ErrNotImplemented)
	}

	if got, err := Get(ctx, "u1", "query"); !errors.Is(err, ErrNotImplemented) || got != nil {
		t.Fatalf("Get got=%v err=%v, want nil and %v", got, err, ErrNotImplemented)
	}

	if err := Update(ctx, "u1", "query", "content", nil, nil); !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("Update err=%v, want %v", err, ErrNotImplemented)
	}

	if err := Delete(ctx, "u1", "query"); !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("Delete err=%v, want %v", err, ErrNotImplemented)
	}

	if err := Clear(ctx, "u1"); !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("Clear err=%v, want %v", err, ErrNotImplemented)
	}

	if got, err := List(ctx, "u1", 10); !errors.Is(err, ErrNotImplemented) || got != nil {
		t.Fatalf("List got=%v err=%v, want nil and %v", got, err, ErrNotImplemented)
	}
}
