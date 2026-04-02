package budget

import (
	"strings"
	"testing"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// --- Mock implementations ---

// mockStore implements types.StoreFS for testing. Only WriteNode is exercised here.
type mockStore struct {
	written []*types.Node
}

func (m *mockStore) ReadNode(id string) (*types.Node, error)           { return nil, nil }
func (m *mockStore) WriteNode(node *types.Node) error                  { m.written = append(m.written, node); return nil }
func (m *mockStore) ReadEdge(id string) (*types.Edge, error)           { return nil, nil }
func (m *mockStore) WriteEdge(edge *types.Edge) error                  { return nil }
func (m *mockStore) DeleteEdge(id string) error                        { return nil }
func (m *mockStore) ArchiveNode(_ string) error                        { return nil }
func (m *mockStore) ReadTemplate(typeName string) (*types.Template, error) { return nil, nil }
func (m *mockStore) AllTemplates() ([]*types.Template, error)           { return nil, nil }
func (m *mockStore) ReadView(name string) (*types.SavedView, error)    { return nil, nil }
func (m *mockStore) AllViews() ([]*types.SavedView, error)             { return nil, nil }
func (m *mockStore) ReadRitual(name string) (*types.Ritual, error)     { return nil, nil }
func (m *mockStore) AllRituals() ([]*types.Ritual, error)              { return nil, nil }
func (m *mockStore) ReadTheme(name string) (*types.Theme, error)       { return nil, nil }
func (m *mockStore) ReadConfig() (*types.Config, error)                { return nil, nil }
func (m *mockStore) WriteConfig(cfg *types.Config) error               { return nil }
func (m *mockStore) StorePath() string                                 { return "/tmp/test-store" }

// mockIndex implements types.GraphIndex for testing.
type mockIndex struct {
	nodes map[string]*types.Node
}

func newMockIndex(nodes ...*types.Node) *mockIndex {
	m := &mockIndex{nodes: make(map[string]*types.Node)}
	for _, n := range nodes {
		m.nodes[n.ID] = n
	}
	return m
}

func (m *mockIndex) GetNode(id string) (*types.Node, error) {
	n, ok := m.nodes[id]
	if !ok {
		return nil, &types.NotFoundError{Kind: "node", ID: id}
	}
	return n, nil
}

func (m *mockIndex) GetEdge(id string) (*types.Edge, error) { return nil, nil }
func (m *mockIndex) AllNodes() []*types.Node {
	out := make([]*types.Node, 0, len(m.nodes))
	for _, n := range m.nodes {
		out = append(out, n)
	}
	return out
}
func (m *mockIndex) AllEdges() []*types.Edge      { return nil }
func (m *mockIndex) EdgesFrom(nodeID string) []*types.Edge { return nil }
func (m *mockIndex) EdgesTo(nodeID string) []*types.Edge   { return nil }

func (m *mockIndex) NodesByType(typeName string) []*types.Node {
	var out []*types.Node
	for _, n := range m.nodes {
		for _, t := range n.Types {
			if t == typeName {
				out = append(out, n)
				break
			}
		}
	}
	return out
}

// --- Helpers ---

func newBudgetNodeForSpend(id, category string, entries []types.SpendEntry) *types.Node {
	props := map[string]interface{}{
		"category":  category,
		"allocated": float64(200),
		"warn_at":   0.8,
		"period":    "month",
	}
	if entries != nil {
		props["spend_log"] = entries
	}
	return &types.Node{
		ID:         id,
		Body:       "Test budget",
		Types:      []string{"budget"},
		Properties: props,
	}
}

var spendNow = time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)

// --- Tests ---

func TestRecordSpend_AppendsEntry(t *testing.T) {
	node := newBudgetNodeForSpend("b1", "groceries", nil)
	store := &mockStore{}
	index := newMockIndex(node)

	err := RecordSpend(store, index, "groceries", 42.50, "weekly shop", spendNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(store.written) != 1 {
		t.Fatalf("expected 1 written node, got %d", len(store.written))
	}
	written := store.written[0]
	entries := spendLog(written)
	if len(entries) != 1 {
		t.Fatalf("expected 1 spend entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Amount != 42.50 {
		t.Errorf("expected amount 42.50, got %g", e.Amount)
	}
	if e.Note != "weekly shop" {
		t.Errorf("expected note 'weekly shop', got %q", e.Note)
	}
	if e.Date != "2026-03-15" {
		t.Errorf("expected date 2026-03-15, got %q", e.Date)
	}
}

func TestRecordSpend_AppendsToExistingLog(t *testing.T) {
	existing := []types.SpendEntry{
		{Date: "2026-03-10", Amount: 20, Note: "prior"},
	}
	node := newBudgetNodeForSpend("b2", "transport", existing)
	store := &mockStore{}
	index := newMockIndex(node)

	err := RecordSpend(store, index, "transport", 15, "bus pass", spendNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries := spendLog(store.written[0])
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries after append, got %d", len(entries))
	}
}

func TestRecordSpend_BumpsModified(t *testing.T) {
	originalModified := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	node := newBudgetNodeForSpend("b3", "dining", nil)
	node.Modified = originalModified

	store := &mockStore{}
	index := newMockIndex(node)

	err := RecordSpend(store, index, "dining", 30, "lunch", spendNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	written := store.written[0]
	if !written.Modified.Equal(spendNow) {
		t.Errorf("expected Modified to be bumped to %v, got %v", spendNow, written.Modified)
	}
}

func TestRecordSpend_CategoryNotFound(t *testing.T) {
	node := newBudgetNodeForSpend("b4", "groceries", nil)
	store := &mockStore{}
	index := newMockIndex(node)

	err := RecordSpend(store, index, "nonexistent-category", 10, "test", spendNow)
	if err == nil {
		t.Fatal("expected an error for unknown category, got nil")
	}

	notFound, ok := err.(*types.NotFoundError)
	if !ok {
		t.Fatalf("expected NotFoundError, got %T: %v", err, err)
	}
	if notFound.ID != "nonexistent-category" {
		t.Errorf("expected ID 'nonexistent-category', got %q", notFound.ID)
	}
}

func TestRecordSpend_CategoryNotFound_ListsAvailable(t *testing.T) {
	grocery := newBudgetNodeForSpend("b-g", "groceries", nil)
	transport := newBudgetNodeForSpend("b-t", "transport", nil)
	store := &mockStore{}
	index := newMockIndex(grocery, transport)

	err := RecordSpend(store, index, "nonexistent", 10, "test", spendNow)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	notFound, ok := err.(*types.NotFoundError)
	if !ok {
		t.Fatalf("expected NotFoundError, got %T: %v", err, err)
	}

	msg := notFound.Error()
	for _, cat := range []string{"groceries", "transport"} {
		if !strings.Contains(msg, cat) {
			t.Errorf("expected error message to mention %q, got:\n%s", cat, msg)
		}
	}
	if !strings.Contains(msg, "Available categories") {
		t.Errorf("expected 'Available categories' in error message, got:\n%s", msg)
	}
}

func TestRecordSpend_CategoryNotFound_NoBudgetNodes(t *testing.T) {
	store := &mockStore{}
	index := newMockIndex() // no nodes at all

	err := RecordSpend(store, index, "anything", 10, "test", spendNow)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "No budget categories found") {
		t.Errorf("expected 'No budget categories found' in error message, got:\n%s", msg)
	}
}

func TestRecordSpend_InvalidAmount_Zero(t *testing.T) {
	node := newBudgetNodeForSpend("b5", "groceries", nil)
	store := &mockStore{}
	index := newMockIndex(node)

	err := RecordSpend(store, index, "groceries", 0, "test", spendNow)
	if err == nil {
		t.Fatal("expected validation error for zero amount, got nil")
	}

	_, ok := err.(*types.ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestRecordSpend_InvalidAmount_Negative(t *testing.T) {
	node := newBudgetNodeForSpend("b6", "groceries", nil)
	store := &mockStore{}
	index := newMockIndex(node)

	err := RecordSpend(store, index, "groceries", -5, "refund", spendNow)
	if err == nil {
		t.Fatal("expected validation error for negative amount, got nil")
	}
}

func TestRecordSpend_PeriodFiltering(t *testing.T) {
	// Budget period is "month". RecordSpend appends; Compute should only sum current month.
	existing := []types.SpendEntry{
		{Date: "2026-02-15", Amount: 999, Note: "old"},
	}
	node := newBudgetNodeForSpend("b7", "fitness", existing)
	store := &mockStore{}
	index := newMockIndex(node)

	err := RecordSpend(store, index, "fitness", 50, "gym membership", spendNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	written := store.written[0]
	summary := Compute(written, spendNow)

	// Only the March entry should count.
	if summary.Spent != 50 {
		t.Errorf("expected period-filtered spend of 50, got %g", summary.Spent)
	}
}
