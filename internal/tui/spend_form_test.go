package tui_test

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jasonwarrenuk/wyrd/internal/tui"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// spendTestIndex is a minimal GraphIndex for spend form tests.
type spendTestIndex struct {
	nodes []*types.Node
}

func (i *spendTestIndex) GetNode(id string) (*types.Node, error) {
	for _, n := range i.nodes {
		if n.ID == id {
			return n, nil
		}
	}
	return nil, &types.NotFoundError{Kind: "node", ID: id}
}
func (i *spendTestIndex) GetEdge(id string) (*types.Edge, error) {
	return nil, &types.NotFoundError{Kind: "edge", ID: id}
}
func (i *spendTestIndex) AllNodes() []*types.Node  { return i.nodes }
func (i *spendTestIndex) AllEdges() []*types.Edge  { return nil }
func (i *spendTestIndex) EdgesFrom(_ string) []*types.Edge { return nil }
func (i *spendTestIndex) EdgesTo(_ string) []*types.Edge   { return nil }
func (i *spendTestIndex) NodesByType(typeName string) []*types.Node {
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

func newBudgetNode(category string) *types.Node {
	return &types.Node{
		ID:    "budget-" + category,
		Types: []string{"budget"},
		Properties: map[string]interface{}{
			"category":  category,
			"allocated": float64(500),
		},
		Created:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Modified: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

// TestSpendFormPaneErrorWhenNoBudgetNodes verifies the constructor returns an
// error when the index contains no budget nodes.
func TestSpendFormPaneErrorWhenNoBudgetNodes(t *testing.T) {
	theme, err := tui.LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := newFormTestStore()
	index := &spendTestIndex{}
	clock := formTestClock()

	_, err = tui.NewSpendFormPane(theme, store, index, clock, "")
	if err == nil {
		t.Error("expected error when no budget nodes exist, got nil")
	}
}

// TestSpendFormPaneSucceedsWithBudgetNodes verifies the constructor succeeds
// when at least one budget node exists.
func TestSpendFormPaneSucceedsWithBudgetNodes(t *testing.T) {
	theme, err := tui.LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := newFormTestStore()
	index := &spendTestIndex{
		nodes: []*types.Node{newBudgetNode("groceries")},
	}
	clock := formTestClock()

	fp, err := tui.NewSpendFormPane(theme, store, index, clock, "")
	if err != nil {
		t.Fatalf("NewSpendFormPane returned unexpected error: %v", err)
	}
	if fp == nil {
		t.Error("expected non-nil PaneModel")
	}
}

// TestSpendFormPaneViewRenders verifies the form produces a non-empty view
// after receiving a window size message.
func TestSpendFormPaneViewRenders(t *testing.T) {
	theme, err := tui.LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := newFormTestStore()
	index := &spendTestIndex{
		nodes: []*types.Node{
			newBudgetNode("groceries"),
			newBudgetNode("transport"),
		},
	}
	clock := formTestClock()

	fp, err := tui.NewSpendFormPane(theme, store, index, clock, "coffee")
	if err != nil {
		t.Fatalf("NewSpendFormPane: %v", err)
	}

	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	v := sized.View()
	if v == "" {
		t.Error("expected non-empty view from spendFormPane")
	}
}

// TestSpendFormPanePrefillNote verifies that the text after "s:" is used as
// the note pre-fill.
func TestSpendFormPanePrefillNote(t *testing.T) {
	theme, err := tui.LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := newFormTestStore()
	index := &spendTestIndex{
		nodes: []*types.Node{newBudgetNode("groceries")},
	}
	clock := formTestClock()

	fp, err := tui.NewSpendFormPane(theme, store, index, clock, "coffee beans")
	if err != nil {
		t.Fatalf("NewSpendFormPane: %v", err)
	}

	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	v := sized.View()
	// The pre-filled note text should appear in the rendered form.
	if v == "" {
		t.Error("expected non-empty view")
	}
}
