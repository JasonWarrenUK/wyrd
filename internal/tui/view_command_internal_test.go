package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jasonwarrenuk/wyrd/internal/store"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// newViewCommandTestModel builds a Model against a real store (so
// StoreFS.ReadView reads genuine JSONC from disk, exactly as production
// does) but with a stubRunner swapped in for QueryRunner, so the query
// results are deterministic without a real query engine. writeView, if
// non-empty, is written to {store}/views/{name}.jsonc before construction.
func newViewCommandTestModel(t *testing.T, viewName, viewJSONC string, runner *stubRunner) Model {
	t.Helper()
	dir := t.TempDir()
	clock := types.StubClock{Fixed: time.Date(2026, 3, 23, 9, 0, 0, 0, time.UTC)}
	s, err := store.New(dir, clock)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	if viewJSONC != "" {
		viewsDir := filepath.Join(s.StorePath(), "views")
		if err := os.MkdirAll(viewsDir, 0o755); err != nil {
			t.Fatalf("MkdirAll views: %v", err)
		}
		path := filepath.Join(viewsDir, viewName+".jsonc")
		if err := os.WriteFile(path, []byte(viewJSONC), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", path, err)
		}
	}

	m, err := New(Config{
		Store:       s,
		StorePath:   dir,
		Index:       s.Index(),
		QueryRunner: runner,
		Clock:       clock,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	return updated.(Model)
}

// TestOpenViewMsg_MountsListView is the end-to-end TD.13 contract test:
// dispatching openViewMsg for a view with "display": "list" reads the view
// from the store, runs its query, and mounts a viewPane on the left pane
// showing the query's rows.
func TestOpenViewMsg_MountsListView(t *testing.T) {
	viewJSONC := `{
		"name": "today",
		"query": "MATCH (n) WHERE 'task' IN n.types RETURN n",
		"display": "list",
		"columns": ["title", "status"]
	}`
	runner := &stubRunner{results: map[string]*types.QueryResult{
		"MATCH (n) WHERE 'task' IN n.types RETURN n": {
			Columns: []string{"title", "status"},
			Rows: []map[string]interface{}{
				{"title": "Write tests", "status": "open"},
			},
		},
	}}
	m := newViewCommandTestModel(t, "today", viewJSONC, runner)

	updated, cmd := m.Update(openViewMsg{name: "today"})
	got, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", updated)
	}
	if cmd == nil {
		t.Fatal("expected a non-nil confirmation command")
	}

	vp, ok := got.leftPane.(viewPane)
	if !ok {
		t.Fatalf("leftPane is %T, want viewPane", got.leftPane)
	}
	out := stripANSI(vp.View())
	if !strings.Contains(out, "Write tests") {
		t.Errorf("expected mounted view to show query row content, got %q", out)
	}
}

// TestOpenViewMsg_MountsTimelineView covers the timeline dispatch branch
// through the same end-to-end path.
func TestOpenViewMsg_MountsTimelineView(t *testing.T) {
	viewJSONC := `{
		"name": "journal",
		"query": "MATCH (n) WHERE 'journal' IN n.types RETURN n",
		"display": "timeline"
	}`
	runner := &stubRunner{results: map[string]*types.QueryResult{
		"MATCH (n) WHERE 'journal' IN n.types RETURN n": {
			Columns: []string{"created", "body"},
			Rows: []map[string]interface{}{
				{"created": "2026-03-01T09:00:00Z", "body": "A journal entry"},
			},
		},
	}}
	m := newViewCommandTestModel(t, "journal", viewJSONC, runner)

	updated, _ := m.Update(openViewMsg{name: "journal"})
	got := updated.(Model)

	vp, ok := got.leftPane.(viewPane)
	if !ok {
		t.Fatalf("leftPane is %T, want viewPane", got.leftPane)
	}
	if vp.view.Display != types.DisplayTimeline {
		t.Errorf("mounted pane's view.Display = %q, want %q", vp.view.Display, types.DisplayTimeline)
	}
	out := stripANSI(vp.View())
	if !strings.Contains(out, "A journal entry") {
		t.Errorf("expected mounted view to show query row content, got %q", out)
	}
}

// TestOpenViewMsg_UnknownNameShowsError covers the not-found path: a
// missing view must not panic or silently mount an empty pane — it reports
// via the status bar and leaves the left pane untouched.
func TestOpenViewMsg_UnknownNameShowsError(t *testing.T) {
	m := newViewCommandTestModel(t, "", "", &stubRunner{})

	updated, cmd := m.Update(openViewMsg{name: "does-not-exist"})
	got, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", updated)
	}
	if cmd == nil {
		t.Fatal("expected a non-nil status-bar clear command")
	}
	if _, isView := got.leftPane.(viewPane); isView {
		t.Error("left pane should not become a viewPane when the named view doesn't exist")
	}
}

// TestOpenViewMsg_QueryErrorIsSticky covers a view whose query fails at
// runtime (e.g. a syntax error introduced by hand-editing the JSONC) — the
// status-bar message must be sticky (esc-only dismissal) since this is an
// error worth reading, not a transient confirmation, mirroring the sticky
// pattern used for sync failures and rename errors elsewhere in app.go.
func TestOpenViewMsg_QueryErrorIsSticky(t *testing.T) {
	viewJSONC := `{
		"name": "broken",
		"query": "NOT VALID CYPHER",
		"display": "list"
	}`
	runner := &stubRunner{err: &types.QueryError{Query: "NOT VALID CYPHER", Message: "parse error"}}
	m := newViewCommandTestModel(t, "broken", viewJSONC, runner)

	updated, _ := m.Update(openViewMsg{name: "broken"})
	got := updated.(Model)

	if !got.statusBar.CaptureSticky() {
		t.Error("expected the query-failure message to be sticky")
	}
}
