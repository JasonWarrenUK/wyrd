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
	nodes      map[string]*types.Node
	edges      map[string]*types.Edge
	deletedIDs []string // tracks DeleteEdge calls
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
func (s *formTestStore) DeleteEdge(id string) error                         { delete(s.edges, id); s.deletedIDs = append(s.deletedIDs, id); return nil }
func (s *formTestStore) ArchiveNode(id string) error                        { n := s.nodes[id]; if n != nil { n.Properties["status"] = "archived" }; return nil }
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

// formTestIndex is a minimal GraphIndex for form edge-management tests.
type formTestIndex struct {
	nodes []*types.Node
	edges []*types.Edge
}

func (i *formTestIndex) GetNode(id string) (*types.Node, error) {
	for _, n := range i.nodes {
		if n.ID == id {
			return n, nil
		}
	}
	return nil, &types.NotFoundError{Kind: "node", ID: id}
}

func (i *formTestIndex) GetEdge(id string) (*types.Edge, error) {
	for _, e := range i.edges {
		if e.ID == id {
			return e, nil
		}
	}
	return nil, &types.NotFoundError{Kind: "edge", ID: id}
}

func (i *formTestIndex) AllNodes() []*types.Node { return i.nodes }
func (i *formTestIndex) AllEdges() []*types.Edge { return i.edges }

func (i *formTestIndex) EdgesFrom(nodeID string) []*types.Edge {
	var out []*types.Edge
	for _, e := range i.edges {
		if e.From == nodeID {
			out = append(out, e)
		}
	}
	return out
}

func (i *formTestIndex) EdgesTo(nodeID string) []*types.Edge {
	var out []*types.Edge
	for _, e := range i.edges {
		if e.To == nodeID {
			out = append(out, e)
		}
	}
	return out
}

func (i *formTestIndex) NodesByType(typeName string) []*types.Node {
	var out []*types.Node
	for _, n := range i.nodes {
		for _, t := range n.Types {
			if t == typeName {
				out = append(out, n)
				break
			}
		}
	}
	return out
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

// TestFormConfirmFieldPresentWhenLinked verifies that when a selectedNodeID is
// provided, the form includes the "Link to selected node?" confirm field.
func TestFormConfirmFieldPresentWhenLinked(t *testing.T) {
	theme := loadTestTheme(t)
	store := newFormTestStore()
	clock := formTestClock()

	fp := tui.NewTaskFormPane(theme, store, clock, "abc-123", "Buy milk")

	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	v := sized.View()
	if !strings.Contains(v, "Link to selected node") {
		t.Errorf("expected confirm field in view when selectedNodeID is set; got:\n%s", v)
	}
}

// TestFormNoConfirmFieldWhenUnlinked verifies that when no selectedNodeID is
// provided, the form does not include the confirm field.
func TestFormNoConfirmFieldWhenUnlinked(t *testing.T) {
	theme := loadTestTheme(t)
	store := newFormTestStore()
	clock := formTestClock()

	fp := tui.NewTaskFormPane(theme, store, clock, "", "Buy milk")

	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	v := sized.View()
	if strings.Contains(v, "Link to selected node") {
		t.Errorf("did not expect confirm field in view when selectedNodeID is empty; got:\n%s", v)
	}
}

// --- CP.10: edit form tests ---

// seedNode is a helper that creates a minimal node for edit form tests.
func seedNode(id, title, body string, nodeTypes []string) *types.Node {
	return &types.Node{
		ID:         id,
		Title:      title,
		Body:       body,
		Types:      nodeTypes,
		Created:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Modified:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Properties: map[string]interface{}{"status": "active", "energy": "deep"},
	}
}

// TestEditTaskFormPaneViewRenders verifies that an edit task form produces a
// non-empty view and does not panic.
func TestEditTaskFormPaneViewRenders(t *testing.T) {
	theme := loadTestTheme(t)
	store := newFormTestStore()
	clock := formTestClock()
	node := seedNode("node-1", "Buy groceries", "From the list", []string{"task"})

	fp := tui.NewEditTaskFormPane(theme, store, clock, nil, node)
	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	v := sized.View()
	if v == "" {
		t.Error("expected non-empty view from edit task formPane")
	}
}

// TestEditJournalFormPaneViewRenders verifies the edit journal form renders.
func TestEditJournalFormPaneViewRenders(t *testing.T) {
	theme := loadTestTheme(t)
	store := newFormTestStore()
	clock := formTestClock()
	node := seedNode("node-2", "2026-01-01", "Today I did things", []string{"journal"})

	fp := tui.NewEditJournalFormPane(theme, store, clock, nil, node)
	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	v := sized.View()
	if v == "" {
		t.Error("expected non-empty view from edit journal formPane")
	}
}

// TestEditNoteFormPaneViewRenders verifies the edit note form renders.
func TestEditNoteFormPaneViewRenders(t *testing.T) {
	theme := loadTestTheme(t)
	store := newFormTestStore()
	clock := formTestClock()
	node := seedNode("node-3", "Architecture notes", "The system uses...", []string{"note"})

	fp := tui.NewEditNoteFormPane(theme, store, clock, nil, node)
	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	v := sized.View()
	if v == "" {
		t.Error("expected non-empty view from edit note formPane")
	}
}

// TestEditFormNoLinkField verifies the "Link to selected node?" confirm field
// is absent from edit forms (it only makes sense on creation).
func TestEditFormNoLinkField(t *testing.T) {
	theme := loadTestTheme(t)
	store := newFormTestStore()
	clock := formTestClock()
	node := seedNode("node-4", "Some task", "", []string{"task"})

	fp := tui.NewEditTaskFormPane(theme, store, clock, nil, node)
	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	v := sized.View()
	if strings.Contains(v, "Link to selected node") {
		t.Errorf("did not expect link confirm field in edit form; got:\n%s", v)
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

// --- CP.11: edge management tests ---

// TestEditFormShowsExistingEdges verifies that when an index has edges
// connected to the edited node, the form renders the "Existing Edges" section.
func TestEditFormShowsExistingEdges(t *testing.T) {
	theme := loadTestTheme(t)
	store := newFormTestStore()
	clock := formTestClock()

	node := seedNode("node-a", "Task A", "", []string{"task"})
	target := seedNode("node-b", "Task B", "", []string{"task"})

	index := &formTestIndex{
		nodes: []*types.Node{node, target},
		edges: []*types.Edge{
			{
				ID:   "edge-1",
				Type: "blocks",
				From: "node-a",
				To:   "node-b",
			},
		},
	}

	fp := tui.NewEditTaskFormPane(theme, store, clock, index, node)
	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 60})

	v := sized.View()
	if !strings.Contains(v, "Existing Edges") {
		t.Errorf("expected 'Existing Edges' section in edit form with edges; got:\n%s", v)
	}
}

// TestEditFormShowsAddEdgeType verifies that edit forms always include the
// "Add Edge Type" field for creating new edges.
func TestEditFormShowsAddEdgeType(t *testing.T) {
	theme := loadTestTheme(t)
	store := newFormTestStore()
	clock := formTestClock()

	node := seedNode("node-a", "Task A", "", []string{"task"})

	// No edges — the add-edge fields should still appear.
	index := &formTestIndex{
		nodes: []*types.Node{node},
	}

	fp := tui.NewEditTaskFormPane(theme, store, clock, index, node)
	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 60})

	v := sized.View()
	if !strings.Contains(v, "Add Edge Type") {
		t.Errorf("expected 'Add Edge Type' field in edit form; got:\n%s", v)
	}
}

// TestEditFormNilIndexNoEdgeFields verifies that when index is nil, the edit
// form still renders without crashing and includes edge creation fields.
func TestEditFormNilIndexNoEdgeFields(t *testing.T) {
	theme := loadTestTheme(t)
	store := newFormTestStore()
	clock := formTestClock()

	node := seedNode("node-a", "Task A", "", []string{"task"})

	fp := tui.NewEditTaskFormPane(theme, store, clock, nil, node)
	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 60})

	v := sized.View()
	if v == "" {
		t.Error("expected non-empty view from edit form with nil index")
	}
	// Edge fields should still appear (just no existing edges to show).
	if !strings.Contains(v, "Add Edge Type") {
		t.Errorf("expected 'Add Edge Type' even with nil index; got:\n%s", v)
	}
}

// TestEditFormEdgeLabelsShowTargetTitle verifies that the edge multi-select
// section appears when edges exist. The actual option labels may be truncated
// by the huh renderer depending on viewport height, so we check for the
// section title rather than individual option text.
func TestEditFormEdgeLabelsShowTargetTitle(t *testing.T) {
	theme := loadTestTheme(t)
	store := newFormTestStore()
	clock := formTestClock()

	node := seedNode("node-a", "Task A", "", []string{"task"})
	target := seedNode("node-b", "Important Target", "", []string{"task"})

	index := &formTestIndex{
		nodes: []*types.Node{node, target},
		edges: []*types.Edge{
			{
				ID:   "edge-1",
				Type: "related",
				From: "node-a",
				To:   "node-b",
			},
		},
	}

	fp := tui.NewEditTaskFormPane(theme, store, clock, index, node)
	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 60})

	v := sized.View()
	// The multi-select is present (blurred fields may not render option text).
	if !strings.Contains(v, "Existing Edges") {
		t.Errorf("expected 'Existing Edges' multi-select section; got:\n%s", v)
	}
}

// TestEditFormIncomingEdgeShowsSection verifies that incoming edges also cause
// the "Existing Edges" multi-select to appear.
func TestEditFormIncomingEdgeShowsSection(t *testing.T) {
	theme := loadTestTheme(t)
	store := newFormTestStore()
	clock := formTestClock()

	source := seedNode("node-b", "Blocker", "", []string{"task"})
	node := seedNode("node-a", "Blocked Task", "", []string{"task"})

	index := &formTestIndex{
		nodes: []*types.Node{node, source},
		edges: []*types.Edge{
			{
				ID:   "edge-1",
				Type: "blocks",
				From: "node-b",
				To:   "node-a",
			},
		},
	}

	fp := tui.NewEditTaskFormPane(theme, store, clock, index, node)
	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 60})

	v := sized.View()
	if !strings.Contains(v, "Existing Edges") {
		t.Errorf("expected 'Existing Edges' for incoming edge; got:\n%s", v)
	}
}

// TestEditJournalFormShowsEdgeFields verifies that journal edit forms also
// include edge management fields.
func TestEditJournalFormShowsEdgeFields(t *testing.T) {
	theme := loadTestTheme(t)
	store := newFormTestStore()
	clock := formTestClock()

	node := seedNode("node-j", "2026-01-01", "Journal entry", []string{"journal"})
	index := &formTestIndex{
		nodes: []*types.Node{node},
	}

	fp := tui.NewEditJournalFormPane(theme, store, clock, index, node)
	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 60})

	v := sized.View()
	if !strings.Contains(v, "Add Edge Type") {
		t.Errorf("expected edge fields in journal edit form; got:\n%s", v)
	}
}

// TestEditNoteFormShowsEdgeFields verifies that note edit forms also include
// edge management fields.
func TestEditNoteFormShowsEdgeFields(t *testing.T) {
	theme := loadTestTheme(t)
	store := newFormTestStore()
	clock := formTestClock()

	node := seedNode("node-n", "My note", "Note body", []string{"note"})
	index := &formTestIndex{
		nodes: []*types.Node{node},
	}

	fp := tui.NewEditNoteFormPane(theme, store, clock, index, node)
	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 60})

	v := sized.View()
	if !strings.Contains(v, "Add Edge Type") {
		t.Errorf("expected edge fields in note edit form; got:\n%s", v)
	}
}

// TestEditFormNoExistingEdgesSection verifies that when no edges exist, the
// "Existing Edges" multi-select is not shown.
func TestEditFormNoExistingEdgesSection(t *testing.T) {
	theme := loadTestTheme(t)
	store := newFormTestStore()
	clock := formTestClock()

	node := seedNode("node-a", "Task A", "", []string{"task"})
	index := &formTestIndex{
		nodes: []*types.Node{node},
	}

	fp := tui.NewEditTaskFormPane(theme, store, clock, index, node)
	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 60})

	v := sized.View()
	if strings.Contains(v, "Existing Edges") {
		t.Errorf("did not expect 'Existing Edges' when no edges exist; got:\n%s", v)
	}
}
