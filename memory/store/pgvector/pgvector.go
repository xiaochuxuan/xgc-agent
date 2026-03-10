package pgvector

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"xgc-agent/embedding"
	"xgc-agent/memory"

	"github.com/lib/pq"
	"github.com/pgvector/pgvector-go"
)

// PgvectorStore stores memory entries with pgvector embeddings.
type PgvectorStore struct {
	db      Client
	options pgvectorOptions
	table   string
	embed   embedding.EmbeddingService
}

// NewPgvectorStore creates a PgvectorStore from a DSN.
func NewPgvectorStore(embed embedding.EmbeddingService, dimension int, opts ...PgvectorOption) (*PgvectorStore, error) {
	options := defaultPgvectorOptions.clone()
	for _, opt := range opts {
		opt(&options)
	}

	// check if embedding is available before connecting to the database
	if embed == nil {
		return nil, ErrEmbeddingUnavailable
	}

	var connString string
	// Priority for connection string: DSN > direct connection parameters > instance defaults
	if options.dsn != "" {
		// If DSN is provided, use it directly.
		connString = options.dsn
	} else {
		// Otherwise, build connection string from direct parameters or instance defaults.
		connString = buildConnString(options)
	}

	// Create database client using the default client builder.
	db, err := DefaultClientBuilder(context.Background(), connString)
	if err != nil {
		return nil, err
	}

	store := &PgvectorStore{
		db:      db,
		options: options,
		table:   options.table,
		embed:   embed,
	}
	return store, nil
}

// Init creates the necessary table and extension for pgvector storage.
func (s *PgvectorStore) Init(ctx context.Context) error {
	if s.options.createExtension {
		if _, err := s.db.ExecContext(ctx, `CREATE EXTENSION IF NOT EXISTS vector`); err != nil {
			return err
		}
	}
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			memory_id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			content TEXT NOT NULL,
			topics TEXT[] NOT NULL DEFAULT '{}',
			metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
			embedding VECTOR(%d) NOT NULL,
			deleted_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
			);
			CREATE INDEX IF NOT EXISTS %s_user_updated_idx ON %s (user_id, updated_at DESC, created_at DESC);
			CREATE INDEX IF NOT EXISTS %s_embedding_idx ON %s USING ivfflat (embedding vector_cosine_ops);
		`, s.table,
		s.options.dimension,
		s.table,
		s.table,
		s.table,
		s.table)
	_, err := s.db.ExecContext(ctx, query)
	return err
}

func (s *PgvectorStore) Add(ctx context.Context, userID string, content string,
	topics []string, metadata map[string]any) error {

	if userID == "" {
		return memory.ErrUserIDRequired
	}
	if s.embed == nil {
		return ErrEmbeddingUnavailable
	}

	// generate embedding for the content
	embeddings, err := s.embed.Embed(ctx, []string{content})
	if err != nil {
		return fmt.Errorf("generate embedding failed: %w", err)
	}
	embedding := embeddings[0]
	if len(embedding) != s.options.dimension {
		return fmt.Errorf("embedding dimension mismatch: expected %d, got %d",
			s.options.dimension, len(embedding))
	}

	// create memory entry
	now := time.Now()
	mem := &memory.Memory{
		Content:  content,
		Topics:   topics,
		Metadata: metadata,
	}
	memoryID := memory.GenerateMemoryID(mem, userID)

	// TODO: metadata
	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata failed: %w", err)
	}

	// convert embedding to pgvector format
	vec := pgvector.NewVector(embedding)

	// generate SQL query with upsert and memory limit enforcement
	var insertQuery string
	args := []any{
		memoryID,
		userID,
		content,
		pq.Array(topics),
		metaJSON,
		vec,
		now,
		now,
	}
	if s.options.memoryLimit > 0 {
		// If memory limit is set, we need to check the count before inserting.
		// We can use an upsert with a CTE to ensure atomicity.
		deletedFilter := ""
		if s.options.softDelete {
			deletedFilter = " AND deleted_at IS NULL"
		}
		insertQuery = fmt.Sprintf(
			"WITH existing AS ("+
				"SELECT 1 FROM %s "+
				"WHERE memory_id = $1 AND user_id = $2%s"+
				"), cnt AS (SELECT COUNT(*) AS c FROM %s "+
				"WHERE user_id = $2%s"+
				") "+
				"INSERT INTO %s "+
				"(memory_id, user_id, content, topics, metadata, embedding, created_at, updated_at) "+
				"SELECT $1, $2, $3, $4, $5, $6, $7, $8 "+
				"WHERE (EXISTS (SELECT 1 FROM existing) OR "+
				"(SELECT c FROM cnt) < $9) "+
				"ON CONFLICT (memory_id) DO UPDATE SET "+
				"content = EXCLUDED.content, "+
				"topics = EXCLUDED.topics, "+
				"metadata = EXCLUDED.metadata, "+
				"embedding = EXCLUDED.embedding, "+
				"deleted_at = NULL, "+
				"updated_at = EXCLUDED.updated_at",
			s.table,
			deletedFilter,
			s.table,
			deletedFilter,
			s.table,
		)
		args = append(args, s.options.memoryLimit)
	} else {
		insertQuery = fmt.Sprintf(
			"INSERT INTO %s "+
				"(memory_id, user_id, content, topics, metadata, embedding, created_at, updated_at) "+
				"VALUES ($1, $2, $3, $4, $5, $6, $7, $8) "+
				"ON CONFLICT (memory_id) DO UPDATE SET "+
				"content = EXCLUDED.content, "+
				"topics = EXCLUDED.topics, "+
				"metadata = EXCLUDED.metadata, "+
				"embedding = EXCLUDED.embedding, "+
				"deleted_at = NULL, "+
				"updated_at = EXCLUDED.updated_at",
			s.table,
		)
	}

	// execute the query
	res, err := s.db.ExecContext(ctx, insertQuery, args...)

	if err != nil {
		return fmt.Errorf("insert memory failed: %w", err)
	}
	if s.options.memoryLimit > 0 {
		// if memory limit is set, we need to check if the row was inserted or updated
		affected, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("check affected rows failed: %w", err)
		}
		if affected == 0 {
			return fmt.Errorf("memory limit exceeded for user %s, limit: %d",
				userID, s.options.memoryLimit)
		}
	}
	return nil
}

func (s *PgvectorStore) Get(ctx context.Context, userID string, memoryID string) (*memory.MemoryItem, error) {
	if userID == "" {
		return nil, memory.ErrUserIDRequired
	}
	if memoryID == "" {
		return nil, memory.ErrMemoryIDRequired
	}

	deletedFilter := ""
	if s.options.softDelete {
		deletedFilter = " AND deleted_at IS NULL"
	}
	query := fmt.Sprintf(
		"SELECT memory_id, user_id, content, topics, metadata, created_at, updated_at "+
			"FROM %s WHERE user_id = $1 AND memory_id = $2%s",
		s.table,
		deletedFilter,
	)

	var entry *memory.MemoryItem
	err := s.db.Query(ctx, func(rows *sql.Rows) error {
		if rows.Next() {
			var scanErr error
			entry, scanErr = scanMemoryEntry(rows)
			if scanErr != nil {
				return scanErr
			}
		}
		return nil
	}, query, userID, memoryID)
	if err != nil {
		return nil, fmt.Errorf("get memory failed: %w", err)
	}
	if entry == nil {
		return nil, memory.ErrNotFound
	}
	return entry, nil
}

func (s *PgvectorStore) Update(ctx context.Context, userID string, memoryID string, content string,
	topic []string, metadata map[string]any) error {

	if userID == "" {
		return memory.ErrUserIDRequired
	}
	if memoryID == "" {
		return memory.ErrMemoryIDRequired
	}

	// generate new embedding for the content
	embeddings, err := s.embed.Embed(ctx, []string{content})
	if err != nil {
		return fmt.Errorf("generate embedding failed: %w", err)
	}
	embedding := embeddings[0]
	if len(embedding) != s.options.dimension {
		return fmt.Errorf("embedding dimension mismatch: expected %d, got %d",
			s.options.dimension, len(embedding))
	}

	// convert embedding to pgvector format
	vec := pgvector.NewVector(embedding)

	meta := metadata
	if meta == nil {
		meta = map[string]any{}
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal metadata failed: %w", err)
	}
	now := time.Now()

	// generate SQL query with upsert and memory limit enforcement
	deletedFilter := ""
	if s.options.softDelete {
		deletedFilter = " AND deleted_at IS NULL"
	}
	updateQuery := fmt.Sprintf(
		"UPDATE %s "+
			"SET content = $1, topics = $2, metadata = $3, embedding = $4, updated_at = $5 "+
			"WHERE user_id = $6 AND memory_id = $7%s",
		s.table,
		deletedFilter,
	)

	// execute the query
	res, err := s.db.ExecContext(ctx, updateQuery,
		content, pq.Array(topic), metaJSON, vec, now, userID, memoryID)
	if err != nil {
		return fmt.Errorf("update memory failed: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check affected rows failed: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("memory entry not found for user %s and memory ID %s", userID, memoryID)
	}
	return nil
}

func (s *PgvectorStore) Delete(ctx context.Context, userID string, memoryID string) error {

	if userID == "" {
		return memory.ErrUserIDRequired
	}
	if memoryID == "" {
		return memory.ErrMemoryIDRequired
	}

	var (
		query string
		args  []any
	)
	if s.options.softDelete {
		now := time.Now()
		query = fmt.Sprintf(
			"UPDATE %s SET deleted_at = $1, updated_at = $2 "+
				"WHERE user_id = $3 AND memory_id = $4 AND deleted_at IS NULL",
			s.table,
		)
		args = []any{now, now, userID, memoryID}
	} else {
		query = fmt.Sprintf(
			"DELETE FROM %s WHERE user_id = $1 AND memory_id = $2",
			s.table,
		)
		args = []any{userID, memoryID}
	}

	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}

	return nil
}

func (s *PgvectorStore) Clear(ctx context.Context, userID string) error {

	if userID == "" {
		return memory.ErrUserIDRequired
	}

	if s.options.softDelete {
		now := time.Now()
		query := fmt.Sprintf(
			"UPDATE %s SET deleted_at = $1, updated_at = $2 "+
				"WHERE user_id = $3 AND deleted_at IS NULL",
			s.table,
		)
		_, err := s.db.ExecContext(ctx, query, now, now, userID)
		return err
	} else {
		query := fmt.Sprintf(
			"DELETE FROM %s WHERE user_id = $1",
			s.table,
		)
		_, err := s.db.ExecContext(ctx, query, userID)
		return err
	}
}

func (s *PgvectorStore) List(ctx context.Context, userID string, limit int) ([]*memory.MemoryItem, error) {
	if userID == "" {
		return nil, memory.ErrUserIDRequired
	}

	deletedFilter := ""
	if s.options.softDelete {
		deletedFilter = " AND deleted_at IS NULL"
	}
	listQuery := fmt.Sprintf(
		"SELECT memory_id, user_id, content, topics, metadata, created_at, updated_at "+
			"FROM %s WHERE user_id = $1%s "+
			"ORDER BY updated_at DESC, created_at DESC",
		s.table,
		deletedFilter,
	)
	args := []any{userID}
	if limit > 0 {
		listQuery += " LIMIT $2"
		args = append(args, limit)
	}

	entries := make([]*memory.MemoryItem, 0)
	err := s.db.Query(ctx, func(rows *sql.Rows) error {
		for rows.Next() {
			entry, err := scanMemoryEntry(rows)
			if err != nil {
				return err
			}
			entries = append(entries, entry)
		}
		return nil
	}, listQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("list memories failed: %w", err)
	}

	return entries, nil
}

// scanMemoryEntry scans a memory entry from database rows.
func scanMemoryEntry(rows *sql.Rows) (*memory.MemoryItem, error) {
	var (
		memoryID  string
		userID    string
		content   string
		topics    pq.StringArray
		metaRaw   []byte
		createdAt time.Time
		updatedAt time.Time
	)

	if err := rows.Scan(
		&memoryID, &userID, &content, &topics, &metaRaw, &createdAt, &updatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan memory entry failed: %w", err)
	}

	metadata := make(map[string]any)
	if err := json.Unmarshal(metaRaw, &metadata); err != nil {
		return nil, fmt.Errorf("unmarshal metadata failed: %w", err)
	}

	return &memory.MemoryItem{
		MemoryID: memoryID,
		UserID:   userID,
		Memory: &memory.Memory{
			Content:  content,
			Topics:   []string(topics),
			Metadata: metadata,
		},
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}
