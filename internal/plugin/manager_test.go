package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// ---- mock store -------------------------------------------------------

// mockStore implements types.PluginStore for testing.
type mockStore struct {
	storePath string
	nodes     map[string]*types.Node
	edges     map[string]*types.Edge
}

func newMockStore(t *testing.T) *mockStore {
	t.Helper()
	dir := t.TempDir()
	return &mockStore{
		storePath: dir,
		nodes:     make(map[string]*types.Node),
		edges:     make(map[string]*types.Edge),
	}
}

func (s *mockStore) StorePath() string                                    { return s.storePath }
func (s *mockStore) ReadNode(id string) (*types.Node, error)              { return s.nodes[id], nil }
func (s *mockStore) WriteNode(node *types.Node) error                     { s.nodes[node.ID] = node; return nil }
func (s *mockStore) ReadEdge(id string) (*types.Edge, error)              { return s.edges[id], nil }
func (s *mockStore) WriteEdge(edge *types.Edge) error                     { s.edges[edge.ID] = edge; return nil }
func (s *mockStore) DeleteEdge(id string) error                           { delete(s.edges, id); return nil }
func (s *mockStore) ArchiveNode(_ string) error                           { return nil }
func (s *mockStore) ReadTemplate(_ string) (*types.Template, error)       { return nil, nil }
func (s *mockStore) AllTemplates() ([]*types.Template, error)             { return nil, nil }
func (s *mockStore) ReadView(_ string) (*types.SavedView, error)          { return nil, nil }
func (s *mockStore) AllViews() ([]*types.SavedView, error)                { return nil, nil }
func (s *mockStore) ReadRitual(_ string) (*types.Ritual, error)           { return nil, nil }
func (s *mockStore) AllRituals() ([]*types.Ritual, error)                 { return nil, nil }
func (s *mockStore) ReadTheme(_ string) (*types.Theme, error)             { return nil, nil }
func (s *mockStore) ReadConfig() (*types.Config, error)                   { return &types.Config{}, nil }
func (s *mockStore) WriteConfig(_ *types.Config) error                    { return nil }
func (s *mockStore) ReadPluginManifest(name string) (*types.PluginManifest, error) {
	manifestPath := filepath.Join(s.storePath, "plugins", name, "plugin.jsonc")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, &types.NotFoundError{Kind: "plugin manifest", ID: name}
	}
	clean := stripJSONC(data)
	var m types.PluginManifest
	if err := json.Unmarshal(clean, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
func (s *mockStore) AllPluginManifests() ([]*types.PluginManifest, error) {
	pluginsDir := filepath.Join(s.storePath, "plugins")
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return nil, nil
	}
	var manifests []*types.PluginManifest
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		m, err := s.ReadPluginManifest(e.Name())
		if err == nil {
			manifests = append(manifests, m)
		}
	}
	return manifests, nil
}
func (s *mockStore) ReadPluginConfig(_ string) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

// ---- helpers ----------------------------------------------------------

// plantManifest writes a plugin.jsonc into a temp store so Discover() can find it.
func plantManifest(t *testing.T, store *mockStore, name string, manifest *types.PluginManifest) string {
	t.Helper()
	pluginDir := filepath.Join(store.storePath, "plugins", name)
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(pluginDir, "plugin.jsonc")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return pluginDir
}

// ---- tests ------------------------------------------------------------

func TestDiscover_PopulatesRegistry(t *testing.T) {
	store := newMockStore(t)

	// Use the real mock-sync-plugin script from testdata.
	testdataDir, err := filepath.Abs("testdata/mock-sync-plugin")
	if err != nil {
		t.Fatal(err)
	}

	manifest := &types.PluginManifest{
		Name:           "test-plugin",
		Version:        "1.0.0",
		Executable:     "mock-sync-plugin",
		ExecutableType: types.ExecutableTypeScript,
		Capabilities:   []types.PluginCapability{types.PluginCapabilitySync},
		Sync: &types.PluginSyncConfig{
			Triggers:  []string{"manual"},
			NodeTypes: []string{"task"},
		},
	}

	pluginDir := plantManifest(t, store, "test-plugin", manifest)

	// Copy the mock script into the planted plugin directory.
	scriptSrc := filepath.Join(testdataDir, "mock-sync-plugin")
	scriptDst := filepath.Join(pluginDir, "mock-sync-plugin")
	if err := copyFile(scriptSrc, scriptDst); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(store)
	if err := manager.Discover(); err != nil {
		t.Fatalf("Discover() returned error: %v", err)
	}

	entry, err := manager.Registry().Get("test-plugin")
	if err != nil {
		t.Fatalf("registry.Get(%q) returned error: %v", "test-plugin", err)
	}
	if entry.Manifest.Name != "test-plugin" {
		t.Errorf("expected manifest name %q, got %q", "test-plugin", entry.Manifest.Name)
	}
}

func TestDiscover_IgnoresMissingDirectory(t *testing.T) {
	store := newMockStore(t)
	// The plugins directory does not exist — Discover() should not error.
	manager := NewManager(store)
	if err := manager.Discover(); err != nil {
		t.Errorf("Discover() should not error on missing directory, got: %v", err)
	}
}

func TestDiscover_SkipsMalformedManifest(t *testing.T) {
	store := newMockStore(t)
	pluginDir := filepath.Join(store.storePath, "plugins", "bad-plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.jsonc"), []byte("{invalid json"), 0o644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(store)
	if err := manager.Discover(); err != nil {
		t.Errorf("Discover() should not error on malformed manifest, got: %v", err)
	}
	// The bad plugin should not be registered.
	if _, err := manager.Registry().Get("bad-plugin"); err == nil {
		t.Error("expected bad-plugin to not be in registry")
	}
}

func TestDeterministicNodeID_Stable(t *testing.T) {
	id1 := DeterministicNodeID("github", "issue-42")
	id2 := DeterministicNodeID("github", "issue-42")
	if id1 != id2 {
		t.Errorf("expected stable ID, got %q and %q", id1, id2)
	}
}

func TestDeterministicNodeID_Unique(t *testing.T) {
	id1 := DeterministicNodeID("github", "issue-42")
	id2 := DeterministicNodeID("github", "issue-43")
	if id1 == id2 {
		t.Error("different inputs produced the same ID")
	}
}

func TestDeterministicNodeID_CrossSource(t *testing.T) {
	id1 := DeterministicNodeID("github", "1")
	id2 := DeterministicNodeID("gitlab", "1")
	if id1 == id2 {
		t.Error("same external ID from different source types produced the same Wyrd ID")
	}
}

func TestRegistryGet_NotFound(t *testing.T) {
	r := &Registry{plugins: make(map[string]*PluginEntry)}
	_, err := r.Get("nonexistent")
	if err == nil {
		t.Error("expected error for missing plugin, got nil")
	}
}

func TestRunSync_UnregisteredPlugin(t *testing.T) {
	store := newMockStore(t)
	manager := NewManager(store)
	err := manager.RunSync("no-such-plugin", 5*time.Second)
	if err == nil {
		t.Error("expected error for unregistered plugin")
	}
}

func TestRunSync_AppliesUpserts(t *testing.T) {
	store := newMockStore(t)

	testdataDir, err := filepath.Abs("testdata/mock-sync-plugin")
	if err != nil {
		t.Fatal(err)
	}

	manifest := &types.PluginManifest{
		Name:           "mock-sync",
		Version:        "0.1.0",
		Executable:     "mock-sync-plugin",
		ExecutableType: types.ExecutableTypeScript,
		Capabilities:   []types.PluginCapability{types.PluginCapabilitySync},
		Sync: &types.PluginSyncConfig{
			Triggers:  []string{"manual"},
			NodeTypes: []string{"task"},
		},
	}

	pluginDir := plantManifest(t, store, "mock-sync", manifest)
	scriptSrc := filepath.Join(testdataDir, "mock-sync-plugin")
	scriptDst := filepath.Join(pluginDir, "mock-sync-plugin")
	if err := copyFile(scriptSrc, scriptDst); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(scriptDst, 0o755); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(store)
	if err := manager.Discover(); err != nil {
		t.Fatal(err)
	}

	if err := manager.RunSync("mock-sync", 10*time.Second); err != nil {
		t.Fatalf("RunSync() returned error: %v", err)
	}

	// Verify that at least one node was upserted.
	expectedID := DeterministicNodeID("mock", "item-1")
	node := store.nodes[expectedID]
	if node == nil {
		t.Errorf("expected node with deterministic ID %q to be in store", expectedID)
	}
}

// copyFile is a test helper that copies a file, preserving permissions.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, info.Mode())
}
