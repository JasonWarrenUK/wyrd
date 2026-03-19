// Package store provides a flat-file implementation of types.StoreFS.
// Nodes are stored as JSON files under {storePath}/nodes/{id}.json,
// edges under {storePath}/edges/{id}.json.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// FileStore is the flat-file implementation of types.StoreFS.
type FileStore struct {
	root string
}

// New returns a FileStore rooted at storePath.
func New(storePath string) *FileStore {
	return &FileStore{root: storePath}
}

// StorePath returns the absolute path to the store root.
func (s *FileStore) StorePath() string {
	return s.root
}

// ReadNode reads a node file from nodes/{id}.json.
func (s *FileStore) ReadNode(id string) (*types.Node, error) {
	path := filepath.Join(s.root, "nodes", id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &types.NotFoundError{Kind: "node", ID: id}
		}
		return nil, fmt.Errorf("reading node %q: %w", id, err)
	}
	var node types.Node
	if err := json.Unmarshal(data, &node); err != nil {
		return nil, &types.ParseError{Source: path, Message: err.Error()}
	}
	return &node, nil
}

// WriteNode persists a node to nodes/{id}.json atomically via a temp file.
func (s *FileStore) WriteNode(node *types.Node) error {
	if node.ID == "" {
		return &types.ValidationError{Field: "id", Message: "node ID must not be empty"}
	}
	dir := filepath.Join(s.root, "nodes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating nodes directory: %w", err)
	}
	data, err := json.MarshalIndent(node, "", "\t")
	if err != nil {
		return fmt.Errorf("marshalling node %q: %w", node.ID, err)
	}
	path := filepath.Join(dir, node.ID+".json")
	return atomicWrite(path, data)
}

// ReadEdge reads an edge file from edges/{id}.json.
func (s *FileStore) ReadEdge(id string) (*types.Edge, error) {
	path := filepath.Join(s.root, "edges", id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &types.NotFoundError{Kind: "edge", ID: id}
		}
		return nil, fmt.Errorf("reading edge %q: %w", id, err)
	}
	var edge types.Edge
	if err := json.Unmarshal(data, &edge); err != nil {
		return nil, &types.ParseError{Source: path, Message: err.Error()}
	}
	return &edge, nil
}

// WriteEdge persists an edge to edges/{id}.json atomically.
func (s *FileStore) WriteEdge(edge *types.Edge) error {
	if edge.ID == "" {
		return &types.ValidationError{Field: "id", Message: "edge ID must not be empty"}
	}
	dir := filepath.Join(s.root, "edges")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating edges directory: %w", err)
	}
	data, err := json.MarshalIndent(edge, "", "\t")
	if err != nil {
		return fmt.Errorf("marshalling edge %q: %w", edge.ID, err)
	}
	path := filepath.Join(dir, edge.ID+".json")
	return atomicWrite(path, data)
}

// DeleteEdge removes an edge file from disk.
func (s *FileStore) DeleteEdge(id string) error {
	path := filepath.Join(s.root, "edges", id+".json")
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return &types.NotFoundError{Kind: "edge", ID: id}
		}
		return fmt.Errorf("deleting edge %q: %w", id, err)
	}
	return nil
}

// ReadTemplate reads a template from templates/{typeName}.json.
func (s *FileStore) ReadTemplate(typeName string) (*types.Template, error) {
	path := filepath.Join(s.root, "templates", typeName+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &types.NotFoundError{Kind: "template", ID: typeName}
		}
		return nil, fmt.Errorf("reading template %q: %w", typeName, err)
	}
	var tmpl types.Template
	if err := json.Unmarshal(data, &tmpl); err != nil {
		return nil, &types.ParseError{Source: path, Message: err.Error()}
	}
	return &tmpl, nil
}

// AllTemplates returns all templates in the templates/ directory.
func (s *FileStore) AllTemplates() ([]*types.Template, error) {
	return readAllJSON(filepath.Join(s.root, "templates"), func() *types.Template { return &types.Template{} })
}

// ReadView reads a saved view from views/{name}.json.
func (s *FileStore) ReadView(name string) (*types.SavedView, error) {
	path := filepath.Join(s.root, "views", name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &types.NotFoundError{Kind: "view", ID: name}
		}
		return nil, fmt.Errorf("reading view %q: %w", name, err)
	}
	var view types.SavedView
	if err := json.Unmarshal(data, &view); err != nil {
		return nil, &types.ParseError{Source: path, Message: err.Error()}
	}
	return &view, nil
}

// AllViews returns all saved views in the views/ directory.
func (s *FileStore) AllViews() ([]*types.SavedView, error) {
	return readAllJSON(filepath.Join(s.root, "views"), func() *types.SavedView { return &types.SavedView{} })
}

// ReadRitual reads a ritual from rituals/{name}.json.
func (s *FileStore) ReadRitual(name string) (*types.Ritual, error) {
	path := filepath.Join(s.root, "rituals", name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &types.NotFoundError{Kind: "ritual", ID: name}
		}
		return nil, fmt.Errorf("reading ritual %q: %w", name, err)
	}
	var ritual types.Ritual
	if err := json.Unmarshal(data, &ritual); err != nil {
		return nil, &types.ParseError{Source: path, Message: err.Error()}
	}
	return &ritual, nil
}

// AllRituals returns all rituals in the rituals/ directory.
func (s *FileStore) AllRituals() ([]*types.Ritual, error) {
	return readAllJSON(filepath.Join(s.root, "rituals"), func() *types.Ritual { return &types.Ritual{} })
}

// ReadTheme reads a theme from themes/{name}.json.
func (s *FileStore) ReadTheme(name string) (*types.Theme, error) {
	path := filepath.Join(s.root, "themes", name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &types.NotFoundError{Kind: "theme", ID: name}
		}
		return nil, fmt.Errorf("reading theme %q: %w", name, err)
	}
	var theme types.Theme
	if err := json.Unmarshal(data, &theme); err != nil {
		return nil, &types.ParseError{Source: path, Message: err.Error()}
	}
	return &theme, nil
}

// ReadConfig reads the configuration from config.json.
func (s *FileStore) ReadConfig() (*types.Config, error) {
	path := filepath.Join(s.root, "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return defaults when no config file exists yet.
			return &types.Config{StorePath: s.root, MaxTraversalDepth: 5}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg types.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, &types.ParseError{Source: path, Message: err.Error()}
	}
	return &cfg, nil
}

// WriteConfig persists the config to config.json.
func (s *FileStore) WriteConfig(cfg *types.Config) error {
	data, err := json.MarshalIndent(cfg, "", "\t")
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}
	path := filepath.Join(s.root, "config.json")
	return atomicWrite(path, data)
}

// ReadPluginManifest reads a plugin manifest from plugins/{name}/plugin.json.
func (s *FileStore) ReadPluginManifest(name string) (*types.PluginManifest, error) {
	path := filepath.Join(s.root, "plugins", name, "plugin.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &types.NotFoundError{Kind: "plugin", ID: name}
		}
		return nil, fmt.Errorf("reading plugin manifest %q: %w", name, err)
	}
	var manifest types.PluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, &types.ParseError{Source: path, Message: err.Error()}
	}
	return &manifest, nil
}

// AllPluginManifests returns all plugin manifests found under plugins/.
func (s *FileStore) AllPluginManifests() ([]*types.PluginManifest, error) {
	pluginsDir := filepath.Join(s.root, "plugins")
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading plugins directory: %w", err)
	}
	var manifests []*types.PluginManifest
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifest, err := s.ReadPluginManifest(entry.Name())
		if err != nil {
			continue // skip plugins with unreadable manifests
		}
		manifests = append(manifests, manifest)
	}
	return manifests, nil
}

// ReadPluginConfig reads a plugin's user config from plugins/{name}/config.json.
func (s *FileStore) ReadPluginConfig(name string) (map[string]interface{}, error) {
	path := filepath.Join(s.root, "plugins", name, "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &types.NotFoundError{Kind: "plugin config", ID: name}
		}
		return nil, fmt.Errorf("reading plugin config %q: %w", name, err)
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, &types.ParseError{Source: path, Message: err.Error()}
	}
	return cfg, nil
}

// atomicWrite writes data to path via a temp file then renames it.
func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("writing temp file %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("renaming %q to %q: %w", tmp, path, err)
	}
	return nil
}

// readAllJSON is a generic helper that reads all *.json files in dir
// and unmarshals them into T using the provided constructor.
func readAllJSON[T any](dir string, newT func() *T) ([]*T, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading directory %q: %w", dir, err)
	}
	var results []*T
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		t := newT()
		if err := json.Unmarshal(data, t); err != nil {
			continue
		}
		results = append(results, t)
	}
	return results, nil
}
