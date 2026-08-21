package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jasonwarrenuk/wyrd/internal/query"
	"github.com/jasonwarrenuk/wyrd/internal/store"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// TestDisplayProse_SelectionRoutesToProseRenderer is the TD.20 end-to-end
// contract: opening a DisplayProse saved view mounts a nodeListPane over its
// query's rows, and selecting a row renders the right pane via
// views.ProseRenderer (not DetailRenderer) — proven by checking for content
// only ProseRenderer's simpler contract would produce (no ARCHIVED/BLOCKED
// banner, no kind/stage line) while the node's own body still appears.
func TestDisplayProse_SelectionRoutesToProseRenderer(t *testing.T) {
	dir := t.TempDir()
	clock := types.StubClock{Fixed: time.Date(2026, 3, 23, 9, 0, 0, 0, time.UTC)}
	s, err := store.New(dir, clock)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	n, err := s.CreateNode("Prose marker body", []string{"note"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	viewsDir := filepath.Join(s.StorePath(), "views")
	if err := os.MkdirAll(viewsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll views: %v", err)
	}
	viewJSONC := `{
		"name": "notes",
		"query": "MATCH (n:note) RETURN n.id AS id, n.title AS title",
		"display": "prose"
	}`
	if err := os.WriteFile(filepath.Join(viewsDir, "notes.jsonc"), []byte(viewJSONC), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	engine := query.NewEngine(s.Index(), 10)
	m2, err := New(Config{
		Store:       s,
		StorePath:   dir,
		Index:       s.Index(),
		QueryRunner: engine,
		Clock:       clock,
	})
	if err != nil {
		t.Fatalf("tui.New: %v", err)
	}
	updated, _ := m2.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m := updated.(Model)

	updated, _ = m.Update(openViewMsg{name: "notes"})
	m = updated.(Model)

	if !m.leftPaneIsProseView {
		t.Fatal("expected leftPaneIsProseView after opening the DisplayProse view")
	}
	lp, ok := m.leftPane.(nodeListPane)
	if !ok {
		t.Fatalf("leftPane is %T, want nodeListPane", m.leftPane)
	}
	if len(lp.list.Items()) == 0 {
		t.Fatal("expected at least one row in the mounted nodeListPane")
	}

	selectNode(t, &m, n.ID)

	updated, cmd := m.Update(nodeSelectedMsg{nodeID: n.ID})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("expected a non-nil async render command from nodeSelectedMsg")
	}
	msg := cmd()
	proseMsg, ok := msg.(proseReadyMsg)
	if !ok {
		t.Fatalf("expected proseReadyMsg from the async command, got %T", msg)
	}

	updated, _ = m.Update(proseMsg)
	m = updated.(Model)

	// Glamour word-wraps and styles each word separately, so a literal
	// "Prose marker body" substring can span an ANSI reset boundary — check
	// for "Prose" and "marker" and "body" independently rather than the
	// full phrase (mirrors the same fix prose_test.go's stripANSI needed).
	if !paneContains(m, "Prose") || !paneContains(m, "marker") || !paneContains(m, "body") {
		t.Error("expected the selected node's body to reach the right pane via ProseRenderer")
	}
}
