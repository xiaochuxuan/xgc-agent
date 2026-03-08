package session

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var ErrSessionNotFound = errors.New("session not found")

// FileStore persists sessions as json blob files.
type FileStore struct {
	Dir string
}

func NewFileStore(dir string) *FileStore {
	return &FileStore{Dir: dir}
}

func (f *FileStore) Save(ctx context.Context, sessionID string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if f == nil {
		return fmt.Errorf("file store is nil")
	}
	if f.Dir == "" {
		return fmt.Errorf("file store dir is empty")
	}
	if sessionID == "" {
		return fmt.Errorf("session id is required")
	}

	if err := os.MkdirAll(f.Dir, 0o755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}

	path := filepath.Join(f.Dir, sessionID+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}
	return nil
}

func (f *FileStore) Load(ctx context.Context, sessionID string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f == nil {
		return nil, fmt.Errorf("file store is nil")
	}
	if f.Dir == "" {
		return nil, fmt.Errorf("file store dir is empty")
	}
	if sessionID == "" {
		return nil, fmt.Errorf("session id is required")
	}

	path := filepath.Join(f.Dir, sessionID+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("read session file: %w", err)
	}
	return b, nil
}

// SQLiteStore persists sessions in sqlite table.
type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(dsn string) (*SQLiteStore, error) {
	if dsn == "" {
		return nil, fmt.Errorf("sqlite dsn is required")
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) init() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("sqlite store is nil")
	}
	const ddl = `
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    data BLOB NOT NULL,
    updated_at INTEGER NOT NULL
);`
	if _, err := s.db.Exec(ddl); err != nil {
		return fmt.Errorf("create sessions table: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Save(ctx context.Context, sessionID string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.db == nil {
		return fmt.Errorf("sqlite store is nil")
	}
	if sessionID == "" {
		return fmt.Errorf("session id is required")
	}

	const upsert = `
INSERT INTO sessions(id, data, updated_at)
VALUES (?, ?, unixepoch())
ON CONFLICT(id) DO UPDATE SET
    data = excluded.data,
    updated_at = excluded.updated_at;`
	if _, err := s.db.ExecContext(ctx, upsert, sessionID, data); err != nil {
		return fmt.Errorf("save session: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Load(ctx context.Context, sessionID string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("sqlite store is nil")
	}
	if sessionID == "" {
		return nil, fmt.Errorf("session id is required")
	}

	const query = `SELECT data FROM sessions WHERE id = ? LIMIT 1;`
	var data []byte
	if err := s.db.QueryRowContext(ctx, query, sessionID).Scan(&data); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("load session: %w", err)
	}
	return data, nil
}

func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}
