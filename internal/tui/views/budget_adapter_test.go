package views

import (
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// stubBudgetIndex is a minimal types.GraphIndex for adapter unit tests —
// only GetNode is exercised by NodesFromQueryResult, but the interface
// requires every method.
type stubBudgetIndex struct {
	nodes []*types.Node
}

func (s *stubBudgetIndex) GetNode(id string) (*types.Node, error) {
	for _, n := range s.nodes {
		if n.ID == id {
			return n, nil
		}
	}
	return nil, &types.NotFoundError{Kind: "node", ID: id}
}
func (s *stubBudgetIndex) GetEdge(id string) (*types.Edge, error) {
	return nil, &types.NotFoundError{Kind: "edge", ID: id}
}
func (s *stubBudgetIndex) AllNodes() []*types.Node               { return s.nodes }
func (s *stubBudgetIndex) AllEdges() []*types.Edge               { return nil }
func (s *stubBudgetIndex) EdgesFrom(nodeID string) []*types.Edge { return nil }
func (s *stubBudgetIndex) EdgesTo(nodeID string) []*types.Edge   { return nil }
func (s *stubBudgetIndex) NodesByType(typeName string) []*types.Node {
	var out []*types.Node
	for _, n := range s.nodes {
		for _, t := range n.Types {
			if t == typeName {
				out = append(out, n)
			}
		}
	}
	return out
}

func TestNodesFromQueryResult_HydratesByID(t *testing.T) {
	budget := &types.Node{ID: "budget-1", Title: "Groceries", Types: []string{"budget"}}
	index := &stubBudgetIndex{nodes: []*types.Node{budget}}

	result := types.QueryResult{
		Rows: []map[string]interface{}{
			{"id": "budget-1", "title": "Groceries"},
		},
	}

	nodes := NodesFromQueryResult(result, index, "")
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0] != budget {
		t.Errorf("expected the hydrated node to be the same pointer index.GetNode returns")
	}
}

func TestNodesFromQueryResult_SkipsUnresolvableID(t *testing.T) {
	index := &stubBudgetIndex{nodes: []*types.Node{
		{ID: "budget-1", Title: "Groceries"},
	}}
	result := types.QueryResult{
		Rows: []map[string]interface{}{
			{"id": "budget-1"},
			{"id": "does-not-exist"},
		},
	}

	nodes := NodesFromQueryResult(result, index, "")
	if len(nodes) != 1 {
		t.Fatalf("expected 1 resolved node, got %d", len(nodes))
	}
	if nodes[0].ID != "budget-1" {
		t.Errorf("ID = %q, want budget-1", nodes[0].ID)
	}
}

func TestNodesFromQueryResult_SkipsMissingIDColumn(t *testing.T) {
	index := &stubBudgetIndex{nodes: []*types.Node{{ID: "budget-1"}}}
	result := types.QueryResult{
		Rows: []map[string]interface{}{
			{"title": "no id column at all"},
		},
	}

	nodes := NodesFromQueryResult(result, index, "")
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for a row with no id column, got %d", len(nodes))
	}
}

func TestNodesFromQueryResult_CustomIDColumn(t *testing.T) {
	index := &stubBudgetIndex{nodes: []*types.Node{{ID: "budget-1", Title: "Groceries"}}}
	result := types.QueryResult{
		Rows: []map[string]interface{}{
			{"node_id": "budget-1"},
		},
	}

	nodes := NodesFromQueryResult(result, index, "node_id")
	if len(nodes) != 1 || nodes[0].ID != "budget-1" {
		t.Fatalf("expected 1 node budget-1 via custom id column, got %v", nodes)
	}
}

func TestNodesFromQueryResult_NilIndexReturnsEmpty(t *testing.T) {
	result := types.QueryResult{
		Rows: []map[string]interface{}{{"id": "budget-1"}},
	}
	nodes := NodesFromQueryResult(result, nil, "")
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for a nil index, got %d", len(nodes))
	}
}

func TestNodesFromQueryResult_EmptyRows(t *testing.T) {
	index := &stubBudgetIndex{}
	nodes := NodesFromQueryResult(types.QueryResult{}, index, "")
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for an empty result, got %d", len(nodes))
	}
}
