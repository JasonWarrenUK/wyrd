package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// Store is the top-level facade implementing StoreFS and PluginStore.
// It manages nodes, edges, templates, views, rituals, themes, and config
// on the local filesystem as JSONC files.
type Store struct {
	path       string
	clock      types.Clock
	index      *memIndex
	watcher    *fsnotify.Watcher
	templates  map[string]*types.Template
	templateMu sync.RWMutex
}

// New creates a Store rooted at the given path using the provided clock.
// It scans existing nodes and edges to build the initial in-memory index,
// then starts a filesystem watcher for incremental updates.
func New(path string, clock types.Clock) (*Store, error) {
	s := &Store{
		path:      path,
		clock:     clock,
		index:     newMemIndex(),
		templates: make(map[string]*types.Template),
	}

	if err := s.ensureDirs(); err != nil {
		return nil, fmt.Errorf("initialising store at %s: %w", path, err)
	}

	if err := s.buildIndex(); err != nil {
		return nil, fmt.Errorf("building index for %s: %w", path, err)
	}

	if err := s.startWatcher(); err != nil {
		// Watcher failure is non-fatal — log and continue without live updates.
		fmt.Printf("warning: file watcher could not be started: %v\n", err)
	}

	return s, nil
}

// ensureDirs creates the store directory structure if it doesn't exist.
func (s *Store) ensureDirs() error {
	dirs := []string{
		filepath.Join(s.path, "nodes"),
		filepath.Join(s.path, "edges"),
		filepath.Join(s.path, "templates"),
		filepath.Join(s.path, "views"),
		filepath.Join(s.path, "rituals"),
		filepath.Join(s.path, "themes"),
		filepath.Join(s.path, "plugins"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}
	return nil
}

// buildIndex scans all node and edge files and populates the in-memory index.
func (s *Store) buildIndex() error {
	nodesDir := filepath.Join(s.path, "nodes")
	entries, err := readdirNames(nodesDir)
	if err != nil {
		return nil // Empty or missing is fine on first run.
	}
	for _, name := range entries {
		if filepath.Ext(name) != ".jsonc" {
			continue
		}
		id := name[:len(name)-len(".jsonc")]
		node, err := s.ReadNode(id)
		if err != nil {
			continue
		}
		s.index.upsertNode(node)
	}

	edgesDir := filepath.Join(s.path, "edges")
	edgeEntries, err := readdirNames(edgesDir)
	if err != nil {
		return nil
	}
	for _, name := range edgeEntries {
		if filepath.Ext(name) != ".jsonc" {
			continue
		}
		id := name[:len(name)-len(".jsonc")]
		edge, err := s.ReadEdge(id)
		if err != nil {
			continue
		}
		s.index.upsertEdge(edge)
	}

	return nil
}

// Index returns the in-memory graph index.
func (s *Store) Index() types.GraphIndex {
	return s.index
}

// StorePath returns the absolute path to the /store root.
func (s *Store) StorePath() string {
	return s.path
}

// Close shuts down the filesystem watcher.
func (s *Store) Close() error {
	if s.watcher != nil {
		return s.watcher.Close()
	}
	return nil
}

// WriteNode persists a node to disk atomically.
func (s *Store) WriteNode(node *types.Node) error {
	createdStr := node.Created.UTC().Format("2006-01-02T15:04:05Z")
	modifiedStr := node.Modified.UTC().Format("2006-01-02T15:04:05Z")

	// Build the nested date object from node.Date.
	// node.Date.Created/Modified mirror node.Created/Modified.
	dateObj := map[string]interface{}{
		"created":  createdStr,
		"modified": modifiedStr,
	}
	if node.Date.Due != nil {
		dateObj["due"] = node.Date.Due.UTC().Format(time.RFC3339)
	}
	if node.Date.About != nil {
		dateObj["about"] = node.Date.About.UTC().Format(time.RFC3339)
	}
	if node.Date.Schedule != nil {
		dateObj["schedule"] = node.Date.Schedule.UTC().Format(time.RFC3339)
	}
	if node.Date.Start != nil {
		dateObj["start"] = node.Date.Start.UTC().Format(time.RFC3339)
	}
	if node.Date.SnoozeUntil != nil {
		dateObj["snooze_until"] = node.Date.SnoozeUntil.UTC().Format(time.RFC3339)
	}

	raw := map[string]interface{}{
		"id":       node.ID,
		"body":     node.Body,
		"types":    node.Types,
		"created":  createdStr,
		"modified": modifiedStr,
		"date":     dateObj,
	}
	if node.Title != "" {
		raw["title"] = node.Title
	}
	if node.Source != nil {
		raw["source"] = node.Source
	}
	for k, v := range node.Properties {
		raw[k] = v
	}
	return writeJSONC(s.nodePath(node.ID), raw)
}

// WriteEdge persists an edge to disk atomically.
func (s *Store) WriteEdge(edge *types.Edge) error {
	raw := map[string]interface{}{
		"id":      edge.ID,
		"type":    edge.Type,
		"from":    edge.From,
		"to":      edge.To,
		"created": edge.Created.UTC().Format("2006-01-02T15:04:05Z"),
	}
	for k, v := range edge.Properties {
		raw[k] = v
	}
	return writeJSONC(s.edgePath(edge.ID), raw)
}

// ReadView reads a saved view by name from disk.
func (s *Store) ReadView(name string) (*types.SavedView, error) {
	path := filepath.Join(s.path, "views", name+".jsonc")
	data, err := readJSONC(path)
	if err != nil {
		if isNotExist(err) {
			return nil, &types.NotFoundError{Kind: "view", ID: name}
		}
		return nil, fmt.Errorf("reading view %s: %w", name, err)
	}
	var view types.SavedView
	if err := unmarshalJSON(data, &view); err != nil {
		return nil, &types.ParseError{Source: name, Message: err.Error()}
	}
	return &view, nil
}

// AllViews returns all saved views found in the store.
func (s *Store) AllViews() ([]*types.SavedView, error) {
	dir := filepath.Join(s.path, "views")
	entries, err := readdirNames(dir)
	if err != nil {
		return nil, nil
	}
	var views []*types.SavedView
	for _, name := range entries {
		if filepath.Ext(name) != ".jsonc" {
			continue
		}
		viewName := name[:len(name)-len(".jsonc")]
		v, err := s.ReadView(viewName)
		if err != nil {
			continue
		}
		views = append(views, v)
	}
	return views, nil
}

// ReadRitual reads a ritual definition by name from disk.
func (s *Store) ReadRitual(name string) (*types.Ritual, error) {
	path := filepath.Join(s.path, "rituals", name+".jsonc")
	data, err := readJSONC(path)
	if err != nil {
		if isNotExist(err) {
			return nil, &types.NotFoundError{Kind: "ritual", ID: name}
		}
		return nil, fmt.Errorf("reading ritual %s: %w", name, err)
	}
	var ritual types.Ritual
	if err := unmarshalJSON(data, &ritual); err != nil {
		return nil, &types.ParseError{Source: name, Message: err.Error()}
	}
	return &ritual, nil
}

// AllRituals returns all ritual definitions found in the store.
func (s *Store) AllRituals() ([]*types.Ritual, error) {
	dir := filepath.Join(s.path, "rituals")
	entries, err := readdirNames(dir)
	if err != nil {
		return nil, nil
	}
	var rituals []*types.Ritual
	for _, name := range entries {
		if filepath.Ext(name) != ".jsonc" {
			continue
		}
		ritualName := name[:len(name)-len(".jsonc")]
		r, err := s.ReadRitual(ritualName)
		if err != nil {
			continue
		}
		rituals = append(rituals, r)
	}
	return rituals, nil
}

// ReadTheme reads a theme definition by name from disk.
func (s *Store) ReadTheme(name string) (*types.Theme, error) {
	path := filepath.Join(s.path, "themes", name+".jsonc")
	data, err := readJSONC(path)
	if err != nil {
		if isNotExist(err) {
			return nil, &types.NotFoundError{Kind: "theme", ID: name}
		}
		return nil, fmt.Errorf("reading theme %s: %w", name, err)
	}
	var theme types.Theme
	if err := unmarshalJSON(data, &theme); err != nil {
		return nil, &types.ParseError{Source: name, Message: err.Error()}
	}
	return &theme, nil
}

// ReadConfig reads the application configuration from disk.
func (s *Store) ReadConfig() (*types.Config, error) {
	path := filepath.Join(s.path, "..", "config.jsonc")
	data, err := readJSONC(path)
	if err != nil {
		if isNotExist(err) {
			return &types.Config{MaxTraversalDepth: 5}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg types.Config
	if err := unmarshalJSON(data, &cfg); err != nil {
		return nil, &types.ParseError{Source: "config.jsonc", Message: err.Error()}
	}
	return &cfg, nil
}

// WriteConfig persists the application configuration.
func (s *Store) WriteConfig(cfg *types.Config) error {
	path := filepath.Join(s.path, "..", "config.jsonc")
	return writeJSONC(path, cfg)
}

// ReadPluginManifest reads a plugin's manifest by plugin name.
func (s *Store) ReadPluginManifest(name string) (*types.PluginManifest, error) {
	path := filepath.Join(s.path, "plugins", name, "manifest.jsonc")
	data, err := readJSONC(path)
	if err != nil {
		if isNotExist(err) {
			return nil, &types.NotFoundError{Kind: "plugin", ID: name}
		}
		return nil, fmt.Errorf("reading plugin manifest %s: %w", name, err)
	}
	var manifest types.PluginManifest
	if err := unmarshalJSON(data, &manifest); err != nil {
		return nil, &types.ParseError{Source: name, Message: err.Error()}
	}
	return &manifest, nil
}

// AllPluginManifests returns all plugin manifests found in the store.
func (s *Store) AllPluginManifests() ([]*types.PluginManifest, error) {
	dir := filepath.Join(s.path, "plugins")
	entries, err := readdirNames(dir)
	if err != nil {
		return nil, nil
	}
	var manifests []*types.PluginManifest
	for _, name := range entries {
		m, err := s.ReadPluginManifest(name)
		if err != nil {
			continue
		}
		manifests = append(manifests, m)
	}
	return manifests, nil
}

// ReadPluginConfig reads a plugin's user configuration as a raw map.
func (s *Store) ReadPluginConfig(name string) (map[string]interface{}, error) {
	path := filepath.Join(s.path, "plugins", name, "config.jsonc")
	data, err := readJSONC(path)
	if err != nil {
		if isNotExist(err) {
			return map[string]interface{}{}, nil
		}
		return nil, fmt.Errorf("reading plugin config %s: %w", name, err)
	}
	var cfg map[string]interface{}
	if err := unmarshalJSON(data, &cfg); err != nil {
		return nil, &types.ParseError{Source: name, Message: err.Error()}
	}
	return cfg, nil
}

// Helpers

func unmarshalJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func isNotExist(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}

func removeFile(path string) error {
	return os.Remove(path)
}

func readdirNames(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names, nil
}
