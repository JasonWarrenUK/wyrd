package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jasonwarrenuk/wyrd/internal/query"
	"github.com/jasonwarrenuk/wyrd/internal/store"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// newModelWithCustomDashboardView builds a Model over a real store whose
// saved dashboard view specifies a non-default column set.
func newModelWithCustomDashboardView(t *testing.T, cols []string) Model {
	t.Helper()
	dir := t.TempDir()
	clock := types.StubClock{Fixed: time.Date(2026, 3, 23, 9, 0, 0, 0, time.UTC)}
	s, err := store.New(dir, clock)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	// Persist a dashboard view with custom columns. The starter store may
	// already ship a views/dashboard.jsonc; overwrite it either way.
	viewJSON := `{"name": "dashboard", "columns": ["` + cols[0] + `", "` + cols[1] + `"]}`
	viewPath := filepath.Join(dir, "views", "dashboard.jsonc")
	if err := os.MkdirAll(filepath.Dir(viewPath), 0o755); err != nil {
		t.Fatalf("mkdir views: %v", err)
	}
	if err := os.WriteFile(viewPath, []byte(viewJSON), 0o644); err != nil {
		t.Fatalf("writing dashboard view: %v", err)
	}

	if _, err := s.CreateNode("A task body", []string{"task"}); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	engine := query.NewEngine(s.Index(), 10)
	m, err := New(Config{
		Store:       s,
		StorePath:   dir,
		Index:       s.Index(),
		QueryRunner: engine,
		Clock:       clock,
	})
	if err != nil {
		t.Fatalf("tui.New: %v", err)
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return updated.(Model)
}

// leftPaneColumns extracts the column set the mounted node list pane renders.
func leftPaneColumns(t *testing.T, m Model) []string {
	t.Helper()
	lp, ok := m.leftPane.(nodeListPane)
	if !ok {
		t.Fatalf("left pane is %T, want nodeListPane", m.leftPane)
	}
	return lp.result.Columns
}

func TestRefreshDashboard_PreservesCustomViewColumns(t *testing.T) {
	custom := []string{"kind", "title"}
	m := newModelWithCustomDashboardView(t, custom)

	if got := leftPaneColumns(t, m); !reflect.DeepEqual(got, custom) {
		t.Fatalf("startup columns = %v, want %v", got, custom)
	}

	// Previously every refresh site dropped the columns argument, reverting
	// the pane to the package defaults after any capture/edit/archive/stage
	// shift. Refresh must keep the saved view's columns.
	m.refreshDashboard()
	if got := leftPaneColumns(t, m); !reflect.DeepEqual(got, custom) {
		t.Errorf("post-refresh columns = %v, want %v", got, custom)
	}
}

func TestRefreshDashboard_FallsBackToStartupColumnsWhenViewUnreadable(t *testing.T) {
	custom := []string{"kind", "title"}
	m := newModelWithCustomDashboardView(t, custom)

	// Corrupt the saved view so ReadView fails; the refresh should fall back
	// to the columns resolved at startup rather than the package defaults.
	viewPath := filepath.Join(m.storePath, "views", "dashboard.jsonc")
	if err := os.WriteFile(viewPath, []byte("{invalid"), 0o644); err != nil {
		t.Fatalf("corrupting view: %v", err)
	}

	m.refreshDashboard()
	if got := leftPaneColumns(t, m); !reflect.DeepEqual(got, custom) {
		t.Errorf("fallback columns = %v, want %v", got, custom)
	}
}
