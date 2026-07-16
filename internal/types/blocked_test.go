package types_test

import (
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// mockGraph is a minimal types.GraphIndex test double. EvalBlockers/Blockers
// only need GetNode and EdgesTo; the remaining methods are unused stubs
// present to satisfy the interface.
type mockGraph struct {
	nodes []*types.Node
	edges []*types.Edge
}

func (m *mockGraph) GetNode(id string) (*types.Node, error) {
	for _, n := range m.nodes {
		if n.ID == id {
			return n, nil
		}
	}
	return nil, &types.NotFoundError{Kind: "node", ID: id}
}

func (m *mockGraph) GetEdge(id string) (*types.Edge, error) {
	for _, e := range m.edges {
		if e.ID == id {
			return e, nil
		}
	}
	return nil, &types.NotFoundError{Kind: "edge", ID: id}
}

func (m *mockGraph) AllNodes() []*types.Node { return m.nodes }
func (m *mockGraph) AllEdges() []*types.Edge { return m.edges }

func (m *mockGraph) EdgesFrom(nodeID string) []*types.Edge {
	var out []*types.Edge
	for _, e := range m.edges {
		if e.From == nodeID {
			out = append(out, e)
		}
	}
	return out
}

func (m *mockGraph) EdgesTo(nodeID string) []*types.Edge {
	var out []*types.Edge
	for _, e := range m.edges {
		if e.To == nodeID {
			out = append(out, e)
		}
	}
	return out
}

func (m *mockGraph) NodesByType(typeName string) []*types.Node {
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

// blockedTestRegistries returns a Kind + StageGroup registry pair with a
// single "task" kind on a terminate-cycle "task-flow" group, mirroring the
// query engine's taskFlowRegistries fixture (internal/query/evaluator_test.go)
// so blocked/terminal semantics stay consistent across both call sites.
func blockedTestRegistries() (*types.KindRegistry, *types.StageGroupRegistry) {
	groups := types.NewStageGroupRegistry([]types.StageGroup{
		{
			Name:   "task-flow",
			Stages: []string{"Open", "Maybe", "Later", "Soon", "Now", "Done"},
			Cycle:  types.CycleTerminate,
		},
	})
	kinds := types.NewKindRegistry([]types.Kind{
		{Name: "task", StageGroup: "task-flow"},
	})
	return kinds, groups
}

func TestEvalBlockers_NonTerminalBlocker(t *testing.T) {
	blocker := &types.Node{ID: "b1", Kind: "task", Stage: "Now"}
	node := &types.Node{ID: "n1", Kind: "task", Stage: "Open"}
	edge := &types.Edge{ID: "e1", Type: string(types.EdgeBlocks), From: "b1", To: "n1"}
	graph := &mockGraph{nodes: []*types.Node{blocker, node}, edges: []*types.Edge{edge}}
	kinds, groups := blockedTestRegistries()

	blocked, unresolved := types.EvalBlockers(graph, kinds, groups, node)
	if !blocked {
		t.Error("expected blocked=true for a non-terminal blocker")
	}
	if unresolved {
		t.Error("expected unresolved=false for a confirmed non-terminal blocker")
	}
}

func TestEvalBlockers_TerminalBlockerLifts(t *testing.T) {
	blocker := &types.Node{ID: "b1", Kind: "task", Stage: "Done"}
	node := &types.Node{ID: "n1", Kind: "task", Stage: "Open"}
	edge := &types.Edge{ID: "e1", Type: string(types.EdgeBlocks), From: "b1", To: "n1"}
	graph := &mockGraph{nodes: []*types.Node{blocker, node}, edges: []*types.Edge{edge}}
	kinds, groups := blockedTestRegistries()

	blocked, unresolved := types.EvalBlockers(graph, kinds, groups, node)
	if blocked {
		t.Error("expected blocked=false when the only blocker is terminal")
	}
	if unresolved {
		t.Error("expected unresolved=false")
	}
}

func TestEvalBlockers_NoIncomingBlocksEdge(t *testing.T) {
	node := &types.Node{ID: "n1", Kind: "task", Stage: "Open"}
	graph := &mockGraph{nodes: []*types.Node{node}}
	kinds, groups := blockedTestRegistries()

	blocked, unresolved := types.EvalBlockers(graph, kinds, groups, node)
	if blocked || unresolved {
		t.Errorf("expected blocked=false, unresolved=false with no blocks edges; got blocked=%v unresolved=%v", blocked, unresolved)
	}
}

func TestEvalBlockers_DanglingEdgePresenceBlocks(t *testing.T) {
	node := &types.Node{ID: "n1", Kind: "task", Stage: "Open"}
	// Edge references a blocker node that doesn't exist in the graph.
	edge := &types.Edge{ID: "e1", Type: string(types.EdgeBlocks), From: "missing", To: "n1"}
	graph := &mockGraph{nodes: []*types.Node{node}, edges: []*types.Edge{edge}}
	kinds, groups := blockedTestRegistries()

	blocked, unresolved := types.EvalBlockers(graph, kinds, groups, node)
	if !blocked {
		t.Error("expected blocked=true for a dangling blocks edge (presence blocks)")
	}
	if !unresolved {
		t.Error("expected unresolved=true for a dangling blocks edge")
	}
}

func TestEvalBlockers_UnknownKindPresenceBlocks(t *testing.T) {
	// Blocker has no Kind set — untriaged, can't resolve a stage group.
	blocker := &types.Node{ID: "b1"}
	node := &types.Node{ID: "n1", Kind: "task", Stage: "Open"}
	edge := &types.Edge{ID: "e1", Type: string(types.EdgeBlocks), From: "b1", To: "n1"}
	graph := &mockGraph{nodes: []*types.Node{blocker, node}, edges: []*types.Edge{edge}}
	kinds, groups := blockedTestRegistries()

	blocked, unresolved := types.EvalBlockers(graph, kinds, groups, node)
	if !blocked {
		t.Error("expected blocked=true when the blocker's kind is unresolvable (presence blocks)")
	}
	if !unresolved {
		t.Error("expected unresolved=true when the blocker's kind is unresolvable")
	}
}

func TestEvalBlockers_NilRegistriesPresenceBlocks(t *testing.T) {
	// Well-formed terminal blocker, but registries aren't wired up at all —
	// terminality can never be confirmed, so the block still holds.
	blocker := &types.Node{ID: "b1", Kind: "task", Stage: "Done"}
	node := &types.Node{ID: "n1", Kind: "task", Stage: "Open"}
	edge := &types.Edge{ID: "e1", Type: string(types.EdgeBlocks), From: "b1", To: "n1"}
	graph := &mockGraph{nodes: []*types.Node{blocker, node}, edges: []*types.Edge{edge}}

	blocked, unresolved := types.EvalBlockers(graph, nil, nil, node)
	if !blocked {
		t.Error("expected blocked=true when registries are nil (presence blocks)")
	}
	if !unresolved {
		t.Error("expected unresolved=true when registries are nil")
	}
}

func TestEvalBlockers_MultipleBlockersMixed(t *testing.T) {
	// One terminal, one non-terminal: overall still blocked, not unresolved.
	terminalBlocker := &types.Node{ID: "b1", Kind: "task", Stage: "Done"}
	activeBlocker := &types.Node{ID: "b2", Kind: "task", Stage: "Later"}
	node := &types.Node{ID: "n1", Kind: "task", Stage: "Open"}
	edges := []*types.Edge{
		{ID: "e1", Type: string(types.EdgeBlocks), From: "b1", To: "n1"},
		{ID: "e2", Type: string(types.EdgeBlocks), From: "b2", To: "n1"},
	}
	graph := &mockGraph{nodes: []*types.Node{terminalBlocker, activeBlocker, node}, edges: edges}
	kinds, groups := blockedTestRegistries()

	blocked, unresolved := types.EvalBlockers(graph, kinds, groups, node)
	if !blocked {
		t.Error("expected blocked=true with one non-terminal blocker present")
	}
	if unresolved {
		t.Error("expected unresolved=false when the block rests on a confirmed non-terminal blocker")
	}
}

func TestEvalBlockers_NonBlocksEdgeIgnored(t *testing.T) {
	other := &types.Node{ID: "b1", Kind: "task", Stage: "Now"}
	node := &types.Node{ID: "n1", Kind: "task", Stage: "Open"}
	edge := &types.Edge{ID: "e1", Type: "related", From: "b1", To: "n1"}
	graph := &mockGraph{nodes: []*types.Node{other, node}, edges: []*types.Edge{edge}}
	kinds, groups := blockedTestRegistries()

	blocked, _ := types.EvalBlockers(graph, kinds, groups, node)
	if blocked {
		t.Error("expected blocked=false — a non-'blocks' edge type must not count")
	}
}

func TestEvalBlockers_NilNodeOrIndex(t *testing.T) {
	kinds, groups := blockedTestRegistries()
	if blocked, unresolved := types.EvalBlockers(&mockGraph{}, kinds, groups, nil); blocked || unresolved {
		t.Error("expected false, false for a nil node")
	}
	node := &types.Node{ID: "n1"}
	if blocked, unresolved := types.EvalBlockers(nil, kinds, groups, node); blocked || unresolved {
		t.Error("expected false, false for a nil index")
	}
}

func TestBlockers_ReturnsNonTerminalAndUnresolvable(t *testing.T) {
	terminalBlocker := &types.Node{ID: "b1", Kind: "task", Stage: "Done"}
	activeBlocker := &types.Node{ID: "b2", Kind: "task", Stage: "Later"}
	unresolvableBlocker := &types.Node{ID: "b3"} // no Kind set
	node := &types.Node{ID: "n1", Kind: "task", Stage: "Open"}
	edges := []*types.Edge{
		{ID: "e1", Type: string(types.EdgeBlocks), From: "b1", To: "n1"},
		{ID: "e2", Type: string(types.EdgeBlocks), From: "b2", To: "n1"},
		{ID: "e3", Type: string(types.EdgeBlocks), From: "b3", To: "n1"},
	}
	graph := &mockGraph{nodes: []*types.Node{terminalBlocker, activeBlocker, unresolvableBlocker, node}, edges: edges}
	kinds, groups := blockedTestRegistries()

	got := types.Blockers(graph, kinds, groups, node)
	if len(got) != 2 {
		t.Fatalf("expected 2 blockers (terminal one excluded), got %d", len(got))
	}
	ids := map[string]bool{got[0].ID: true, got[1].ID: true}
	if !ids["b2"] || !ids["b3"] {
		t.Errorf("expected blockers b2 and b3, got %v", ids)
	}
	if ids["b1"] {
		t.Error("terminal blocker b1 must not appear in Blockers")
	}
}

func TestBlockers_NoBlockers(t *testing.T) {
	node := &types.Node{ID: "n1", Kind: "task", Stage: "Open"}
	graph := &mockGraph{nodes: []*types.Node{node}}
	kinds, groups := blockedTestRegistries()

	got := types.Blockers(graph, kinds, groups, node)
	if len(got) != 0 {
		t.Errorf("expected no blockers, got %d", len(got))
	}
}

func TestBlockers_DanglingEdgeOmitted(t *testing.T) {
	node := &types.Node{ID: "n1", Kind: "task", Stage: "Open"}
	edge := &types.Edge{ID: "e1", Type: string(types.EdgeBlocks), From: "missing", To: "n1"}
	graph := &mockGraph{nodes: []*types.Node{node}, edges: []*types.Edge{edge}}
	kinds, groups := blockedTestRegistries()

	got := types.Blockers(graph, kinds, groups, node)
	if len(got) != 0 {
		t.Errorf("expected 0 blocker nodes for a dangling edge (nothing to return), got %d", len(got))
	}
}
