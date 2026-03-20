package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// ---------------------------------------------------------------------------
// WriteNode / WriteEdge round-trip
// ---------------------------------------------------------------------------

func TestWriteNode_RoundTrip(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	node := &types.Node{
		ID:       "aaaaaaaa-0000-0000-0000-000000000001",
		Body:     "Full round-trip node",
		Types:    []string{"task", "note"},
		Created:  s.clock.Now(),
		Modified: s.clock.Now(),
		Properties: map[string]interface{}{
			"status":   "inbox",
			"priority": float64(2),
		},
	}

	if err := s.WriteNode(node); err != nil {
		t.Fatalf("WriteNode: %v", err)
	}

	got, err := s.ReadNode(node.ID)
	if err != nil {
		t.Fatalf("ReadNode: %v", err)
	}

	if got.ID != node.ID {
		t.Errorf("ID = %q, want %q", got.ID, node.ID)
	}
	if got.Body != node.Body {
		t.Errorf("Body = %q, want %q", got.Body, node.Body)
	}
	if len(got.Types) != 2 {
		t.Errorf("Types = %v, want 2 types", got.Types)
	}
	if got.Properties["status"] != "inbox" {
		t.Errorf("status = %v, want inbox", got.Properties["status"])
	}
}

func TestWriteEdge_RoundTrip(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	n1, _ := s.CreateNode("Source", []string{"task"})
	n2, _ := s.CreateNode("Target", []string{"task"})

	edge := &types.Edge{
		ID:      "bbbbbbbb-0000-0000-0000-000000000001",
		Type:    "blocks",
		From:    n1.ID,
		To:      n2.ID,
		Created: s.clock.Now(),
		Properties: map[string]interface{}{
			"reason": "dependency",
		},
	}

	if err := s.WriteEdge(edge); err != nil {
		t.Fatalf("WriteEdge: %v", err)
	}

	got, err := s.ReadEdge(edge.ID)
	if err != nil {
		t.Fatalf("ReadEdge: %v", err)
	}

	if got.ID != edge.ID {
		t.Errorf("ID = %q, want %q", got.ID, edge.ID)
	}
	if got.Type != "blocks" {
		t.Errorf("Type = %q, want blocks", got.Type)
	}
	if got.From != n1.ID || got.To != n2.ID {
		t.Errorf("From/To mismatch: from=%q to=%q", got.From, got.To)
	}
}

func TestWriteNode_AtomicFile(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	node, err := s.CreateNode("Atomic write test", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Read the raw file and confirm it is valid JSON.
	rawPath := filepath.Join(s.path, "nodes", node.ID+".jsonc")
	data, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("reading raw file: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Errorf("on-disk file is not valid JSON: %v", err)
	}

	if raw["id"] != node.ID {
		t.Errorf("on-disk id = %v, want %q", raw["id"], node.ID)
	}
}

// ---------------------------------------------------------------------------
// Views
// ---------------------------------------------------------------------------

func TestReadView_Found(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	viewData := map[string]interface{}{
		"name":    "inbox",
		"query":   `MATCH (n:task) WHERE n.status = "inbox" RETURN n`,
		"pinned":  true,
		"display": "list",
	}
	viewPath := filepath.Join(s.path, "views", "inbox.jsonc")
	if err := writeJSONC(viewPath, viewData); err != nil {
		t.Fatalf("writing view file: %v", err)
	}

	view, err := s.ReadView("inbox")
	if err != nil {
		t.Fatalf("ReadView: %v", err)
	}
	if view.Name != "inbox" {
		t.Errorf("Name = %q, want inbox", view.Name)
	}
	if view.Query == "" {
		t.Error("Query is empty")
	}
}

func TestReadView_NotFound(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	_, err := s.ReadView("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing view")
	}
	var nfe *types.NotFoundError
	if !isNotFoundError(err, &nfe) {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestAllViews(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	for _, name := range []string{"inbox", "focus"} {
		data := map[string]interface{}{
			"name":  name,
			"query": `MATCH (n) RETURN n`,
		}
		if err := writeJSONC(filepath.Join(s.path, "views", name+".jsonc"), data); err != nil {
			t.Fatalf("writing view %s: %v", name, err)
		}
	}

	views, err := s.AllViews()
	if err != nil {
		t.Fatalf("AllViews: %v", err)
	}
	if len(views) != 2 {
		t.Errorf("got %d views, want 2", len(views))
	}
}

// ---------------------------------------------------------------------------
// Rituals
// ---------------------------------------------------------------------------

func TestReadRitual_Found(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	ritualData := map[string]interface{}{
		"name":     "morning",
		"friction": "gate",
		"schedule": map[string]interface{}{
			"days": []string{"mon", "tue", "wed", "thu", "fri"},
			"time": "09:00",
		},
		"steps": []interface{}{},
	}
	ritualPath := filepath.Join(s.path, "rituals", "morning.jsonc")
	if err := writeJSONC(ritualPath, ritualData); err != nil {
		t.Fatalf("writing ritual: %v", err)
	}

	ritual, err := s.ReadRitual("morning")
	if err != nil {
		t.Fatalf("ReadRitual: %v", err)
	}
	if ritual.Name != "morning" {
		t.Errorf("Name = %q, want morning", ritual.Name)
	}
}

func TestAllRituals(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	for _, name := range []string{"morning", "evening"} {
		data := map[string]interface{}{
			"name":     name,
			"friction": "nudge",
			"schedule": map[string]interface{}{"days": []string{"mon"}, "time": "09:00"},
			"steps":    []interface{}{},
		}
		if err := writeJSONC(filepath.Join(s.path, "rituals", name+".jsonc"), data); err != nil {
			t.Fatalf("writing ritual %s: %v", name, err)
		}
	}

	rituals, err := s.AllRituals()
	if err != nil {
		t.Fatalf("AllRituals: %v", err)
	}
	if len(rituals) != 2 {
		t.Errorf("got %d rituals, want 2", len(rituals))
	}
}

// ---------------------------------------------------------------------------
// Themes
// ---------------------------------------------------------------------------

func TestReadTheme_Found(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	themeData := map[string]interface{}{
		"name":   "cairn",
		"source": "https://example.com/cairn",
		"tiers":  map[string]interface{}{},
		"glyphs": map[string]interface{}{},
	}
	themePath := filepath.Join(s.path, "themes", "cairn.jsonc")
	if err := writeJSONC(themePath, themeData); err != nil {
		t.Fatalf("writing theme: %v", err)
	}

	theme, err := s.ReadTheme("cairn")
	if err != nil {
		t.Fatalf("ReadTheme: %v", err)
	}
	if theme.Name != "cairn" {
		t.Errorf("Name = %q, want cairn", theme.Name)
	}
}

func TestReadTheme_NotFound(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	_, err := s.ReadTheme("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing theme")
	}
	var nfe *types.NotFoundError
	if !isNotFoundError(err, &nfe) {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

func TestReadConfig_Missing(t *testing.T) {
	// ReadConfig traverses one level up from s.path using filepath.Join(s.path, "..", "config.jsonc").
	// We create a dedicated parent+store hierarchy to control the layout.
	parent := t.TempDir()
	storePath := filepath.Join(parent, "store")
	clock := &fixedClock{t: time.Date(2026, 3, 17, 10, 30, 0, 0, time.UTC)}
	s, err := New(storePath, clock)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	// No config.jsonc in parent — should return defaults.
	cfg, err := s.ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if cfg.MaxTraversalDepth != 5 {
		t.Errorf("MaxTraversalDepth = %d, want 5 (default)", cfg.MaxTraversalDepth)
	}
}

func TestWriteConfig_RoundTrip(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	// WriteConfig goes one level up from s.path, so we must ensure the parent
	// directory exists (it does — it's the TempDir root).
	cfg := &types.Config{
		MaxTraversalDepth: 10,
		Theme:             "cairn",
		Editor:            "zed",
	}
	if err := s.WriteConfig(cfg); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	got, err := s.ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if got.MaxTraversalDepth != 10 {
		t.Errorf("MaxTraversalDepth = %d, want 10", got.MaxTraversalDepth)
	}
	if got.Theme != "cairn" {
		t.Errorf("Theme = %q, want cairn", got.Theme)
	}
	if got.Editor != "zed" {
		t.Errorf("Editor = %q, want zed", got.Editor)
	}
}

// ---------------------------------------------------------------------------
// Plugin manifests
// ---------------------------------------------------------------------------

func TestReadPluginManifest_Found(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	pluginDir := filepath.Join(s.path, "plugins", "myplugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("creating plugin dir: %v", err)
	}

	manifestData := map[string]interface{}{
		"name":            "myplugin",
		"version":         "1.0.0",
		"description":     "A test plugin",
		"executable":      "myplugin",
		"executable_type": "binary",
		"capabilities":    []string{"sync"},
	}
	manifestPath := filepath.Join(pluginDir, "manifest.jsonc")
	if err := writeJSONC(manifestPath, manifestData); err != nil {
		t.Fatalf("writing manifest: %v", err)
	}

	manifest, err := s.ReadPluginManifest("myplugin")
	if err != nil {
		t.Fatalf("ReadPluginManifest: %v", err)
	}
	if manifest.Name != "myplugin" {
		t.Errorf("Name = %q, want myplugin", manifest.Name)
	}
	if manifest.Version != "1.0.0" {
		t.Errorf("Version = %q, want 1.0.0", manifest.Version)
	}
}

func TestAllPluginManifests(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	for _, name := range []string{"plugin-a", "plugin-b"} {
		pluginDir := filepath.Join(s.path, "plugins", name)
		if err := os.MkdirAll(pluginDir, 0o755); err != nil {
			t.Fatalf("creating plugin dir %s: %v", name, err)
		}
		data := map[string]interface{}{
			"name":            name,
			"version":         "0.1.0",
			"executable":      name,
			"executable_type": "binary",
			"capabilities":    []string{},
		}
		if err := writeJSONC(filepath.Join(pluginDir, "manifest.jsonc"), data); err != nil {
			t.Fatalf("writing manifest for %s: %v", name, err)
		}
	}

	manifests, err := s.AllPluginManifests()
	if err != nil {
		t.Fatalf("AllPluginManifests: %v", err)
	}
	if len(manifests) != 2 {
		t.Errorf("got %d manifests, want 2", len(manifests))
	}
}

// ---------------------------------------------------------------------------
// Node type operations
// ---------------------------------------------------------------------------

func TestAddType(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	node, err := s.CreateNode("Multi-type node", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	if err := s.AddType(node.ID, "note"); err != nil {
		t.Fatalf("AddType: %v", err)
	}

	got, err := s.ReadNode(node.ID)
	if err != nil {
		t.Fatalf("ReadNode: %v", err)
	}

	hasTask, hasNote := false, false
	for _, typ := range got.Types {
		switch typ {
		case "task":
			hasTask = true
		case "note":
			hasNote = true
		}
	}
	if !hasTask {
		t.Error("expected task type to remain after AddType")
	}
	if !hasNote {
		t.Error("expected note type to be added")
	}
}

func TestRemoveType(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	node, err := s.CreateNode("Two-type node", []string{"task", "note"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	if err := s.RemoveType(node.ID, "note"); err != nil {
		t.Fatalf("RemoveType: %v", err)
	}

	got, err := s.ReadNode(node.ID)
	if err != nil {
		t.Fatalf("ReadNode: %v", err)
	}

	for _, typ := range got.Types {
		if typ == "note" {
			t.Error("note type should have been removed")
		}
	}
	if len(got.Types) != 1 || got.Types[0] != "task" {
		t.Errorf("Types = %v, want [task]", got.Types)
	}
}

func TestRemoveType_LastType(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	node, err := s.CreateNode("Single-type node", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	err = s.RemoveType(node.ID, "task")
	if err == nil {
		t.Fatal("expected error when removing the last type, got nil")
	}
	// Accept any non-nil error — ValidationError is the expected type here.
	_ = err
	// Node must be unchanged.
	got, err := s.ReadNode(node.ID)
	if err != nil {
		t.Fatalf("ReadNode: %v", err)
	}
	if len(got.Types) != 1 || got.Types[0] != "task" {
		t.Errorf("Types = %v, want [task] (node unchanged after rejected remove)", got.Types)
	}
}

// ---------------------------------------------------------------------------
// Index operations
// ---------------------------------------------------------------------------

func TestEdgesTo(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	n1, _ := s.CreateNode("Source", []string{"task"})
	n2, _ := s.CreateNode("Target", []string{"task"})
	edge, _ := s.CreateEdge("blocks", n1.ID, n2.ID, nil)

	idx := s.Index()
	edges := idx.EdgesTo(n2.ID)

	found := false
	for _, e := range edges {
		if e.ID == edge.ID {
			found = true
		}
	}
	if !found {
		t.Error("expected edge in EdgesTo result")
	}
}

func TestAllNodes_Count(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	s.CreateNode("Node 1", []string{"task"})
	s.CreateNode("Node 2", []string{"task"})
	s.CreateNode("Node 3", []string{"note"})

	idx := s.Index()
	nodes := idx.AllNodes()
	if len(nodes) != 3 {
		t.Errorf("AllNodes returned %d nodes, want 3", len(nodes))
	}
}

func TestAllEdges_Count(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	n1, _ := s.CreateNode("A", []string{"task"})
	n2, _ := s.CreateNode("B", []string{"task"})
	n3, _ := s.CreateNode("C", []string{"task"})

	s.CreateEdge("blocks", n1.ID, n2.ID, nil)
	s.CreateEdge("related", n2.ID, n3.ID, nil)

	idx := s.Index()
	edges := idx.AllEdges()
	if len(edges) != 2 {
		t.Errorf("AllEdges returned %d edges, want 2", len(edges))
	}
}

func TestGetEdge_NotFound(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	idx := s.Index()
	_, err := idx.GetEdge("00000000-0000-0000-0000-000000000099")
	if err == nil {
		t.Fatal("expected error for missing edge")
	}
	var nfe *types.NotFoundError
	if !isNotFoundError(err, &nfe) {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestBuildIndex_WithEdges(t *testing.T) {
	// Write nodes and edges to a temp dir, then open a fresh store.
	// This exercises buildIndex reading from disk on startup.
	dir := t.TempDir()
	clock := &fixedClock{t: time.Date(2026, 3, 17, 10, 30, 0, 0, time.UTC)}

	s1, err := New(dir, clock)
	if err != nil {
		t.Fatalf("New (first): %v", err)
	}

	n1, _ := s1.CreateNode("Alpha", []string{"task"})
	n2, _ := s1.CreateNode("Beta", []string{"task"})
	edge, _ := s1.CreateEdge("blocks", n1.ID, n2.ID, nil)
	s1.Close()

	// Open a second store from the same directory.
	s2, err := New(dir, clock)
	if err != nil {
		t.Fatalf("New (second): %v", err)
	}
	defer s2.Close()

	idx := s2.Index()
	if _, err := idx.GetNode(n1.ID); err != nil {
		t.Errorf("n1 not found after rebuild: %v", err)
	}
	if _, err := idx.GetEdge(edge.ID); err != nil {
		t.Errorf("edge not found after rebuild: %v", err)
	}
	edges := idx.EdgesFrom(n1.ID)
	if len(edges) != 1 {
		t.Errorf("EdgesFrom returned %d edges after rebuild, want 1", len(edges))
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func isNotFoundError(err error, target **types.NotFoundError) bool {
	if nfe, ok := err.(*types.NotFoundError); ok {
		*target = nfe
		return true
	}
	return false
}
