package pgvector

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
)

type mockEmbed struct {
	dim int
}

func (m mockEmbed) Provider() string {
	return "mock"
}

func (m mockEmbed) Dimension() int {
	return m.dim
}

func (m mockEmbed) Embed(_ context.Context, texts []string) ([][]float32, error) {
	vecs := make([][]float32, len(texts))
	for i := range texts {
		vecs[i] = make([]float32, m.dim)
	}
	return vecs, nil
}

func TestPgvectorStoreInit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	store := &PgvectorStore{
		db:      &sqlClient{db: db},
		options: pgvectorOptions{createExtension: true, dimension: 3},
		table:   "memory_entries",
	}

	mock.ExpectExec("CREATE EXTENSION IF NOT EXISTS vector").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS memory_entries").WillReturnResult(sqlmock.NewResult(0, 0))

	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestPgvectorStoreGetFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	store := &PgvectorStore{
		db:      &sqlClient{db: db},
		options: pgvectorOptions{softDelete: false},
		table:   "memory_entries",
	}

	createdAt := time.Now().Add(-time.Minute)
	updatedAt := time.Now()

	rows := sqlmock.NewRows([]string{"memory_id", "user_id", "content", "topics", "metadata", "created_at", "updated_at"}).
		AddRow("m1", "u1", "hello", pq.StringArray{"t1", "t2"}, []byte(`{"k":"v"}`), createdAt, updatedAt)

	mock.ExpectQuery("SELECT memory_id, user_id, content, topics, metadata, created_at, updated_at").
		WithArgs("u1", "m1").
		WillReturnRows(rows)

	entry, err := store.Get(context.Background(), "u1", "m1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if entry == nil || entry.MemoryID != "m1" || entry.UserID != "u1" {
		t.Fatalf("Get: unexpected entry: %+v", entry)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestPgvectorStoreGetNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	store := &PgvectorStore{
		db:      &sqlClient{db: db},
		options: pgvectorOptions{softDelete: false},
		table:   "memory_entries",
	}

	rows := sqlmock.NewRows([]string{"memory_id", "user_id", "content", "topics", "metadata", "created_at", "updated_at"})
	mock.ExpectQuery("SELECT memory_id, user_id, content, topics, metadata, created_at, updated_at").
		WithArgs("u1", "m1").
		WillReturnRows(rows)

	entry, err := store.Get(context.Background(), "u1", "m1")
	if err == nil {
		t.Fatalf("expected error, got nil entry=%+v", entry)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestPgvectorStoreAdd(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	store := &PgvectorStore{
		db:      &sqlClient{db: db},
		options: pgvectorOptions{dimension: 3, memoryLimit: 0},
		table:   "memory_entries",
		embed:   mockEmbed{dim: 3},
	}

	mock.ExpectExec("INSERT INTO memory_entries").WillReturnResult(sqlmock.NewResult(0, 1))

	if err := store.Add(context.Background(), "u1", "hello", []string{"t1"}, map[string]any{"k": "v"}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestPgvectorStoreAddMemoryLimitExceeded(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	store := &PgvectorStore{
		db:      &sqlClient{db: db},
		options: pgvectorOptions{dimension: 3, memoryLimit: 1},
		table:   "memory_entries",
		embed:   mockEmbed{dim: 3},
	}

	mock.ExpectExec("WITH existing AS").WillReturnResult(sqlmock.NewResult(0, 0))

	if err := store.Add(context.Background(), "u1", "hello", []string{"t1"}, map[string]any{"k": "v"}); err == nil {
		t.Fatalf("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestPgvectorStoreUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	store := &PgvectorStore{
		db:      &sqlClient{db: db},
		options: pgvectorOptions{dimension: 3, softDelete: false},
		table:   "memory_entries",
		embed:   mockEmbed{dim: 3},
	}

	mock.ExpectExec("UPDATE memory_entries").WillReturnResult(sqlmock.NewResult(0, 1))

	if err := store.Update(context.Background(), "u1", "m1", "hello", []string{"t1"}, map[string]any{"k": "v"}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestPgvectorStoreUpdateNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	store := &PgvectorStore{
		db:      &sqlClient{db: db},
		options: pgvectorOptions{dimension: 3, softDelete: false},
		table:   "memory_entries",
		embed:   mockEmbed{dim: 3},
	}

	mock.ExpectExec("UPDATE memory_entries").WillReturnResult(sqlmock.NewResult(0, 0))

	if err := store.Update(context.Background(), "u1", "m1", "hello", []string{"t1"}, map[string]any{"k": "v"}); err == nil {
		t.Fatalf("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestPgvectorStoreDelete(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	store := &PgvectorStore{
		db:      &sqlClient{db: db},
		options: pgvectorOptions{softDelete: false},
		table:   "memory_entries",
	}

	mock.ExpectExec("DELETE FROM memory_entries").WithArgs("u1", "m1").WillReturnResult(sqlmock.NewResult(0, 1))

	if err := store.Delete(context.Background(), "u1", "m1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestPgvectorStoreClear(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	store := &PgvectorStore{
		db:      &sqlClient{db: db},
		options: pgvectorOptions{softDelete: false},
		table:   "memory_entries",
	}

	mock.ExpectExec("DELETE FROM memory_entries").WithArgs("u1").WillReturnResult(sqlmock.NewResult(0, 1))

	if err := store.Clear(context.Background(), "u1"); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestPgvectorStoreList(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	store := &PgvectorStore{
		db:      &sqlClient{db: db},
		options: pgvectorOptions{softDelete: false},
		table:   "memory_entries",
	}

	createdAt := time.Now().Add(-time.Minute)
	updatedAt := time.Now()
	rows := sqlmock.NewRows([]string{"memory_id", "user_id", "content", "topics", "metadata", "created_at", "updated_at"}).
		AddRow("m1", "u1", "hello", pq.StringArray{"t1"}, []byte(`{"k":"v"}`), createdAt, updatedAt)

	mock.ExpectQuery("SELECT memory_id, user_id, content, topics, metadata, created_at, updated_at").
		WithArgs("u1", 1).
		WillReturnRows(rows)

	entries, err := store.List(context.Background(), "u1", 1)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 || entries[0].MemoryID != "m1" {
		t.Fatalf("List: unexpected entries: %+v", entries)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}
