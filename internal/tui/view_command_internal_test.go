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

// TestEscRestoresDashboardFromViewPane covers the other half of the escape
// route: esc, not just a bare ":view", puts the node-list dashboard back
// once a viewPane is mounted. Before this, esc was a complete no-op with a
// viewPane focused — updateFocusedPane forwarded it to viewPane.Update,
// which ignores everything except tea.WindowSizeMsg.
func TestEscRestoresDashboardFromViewPane(t *testing.T) {
	viewJSONC := `{
		"name": "today",
		"query": "MATCH (n) WHERE 'task' IN n.types RETURN n",
		"display": "list"
	}`
	runner := &stubRunner{results: map[string]*types.QueryResult{
		"MATCH (n) WHERE 'task' IN n.types RETURN n": {
			Columns: []string{"title"},
			Rows:    []map[string]interface{}{{"title": "Write tests"}},
		},
	}}
	m := newViewCommandTestModel(t, "today", viewJSONC, runner)

	updated, _ := m.Update(openViewMsg{name: "today"})
	m = updated.(Model)
	if _, ok := m.leftPane.(viewPane); !ok {
		t.Fatalf("setup: leftPane is %T, want viewPane", m.leftPane)
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	got, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", updated)
	}
	if _, ok := got.leftPane.(nodeListPane); !ok {
		t.Errorf("leftPane is %T after esc, want nodeListPane", got.leftPane)
	}
}

// TestEscDismissesStickyMessageBeforeRestoringDashboard pins the ordering:
// when both a sticky status-bar message and a viewPane are active at once,
// esc dismisses the sticky message first (matching every other overlay's
// esc-to-close behaviour) rather than immediately restoring the dashboard —
// a second esc handles the restore.
func TestEscDismissesStickyMessageBeforeRestoringDashboard(t *testing.T) {
	viewJSONC := `{
		"name": "today",
		"query": "MATCH (n) WHERE 'task' IN n.types RETURN n",
		"display": "list"
	}`
	runner := &stubRunner{results: map[string]*types.QueryResult{
		"MATCH (n) WHERE 'task' IN n.types RETURN n": {
			Columns: []string{"title"},
			Rows:    []map[string]interface{}{{"title": "Write tests"}},
		},
	}}
	m := newViewCommandTestModel(t, "today", viewJSONC, runner)

	updated, _ := m.Update(openViewMsg{name: "today"})
	m = updated.(Model)
	m.statusBar.SetCaptureText("some sticky message")
	m.statusBar.MarkCaptureSticky()

	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	got := updated.(Model)

	if got.statusBar.CaptureSticky() {
		t.Error("expected the first esc to dismiss the sticky message")
	}
	if _, ok := got.leftPane.(viewPane); !ok {
		t.Fatalf("leftPane is %T after the first esc, want viewPane still mounted (sticky dismissal takes priority)", got.leftPane)
	}

	updated, _ = got.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	got2 := updated.(Model)
	if _, ok := got2.leftPane.(nodeListPane); !ok {
		t.Errorf("leftPane is %T after the second esc, want nodeListPane", got2.leftPane)
	}
}

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

// TestApplyThemeRebuildsViewPane covers the applyTheme branch added for
// viewPane: before this, a viewPane fell through applyTheme's pane-rebuild
// branches untouched, so its palette builders kept reading the *ActiveTheme
// from before the switch and rendered in the old theme's colours while the
// rest of the frame (status bar, right pane, overlays) repainted.
func TestApplyThemeRebuildsViewPane(t *testing.T) {
	themeA, err := LoadTheme(".", "cairn")
	if err != nil {
		t.Fatalf("LoadTheme cairn: %v", err)
	}
	themeB, err := LoadTheme(".", "peat")
	if err != nil {
		t.Fatalf("LoadTheme peat: %v", err)
	}

	view := &types.SavedView{Name: "today", Display: types.DisplayList, Columns: []string{"title"}}
	result := types.QueryResult{Columns: []string{"title"}, Rows: []map[string]interface{}{{"title": "Write tests"}}}
	vp := newViewPane(view, result, themeA)
	// A prior resize should survive the theme switch — applyTheme must not
	// rebuild through newViewPane, which would reset width back to its
	// 80-column default.
	sized, _ := vp.Update(tea.WindowSizeMsg{Width: 240, Height: 40})
	m := Model{leftPane: sized, theme: themeA}

	got := m.applyTheme(themeB)

	newVp, ok := got.leftPane.(viewPane)
	if !ok {
		t.Fatalf("leftPane is %T after applyTheme, want viewPane", got.leftPane)
	}
	if newVp.theme != themeB {
		t.Error("expected viewPane.theme to be repointed at the new theme")
	}
	if newVp.width != 120 {
		t.Errorf("viewPane.width = %d after applyTheme, want 120 (resize preserved, not reset to the 80 default)", newVp.width)
	}
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

// TestOpenViewMsg_BareViewRestoresDashboard covers the escape route added
// alongside esc: an openViewMsg with an empty name (what the palette's bare
// ":view" now dispatches, rather than the old silent no-op) restores the
// node-list dashboard instead of trying to look up a view literally named
// "". This and esc are the only two ways back to the dashboard once a
// viewPane has been mounted — see leftPaneNeedsRestore's doc comment for why
// a route back was needed at all.
func TestOpenViewMsg_BareViewRestoresDashboard(t *testing.T) {
	viewJSONC := `{
		"name": "today",
		"query": "MATCH (n) WHERE 'task' IN n.types RETURN n",
		"display": "list"
	}`
	runner := &stubRunner{results: map[string]*types.QueryResult{
		"MATCH (n) WHERE 'task' IN n.types RETURN n": {
			Columns: []string{"title"},
			Rows:    []map[string]interface{}{{"title": "Write tests"}},
		},
	}}
	m := newViewCommandTestModel(t, "today", viewJSONC, runner)

	// First mount a viewPane, exactly as :view today would.
	updated, _ := m.Update(openViewMsg{name: "today"})
	m = updated.(Model)
	if _, ok := m.leftPane.(viewPane); !ok {
		t.Fatalf("setup: leftPane is %T, want viewPane", m.leftPane)
	}

	// A bare ":view" (empty name) should restore the dashboard rather than
	// reporting "No view \"\"".
	updated, cmd := m.Update(openViewMsg{name: ""})
	got, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", updated)
	}
	if cmd == nil {
		t.Fatal("expected a non-nil status-bar clear command")
	}
	if _, ok := got.leftPane.(nodeListPane); !ok {
		t.Errorf("leftPane is %T after bare :view, want nodeListPane", got.leftPane)
	}
}

// TestOpenViewMsg_UnsupportedDisplayModeStillMountsWithWarning covers
// DisplayBudget: viewPane.View has no renderer for it yet and falls back to
// list rendering, so the data stays visible, but the openViewMsg handler
// must flag the mode by name via a sticky status message rather than
// leaving the fallback unexplained. DisplayProse moved to its own test
// below (TD.20): it's wired now and no longer takes this fallback path.
func TestOpenViewMsg_UnsupportedDisplayModeStillMountsWithWarning(t *testing.T) {
	viewJSONC := `{
		"name": "notes",
		"query": "MATCH (n:note) RETURN n.id AS id, n.title AS title",
		"display": "budget"
	}`
	runner := &stubRunner{results: map[string]*types.QueryResult{
		"MATCH (n:note) RETURN n.id AS id, n.title AS title": {
			Columns: []string{"id", "title"},
			Rows:    []map[string]interface{}{{"id": "n-1", "title": "A note"}},
		},
	}}
	m := newViewCommandTestModel(t, "notes", viewJSONC, runner)

	updated, _ := m.Update(openViewMsg{name: "notes"})
	got := updated.(Model)

	if _, ok := got.leftPane.(viewPane); !ok {
		t.Fatalf("leftPane is %T, want viewPane (unsupported modes still mount)", got.leftPane)
	}
	if !got.statusBar.CaptureSticky() {
		t.Error("expected the unsupported-mode message to be sticky")
	}
}

// TestOpenViewMsg_DisplayProseMountsNodeListPane covers TD.20: a
// DisplayProse saved view mounts a nodeListPane (the query's rows shown as
// a selectable list, matching the ordinary dashboard's own shape), sets
// leftPaneIsProseView, and reports success rather than the "not yet
// supported" warning DisplayProse used to carry before this task. The
// query aliases a scalar id column (RETURN n.id AS id, ...) rather than a
// bare "RETURN n": nodeListPane/rowsToItems reads row["id"] as a string, and
// a bare "RETURN n" binds the whole *types.Node under the match variable
// name instead, which is not a shape nodeListPane can select against.
func TestOpenViewMsg_DisplayProseMountsNodeListPane(t *testing.T) {
	viewJSONC := `{
		"name": "notes",
		"query": "MATCH (n:note) RETURN n.id AS id, n.title AS title",
		"display": "prose"
	}`
	runner := &stubRunner{results: map[string]*types.QueryResult{
		"MATCH (n:note) RETURN n.id AS id, n.title AS title": {
			Columns: []string{"id", "title"},
			Rows:    []map[string]interface{}{{"id": "n-1", "title": "A note"}},
		},
	}}
	m := newViewCommandTestModel(t, "notes", viewJSONC, runner)

	updated, cmd := m.Update(openViewMsg{name: "notes"})
	got := updated.(Model)

	if _, ok := got.leftPane.(nodeListPane); !ok {
		t.Fatalf("leftPane is %T, want nodeListPane (TD.20 DisplayProse mount)", got.leftPane)
	}
	if !got.leftPaneIsProseView {
		t.Error("expected leftPaneIsProseView to be true after mounting a DisplayProse view")
	}
	if got.statusBar.CaptureSticky() {
		t.Error("expected a plain (non-sticky) success message, not the unsupported-mode warning")
	}
	if cmd == nil {
		t.Error("expected a non-nil status-bar clear command on success")
	}
}

// TestOpenViewMsg_GuardedAgainstOpenOverlay is the openViewMsg-specific
// sibling of TestFormOpenMsgDoesNotMountUnderOpenOverlay: before this guard,
// openViewMsg was the only open-message handler in the switch without it,
// so dispatching :view while :help/:log/:kinds/:stages was open would mount
// a viewPane underneath the overlay — the left pane silently swapped out
// from under the user, discovered only after closing the overlay.
func TestOpenViewMsg_GuardedAgainstOpenOverlay(t *testing.T) {
	viewJSONC := `{
		"name": "today",
		"query": "MATCH (n) WHERE 'task' IN n.types RETURN n",
		"display": "list"
	}`
	runner := &stubRunner{results: map[string]*types.QueryResult{
		"MATCH (n) WHERE 'task' IN n.types RETURN n": {
			Columns: []string{"title"},
			Rows:    []map[string]interface{}{{"title": "Write tests"}},
		},
	}}

	overlays := []struct {
		name     string
		open     func(m *Model)
		isActive func(m Model) bool
	}{
		{
			name:     "help",
			open:     func(m *Model) { m.helpOverlay.Open(m.layout.totalWidth, m.layout.totalHeight, m.keyMap.AllBindings()) },
			isActive: func(m Model) bool { return m.helpOverlay.IsActive() },
		},
		{
			name:     "log",
			open:     func(m *Model) { m.logOverlay.Open(m.layout.totalWidth, m.layout.totalHeight) },
			isActive: func(m Model) bool { return m.logOverlay.IsActive() },
		},
		{
			name:     "kinds",
			open:     func(m *Model) { m.kindsOverlay.Open(m.layout.totalWidth, m.layout.totalHeight) },
			isActive: func(m Model) bool { return m.kindsOverlay.IsActive() },
		},
		{
			name:     "stages",
			open:     func(m *Model) { m.stagesOverlay.Open(m.layout.totalWidth, m.layout.totalHeight) },
			isActive: func(m Model) bool { return m.stagesOverlay.IsActive() },
		},
	}

	for _, ov := range overlays {
		t.Run(ov.name, func(t *testing.T) {
			m := newViewCommandTestModel(t, "today", viewJSONC, runner)
			ov.open(&m)
			if !ov.isActive(m) {
				t.Fatalf("%s overlay did not open", ov.name)
			}

			updated, _ := m.Update(openViewMsg{name: "today"})
			got, ok := updated.(Model)
			if !ok {
				t.Fatalf("Update returned %T, want Model", updated)
			}

			if _, isView := got.leftPane.(viewPane); isView {
				t.Errorf("openViewMsg mounted a viewPane while the %s overlay was still active", ov.name)
			}
			if !ov.isActive(got) {
				t.Errorf("%s overlay closed unexpectedly after openViewMsg", ov.name)
			}
		})
	}
}
