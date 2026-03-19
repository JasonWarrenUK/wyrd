package types

// GraphIndex is the in-memory graph built from the flat-file store.
// It supports O(1) edge lookups per node, property filtering, and traversal.
// The index rebuilds on file change (via the filesystem watcher) and on sync.
type GraphIndex interface {
	// GetNode returns a node by UUID. Returns NotFoundError if absent.
	GetNode(id string) (*Node, error)

	// GetEdge returns an edge by UUID. Returns NotFoundError if absent.
	GetEdge(id string) (*Edge, error)

	// AllNodes returns every node in the index.
	AllNodes() []*Node

	// AllEdges returns every edge in the index.
	AllEdges() []*Edge

	// EdgesFrom returns all edges whose From field equals nodeID.
	EdgesFrom(nodeID string) []*Edge

	// EdgesTo returns all edges whose To field equals nodeID.
	EdgesTo(nodeID string) []*Edge

	// NodesByType returns all nodes that include typeName in their Types slice.
	NodesByType(typeName string) []*Node
}

// StoreFS is the file-system facade for reading and writing graph entities.
// All mutations go through StoreFS, never through direct file writes.
type StoreFS interface {
	// ReadNode reads a node from disk by UUID.
	ReadNode(id string) (*Node, error)

	// WriteNode persists a node to disk atomically.
	WriteNode(node *Node) error

	// ReadEdge reads an edge from disk by UUID.
	ReadEdge(id string) (*Edge, error)

	// WriteEdge persists an edge to disk atomically.
	WriteEdge(edge *Edge) error

	// DeleteEdge removes an edge file from disk.
	// Nodes are never deleted; use archive instead.
	DeleteEdge(id string) error

	// ReadTemplate reads a template definition by type name.
	ReadTemplate(typeName string) (*Template, error)

	// AllTemplates returns all templates found in the store.
	AllTemplates() ([]*Template, error)

	// ReadView reads a saved view by name.
	ReadView(name string) (*SavedView, error)

	// AllViews returns all saved views found in the store.
	AllViews() ([]*SavedView, error)

	// ReadRitual reads a ritual definition by name.
	ReadRitual(name string) (*Ritual, error)

	// AllRituals returns all rituals found in the store.
	AllRituals() ([]*Ritual, error)

	// ReadTheme reads a theme definition by name.
	ReadTheme(name string) (*Theme, error)

	// ReadConfig reads the application configuration.
	ReadConfig() (*Config, error)

	// WriteConfig persists the application configuration.
	WriteConfig(cfg *Config) error

	// StorePath returns the absolute path to the /store root.
	StorePath() string
}

// PluginStore extends StoreFS with plugin-specific operations.
type PluginStore interface {
	StoreFS

	// ReadPluginManifest reads a plugin's manifest by plugin name.
	ReadPluginManifest(name string) (*PluginManifest, error)

	// AllPluginManifests returns all plugin manifests found in the store.
	AllPluginManifests() ([]*PluginManifest, error)

	// ReadPluginConfig reads a plugin's user configuration as a raw map.
	ReadPluginConfig(name string) (map[string]interface{}, error)
}

// QueryResult holds the output of a query execution.
type QueryResult struct {
	// Rows is the result set. Each row maps column names to values.
	Rows []map[string]interface{}

	// Columns is the ordered list of column names, in the order they appear
	// in the RETURN clause.
	Columns []string
}

// QueryRunner executes read-only Cypher subset queries against a GraphIndex.
type QueryRunner interface {
	// Run executes the query and returns results. Returns QueryError on failure.
	// Variables like $today and $now are resolved against the provided clock.
	Run(query string, clock Clock) (*QueryResult, error)
}
