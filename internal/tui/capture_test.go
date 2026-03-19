package tui_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/tui"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// ---- Minimal mock store for capture tests ----------------------------------

type captureStore struct {
	nodes map[string]*types.Node
	edges map[string]*types.Edge
}

func newCaptureStore() *captureStore {
	return &captureStore{
		nodes: make(map[string]*types.Node),
		edges: make(map[string]*types.Edge),
	}
}

func (s *captureStore) ReadNode(id string) (*types.Node, error) {
	n, ok := s.nodes[id]
	if !ok {
		return nil, fmt.Errorf("node %s not found", id)
	}
	return n, nil
}
func (s *captureStore) WriteNode(n *types.Node) error {
	s.nodes[n.ID] = n
	return nil
}
func (s *captureStore) ReadEdge(id string) (*types.Edge, error) {
	e, ok := s.edges[id]
	if !ok {
		return nil, fmt.Errorf("edge %s not found", id)
	}
	return e, nil
}
func (s *captureStore) WriteEdge(e *types.Edge) error {
	s.edges[e.ID] = e
	return nil
}
func (s *captureStore) DeleteEdge(id string) error {
	delete(s.edges, id)
	return nil
}
func (s *captureStore) ReadTemplate(_ string) (*types.Template, error)  { return nil, nil }
func (s *captureStore) AllTemplates() ([]*types.Template, error)         { return nil, nil }
func (s *captureStore) ReadView(_ string) (*types.SavedView, error)      { return nil, nil }
func (s *captureStore) AllViews() ([]*types.SavedView, error)            { return nil, nil }
func (s *captureStore) ReadRitual(_ string) (*types.Ritual, error)       { return nil, nil }
func (s *captureStore) AllRituals() ([]*types.Ritual, error)             { return nil, nil }
func (s *captureStore) ReadTheme(_ string) (*types.Theme, error)         { return nil, nil }
func (s *captureStore) ReadConfig() (*types.Config, error)               { return nil, nil }
func (s *captureStore) WriteConfig(_ *types.Config) error                { return nil }
func (s *captureStore) StorePath() string                                 { return "/tmp/store" }

func fixedCaptureClock() types.Clock {
	return types.StubClock{Fixed: time.Date(2025, 3, 17, 9, 0, 0, 0, time.UTC)}
}

// ---- CaptureBar tests ------------------------------------------------------

func TestCaptureBar_DefaultTypeIsTask(t *testing.T) {
	store := newCaptureStore()
	bar := tui.NewCaptureBar(store, fixedCaptureClock())

	bar.Focus("")
	bar.SetInput("Review PRs")

	result, err := bar.Submit()
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if result == nil {
		t.Fatal("expected a CaptureResult, got nil")
	}
	if result.Node.Types[0] != "task" {
		t.Errorf("expected type task, got %q", result.Node.Types[0])
	}
	if result.Node.Body != "Review PRs" {
		t.Errorf("unexpected body: %q", result.Node.Body)
	}
	if result.Node.Properties["status"] != "inbox" {
		t.Errorf("expected status inbox, got %v", result.Node.Properties["status"])
	}
}

func TestCaptureBar_TaskPrefix(t *testing.T) {
	store := newCaptureStore()
	bar := tui.NewCaptureBar(store, fixedCaptureClock())

	bar.Focus("")
	bar.SetInput("t: Buy groceries")

	result, err := bar.Submit()
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if result.Node.Types[0] != "task" {
		t.Errorf("expected type task, got %q", result.Node.Types[0])
	}
	if result.Node.Body != "Buy groceries" {
		t.Errorf("unexpected body: %q", result.Node.Body)
	}
}

func TestCaptureBar_JournalPrefix(t *testing.T) {
	store := newCaptureStore()
	bar := tui.NewCaptureBar(store, fixedCaptureClock())

	bar.Focus("")
	bar.SetInput("j: Today was productive")

	result, err := bar.Submit()
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if result.Node.Types[0] != "journal" {
		t.Errorf("expected type journal, got %q", result.Node.Types[0])
	}
	if result.Node.Body != "Today was productive" {
		t.Errorf("unexpected body: %q", result.Node.Body)
	}
	// Journal nodes carry a date, not a status.
	if _, hasDate := result.Node.Properties["date"]; !hasDate {
		t.Error("expected journal node to have a date property")
	}
	if _, hasStatus := result.Node.Properties["status"]; hasStatus {
		t.Error("journal node should not have a status property")
	}
}

func TestCaptureBar_NotePrefix(t *testing.T) {
	store := newCaptureStore()
	bar := tui.NewCaptureBar(store, fixedCaptureClock())

	bar.Focus("")
	bar.SetInput("n: GraphQL tip — use fragments")

	result, err := bar.Submit()
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if result.Node.Types[0] != "note" {
		t.Errorf("expected type note, got %q", result.Node.Types[0])
	}
	if result.Node.Body != "GraphQL tip — use fragments" {
		t.Errorf("unexpected body: %q", result.Node.Body)
	}
	// Notes do not carry a status.
	if _, hasStatus := result.Node.Properties["status"]; hasStatus {
		t.Error("note node should not have a status property")
	}
}

func TestCaptureBar_EmptyInput(t *testing.T) {
	store := newCaptureStore()
	bar := tui.NewCaptureBar(store, fixedCaptureClock())

	bar.Focus("")
	bar.SetInput("   ")

	result, err := bar.Submit()
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if result != nil {
		t.Error("expected nil result for empty input")
	}
	if len(store.nodes) != 0 {
		t.Errorf("expected 0 nodes written for empty input, got %d", len(store.nodes))
	}
}

func TestCaptureBar_CreatesRelatedEdgeWhenNodeSelected(t *testing.T) {
	store := newCaptureStore()

	// Seed a pre-existing node that will be "selected" in the right pane.
	existing := &types.Node{
		ID:         "existing-node-id",
		Body:       "Existing task",
		Types:      []string{"task"},
		Properties: map[string]interface{}{},
	}
	store.nodes[existing.ID] = existing

	bar := tui.NewCaptureBar(store, fixedCaptureClock())
	bar.Focus("existing-node-id")
	bar.SetInput("Follow up on this")

	result, err := bar.Submit()
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if result.Edge == nil {
		t.Fatal("expected a related edge to be created")
	}
	if result.Edge.From != result.Node.ID {
		t.Errorf("edge from should be new node ID, got %q", result.Edge.From)
	}
	if result.Edge.To != "existing-node-id" {
		t.Errorf("edge to should be existing-node-id, got %q", result.Edge.To)
	}
	if result.Edge.Type != string(types.EdgeRelated) {
		t.Errorf("expected edge type related, got %q", result.Edge.Type)
	}
}

func TestCaptureBar_NoEdgeWhenNoNodeSelected(t *testing.T) {
	store := newCaptureStore()
	bar := tui.NewCaptureBar(store, fixedCaptureClock())

	bar.Focus("") // no selected node
	bar.SetInput("Standalone task")

	result, err := bar.Submit()
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if result.Edge != nil {
		t.Error("expected no edge when no node is selected")
	}
}

func TestCaptureBar_FocusAndBlur(t *testing.T) {
	store := newCaptureStore()
	bar := tui.NewCaptureBar(store, fixedCaptureClock())

	if bar.IsFocused() {
		t.Error("bar should not be focused initially")
	}

	bar.Focus("")
	if !bar.IsFocused() {
		t.Error("bar should be focused after Focus()")
	}

	bar.Blur()
	if bar.IsFocused() {
		t.Error("bar should not be focused after Blur()")
	}
}

func TestCaptureBar_AppendAndBackspace(t *testing.T) {
	store := newCaptureStore()
	bar := tui.NewCaptureBar(store, fixedCaptureClock())

	bar.AppendRune('H')
	bar.AppendRune('i')
	bar.AppendRune('!')

	if bar.Input() != "Hi!" {
		t.Errorf("unexpected input: %q", bar.Input())
	}

	bar.Backspace()

	if bar.Input() != "Hi" {
		t.Errorf("unexpected input after backspace: %q", bar.Input())
	}
}

func TestCaptureBar_ResetAfterSubmit(t *testing.T) {
	store := newCaptureStore()
	bar := tui.NewCaptureBar(store, fixedCaptureClock())

	bar.Focus("some-node")
	bar.SetInput("task body")

	_, err := bar.Submit()
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	// After submission the bar should be cleared and unfocused.
	if bar.IsFocused() {
		t.Error("bar should not be focused after submit")
	}
	if bar.Input() != "" {
		t.Errorf("input should be empty after submit, got %q", bar.Input())
	}
}
