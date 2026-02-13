package pgvector

import (
	"errors"
	"fmt"
	"strings"
	"xgc-agent/tools"
)

const (
	defaultHost     = "localhost"
	defaultPort     = 5432
	defaultUser     = "postgres"
	defaultPassword = "password"
	defaultDBName   = "pgvector_memory"
	defaultSSLMode  = "disable"

	defaultTable       = "memory_entries"
	defaultMemoryLimit = 100
)

var (
	ErrEmbeddingUnavailable    = errors.New("pgvector: embedding unavailable")
	ErrInvalidOption           = errors.New("pgvector: invalid option")
	ErrInvalidConnectionString = errors.New("pgvector: invalid connection string")
)

var defaultPgvectorOptions = pgvectorOptions{
	host:            defaultHost,
	port:            defaultPort,
	user:            defaultUser,
	password:        defaultPassword,
	dbname:          defaultDBName,
	sslMode:         defaultSSLMode,
	table:           defaultTable,
	memoryLimit:     defaultMemoryLimit,
	createExtension: true,
}

type pgvectorOptions struct {
	// PostgreSQL connection settings
	// The DSN (Data Source Name) for connecting to the PostgreSQL database.
	dsn string
	// The host of the PostgreSQL server.
	host string
	// The port of the PostgreSQL server.
	port int
	// The username for authenticating with the PostgreSQL server.
	user string
	// The password for authenticating with the PostgreSQL server.
	password string
	// The name of the PostgreSQL database to connect to.
	dbname string
	// Whether to automatically create the pgvector extension if it does not exist.
	createExtension bool
	// The SSL mode to use when connecting to PostgreSQL (e.g., "disable", "require", "verify-ca", "verify-full").
	sslMode string

	// The name of the table to store memory entries.
	table string
	// The maximum number of memory entries to store per user. If <= 0, there is no limit.
	memoryLimit int
	// The embedding dimension for the vectors.
	dimension int
	// Whether to use soft delete (setting deleted_at instead of hard deleting rows).
	softDelete bool
	// the registered tools to be added to the manager's tool registry
	toolRegistry *tools.Registry
	// the enabled tools
	enableTools map[string]bool
}

// PgvectorOption configures the pgvector store.
type PgvectorOption func(*pgvectorOptions)

func WithDSN(dsn string) PgvectorOption {
	return func(p *pgvectorOptions) {
		p.dsn = dsn
	}
}

func WithHost(host string) PgvectorOption {
	return func(p *pgvectorOptions) {
		p.host = host
	}
}

func WithPort(port int) PgvectorOption {
	return func(p *pgvectorOptions) {
		p.port = port
	}
}

func WithUser(user string) PgvectorOption {
	return func(p *pgvectorOptions) {
		p.user = user
	}
}

func WithPassword(password string) PgvectorOption {
	return func(p *pgvectorOptions) {
		p.password = password
	}
}

func WithDBName(dbname string) PgvectorOption {
	return func(p *pgvectorOptions) {
		p.dbname = dbname
	}
}

func WithExtension(enabled bool) PgvectorOption {
	return func(p *pgvectorOptions) {
		p.createExtension = enabled
	}
}

func WithSSLMode(sslMode string) PgvectorOption {
	return func(p *pgvectorOptions) {
		p.sslMode = sslMode
	}
}

func WithTable(table string) PgvectorOption {
	return func(p *pgvectorOptions) {
		p.table = table
	}
}

func WithMemoryLimit(limit int) PgvectorOption {
	return func(p *pgvectorOptions) {
		p.memoryLimit = limit
	}
}

func WithDimension(dimension int) PgvectorOption {
	return func(p *pgvectorOptions) {
		p.dimension = dimension
	}
}

func WithSoftDelete(enabled bool) PgvectorOption {
	return func(p *pgvectorOptions) {
		p.softDelete = enabled
	}
}

func WithToolRegistry(reg *tools.Registry) PgvectorOption {
	return func(p *pgvectorOptions) {
		p.toolRegistry = reg
	}
}

func WithEnabledTools(tools map[string]bool) PgvectorOption {
	return func(p *pgvectorOptions) {
		p.enableTools = tools
	}
}

// clone returns a copy of pgvectorOptions.
// It performs a shallow copy and makes a new map for `enableTools`
// to avoid accidental sharing of the internal map between instances.
func (p pgvectorOptions) clone() pgvectorOptions {
	cp := p
	if p.enableTools != nil {
		m := make(map[string]bool, len(p.enableTools))
		for k, v := range p.enableTools {
			m[k] = v
		}
		cp.enableTools = m
	}
	return cp
}

// buildConnString builds a PostgreSQL connection string from options.
func buildConnString(opts pgvectorOptions) string {
	// Default values.
	host := opts.host
	if host == "" {
		host = defaultHost
	}
	port := opts.port
	if port == 0 {
		port = defaultPort
	}
	database := opts.dbname
	if database == "" {
		database = defaultDBName
	}
	sslMode := opts.sslMode
	if sslMode == "" {
		sslMode = defaultSSLMode
	}

	// Build connection string.
	var conn strings.Builder
	fmt.Fprintf(&conn, "host=%s port=%d dbname=%s sslmode=%s",
		host, port, database, sslMode)
	if opts.user != "" {
		fmt.Fprintf(&conn, " user=%s", opts.user)
	}
	if opts.password != "" {
		fmt.Fprintf(&conn, " password=%s", opts.password)
	}
	return conn.String()
}
