package neo4jgraph

import (
	"errors"
	"time"
	"xgc-agent/tools"
)

const (
	defaultURI                          = "bolt://localhost:7687"
	defaultUsername                     = "neo4j"
	defaultPassword                     = "password"
	defaultDatabase                     = "neo4j"
	defaultMaxConnectionLifetime        = time.Hour
	defaultMaxConnectionPoolSize        = 50
	defaultConnectionAcquisitionTimeout = 60 * time.Second
	defaultCreateIndexes                = true
)

var (
	ErrInvalidURI       = errors.New("neo4j: invalid URI")
	ErrUserIDRequired   = errors.New("neo4j: user ID is required")
	ErrEntityIDRequired = errors.New("neo4j: entity ID is required")
	ErrNameRequired     = errors.New("neo4j: name is required")
	ErrTypeRequired     = errors.New("neo4j: type is required")
	ErrFromIDRequired   = errors.New("neo4j: from entity ID is required")
	ErrToIDRequired     = errors.New("neo4j: to entity ID is required")
	ErrRelationshipType = errors.New("neo4j: relationship type is invalid")
	ErrNotImplemented   = errors.New("neo4j: not implemented")
)

type neo4jOptions struct {
	uri      string
	username string
	password string
	database string
	// Connection pool settings
	maxConnectionLifetime time.Duration
	// Max number of connections in the pool. Default is 50.
	maxConnectionPoolSize int
	// Connection acquisition timeout. Default is 60 seconds.
	connectionAcquisitionTimeout time.Duration
	// Whether to create indexes on startup. Default is true.
	createIndexes bool

	// the registered tools to be added to the manager's tool registry
	toolRegistry *tools.Registry
	// the enabled tools
	enableTools map[string]bool
}

// Neo4jOption configures the Neo4j graph store.
type Neo4jOption func(*neo4jOptions)

// WithURI sets the Neo4j connection URI.
func WithURI(uri string) Neo4jOption {
	return func(o *neo4jOptions) {
		o.uri = uri
	}
}

// WithUsername sets the Neo4j username.
func WithUsername(username string) Neo4jOption {
	return func(o *neo4jOptions) {
		o.username = username
	}
}

// WithPassword sets the Neo4j password.
func WithPassword(password string) Neo4jOption {
	return func(o *neo4jOptions) {
		o.password = password
	}
}

// WithDatabase sets the Neo4j database name.
func WithDatabase(database string) Neo4jOption {
	return func(o *neo4jOptions) {
		o.database = database
	}
}

// WithMaxConnectionLifetime sets the maximum connection lifetime.
func WithMaxConnectionLifetime(d time.Duration) Neo4jOption {
	return func(o *neo4jOptions) {
		o.maxConnectionLifetime = d
	}
}

// WithMaxConnectionPoolSize sets the maximum connection pool size.
func WithMaxConnectionPoolSize(size int) Neo4jOption {
	return func(o *neo4jOptions) {
		o.maxConnectionPoolSize = size
	}
}

// WithConnectionAcquisitionTimeout sets the acquisition timeout for connections.
func WithConnectionAcquisitionTimeout(d time.Duration) Neo4jOption {
	return func(o *neo4jOptions) {
		o.connectionAcquisitionTimeout = d
	}
}

// WithCreateIndexes toggles index creation on startup.
func WithCreateIndexes(enabled bool) Neo4jOption {
	return func(o *neo4jOptions) {
		o.createIndexes = enabled
	}
}

// WithToolRegistry sets the tool registry for the graph store.
func WithToolRegistry(registry *tools.Registry) Neo4jOption {
	return func(o *neo4jOptions) {
		o.toolRegistry = registry
	}
}

// WithEnabledTools sets the enabled tools for the graph store.
func WithEnabledTools(tools map[string]bool) Neo4jOption {
	return func(o *neo4jOptions) {
		o.enableTools = tools
	}
}

func defaultOptions() neo4jOptions {
	return neo4jOptions{
		uri:                          defaultURI,
		username:                     defaultUsername,
		password:                     defaultPassword,
		database:                     defaultDatabase,
		maxConnectionLifetime:        defaultMaxConnectionLifetime,
		maxConnectionPoolSize:        defaultMaxConnectionPoolSize,
		connectionAcquisitionTimeout: defaultConnectionAcquisitionTimeout,
		createIndexes:                defaultCreateIndexes,
		toolRegistry:                 tools.DefaultRegistry,
		enableTools:                  make(map[string]bool),
	}
}
