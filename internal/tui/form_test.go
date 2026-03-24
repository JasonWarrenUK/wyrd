package tui_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jasonwarrenuk/wyrd/internal/tui"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// formTestStore is a minimal StoreFS for form tests.
type formTestStore struct {
	nodes map[string]*types.Node
	edges map[string]*types.Edge
}

func newFormTestStore() *formTestStore {
	return &formTestStore{
		nodes: make(map[string]*types.Node),
		edges: make(map[string]*types.Edge),
	}
}

func (s *formTestStore) ReadNode(id string) (*types.Node, error)            { return s.nodes[id], nil }
func (s *formTestStore) WriteNode(n *types.Node) error                      { s.nodes[n.ID] = n; return nil }
func (s *formTestStore) ReadEdge(id string) (*types.Edge, error)            { return s.edges[id], nil }
func (s *formTestStore) WriteEdge(e *types.Edge) error                      { s.edges[e.ID] = e; return nil }
func (s *formTestStore) DeleteEdge(id string) error                         { delete(s.edges, id); return nil }
func (s *formTestStore) ReadTemplate(_ string) (*types.Template, error)     { return nil, nil }
func (s *formTestStore) AllTemplates() ([]*types.Template, error)           { return nil, nil }
func (s *formTestStore) ReadView(_ string) (*types.SavedView, error)        { return nil, nil }
func (s *formTestStore) AllViews() ([]*types.SavedView, error)              { return nil, nil }
func (s *formTestStore) ReadRitual(_ string) (*types.Ritual, error)         { return nil, nil }
func (s *formTestStore) AllRituals() ([]*types.Ritual, error)               { return nil, nil }
func (s *formTestStore) ReadTheme(_ string) (*types.Theme, error)           { return nil, nil }
func (s *formTestStore) ReadConfig() (*types.Config, error)                 { return nil, nil }
func (s *formTestStore) WriteConfig(_ *types.Config) error                  { return nil }
func (s *formTestStore) StorePath() string                                  { return "/tmp/form-test" }

func formTestClock() types.Clock {
	return types.StubClock{Fixed: time.Date(2026, 3, 23, 9, 0, 0, 0, time.UTC)}
}

func loadTestTheme(t *testing.T) *tui.ActiveTheme {
	t.Helper()
	theme, err := tui.LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	return theme
}

// TestTaskFormPaneViewRenders verifies that a task formPane produces a
// non-empty view and does not panic.
func TestTaskFormPaneViewRenders(t *testing.T) {
	theme := loadTestTheme(t)
	store := newFormTestStore()
	clock := formTestClock()

	fp := tui.NewTaskFormPane(theme, store, clock, "", "Buy milk")

	// Deliver a size message.
	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	v := sized.View()
	if v == "" {
		t.Error("expected non-empty view from task formPane")
	}
}

// TestJournalFormPaneViewRenders verifies the journal form renders.
func TestJournalFormPaneViewRenders(t *testing.T) {
	theme := loadTestTheme(t)
	store := newFormTestStore()
	clock := formTestClock()

	fp := tui.NewJournalFormPane(theme, store, clock, "", "")

	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	v := sized.View()
	if v == "" {
		t.Error("expected non-empty view from journal formPane")
	}
}

// TestNoteFormPaneViewRenders verifies the note form renders.
func TestNoteFormPaneViewRenders(t *testing.T) {
	theme := loadTestTheme(t)
	store := newFormTestStore()
	clock := formTestClock()

	fp := tui.NewNoteFormPane(theme, store, clock, "", "My note")

	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	v := sized.View()
	if v == "" {
		t.Error("expected non-empty view from note formPane")
	}
}

// TestTaskFormBodyPlaceholder verifies the task form body textarea shows its
// placeholder text when the field is empty.
func TestTaskFormBodyPlaceholder(t *testing.T) {
	theme := loadTestTheme(t)
	store := newFormTestStore()
	clock := formTestClock()

	fp := tui.NewTaskFormPane(theme, store, clock, "", "")

	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	v := sized.View()
	// The cursor is injected between the first character and the rest of the
	// placeholder, splitting "Describe" → "D" + ANSI + "escribe". Search for
	// the unambiguous suffix that is always contiguous in the rendered output.
	if !strings.Contains(v, "escribe the task") {
		t.Errorf("expected task body placeholder in view; got:\n%s", v)
	}
}

// TestJournalFormBodyPlaceholder verifies the journal form body textarea shows
// its placeholder text when the field is empty.
func TestJournalFormBodyPlaceholder(t *testing.T) {
	theme := loadTestTheme(t)
	store := newFormTestStore()
	clock := formTestClock()

	fp := tui.NewJournalFormPane(theme, store, clock, "", "")

	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	v := sized.View()
	if !strings.Contains(v, "rite your entry") {
		t.Errorf("expected journal body placeholder in view; got:\n%s", v)
	}
}

// TestNoteFormBodyPlaceholder verifies the note form body textarea shows its
// placeholder text when the field is empty.
func TestNoteFormBodyPlaceholder(t *testing.T) {
	theme := loadTestTheme(t)
	store := newFormTestStore()
	clock := formTestClock()

	fp := tui.NewNoteFormPane(theme, store, clock, "", "My note")

	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	v := sized.View()
	if !strings.Contains(v, "rite your note") {
		t.Errorf("expected note body placeholder in view; got:\n%s", v)
	}
}

// TestFormKeyBindingsAccurate verifies that the keybinding help text reflects
// how the huh Text field actually behaves.
func TestFormKeyBindingsAccurate(t *testing.T) {
	theme := loadTestTheme(t)
	store := newFormTestStore()
	clock := formTestClock()

	fp := tui.NewTaskFormPane(theme, store, clock, "", "")
	bindings := fp.KeyBindings()

	keySet := make(map[string]string)
	for _, b := range bindings {
		keySet[b.Key] = b.Description
	}

	required := []string{"alt+enter", "ctrl+e", "ctrl+c"}
	for _, k := range required {
		if _, ok := keySet[k]; !ok {
			t.Errorf("expected keybinding %q to be present", k)
		}
	}

	if _, ok := keySet["esc"]; ok {
		t.Error("keybinding \"esc\" should not be present — huh uses ctrl+c to abort")
	}
}

// TestFormPaneHandleFocusLostIsNoop verifies the interface contract.
func TestFormPaneHandleFocusLostIsNoop(t *testing.T) {
	theme := loadTestTheme(t)
	store := newFormTestStore()
	clock := formTestClock()

	fp := tui.NewTaskFormPane(theme, store, clock, "", "")
	cmd := fp.HandleFocusLost()
	if cmd != nil {
		t.Error("expected nil cmd from HandleFocusLost")
	}
}
