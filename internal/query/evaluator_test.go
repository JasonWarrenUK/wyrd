package query

import (
	"testing"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// ---------------------------------------------------------------------------
// Mock graph index for tests
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Test fixtures
// ---------------------------------------------------------------------------

func makeNode(id, body string, nodeTypes []string, props map[string]interface{}) *types.Node {
	n := &types.Node{
		ID:         id,
		Body:       body,
		Types:      nodeTypes,
		Created:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Modified:   time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
		Properties: props,
	}
	return n
}

func makeEdge(id, edgeType, from, to string) *types.Edge {
	return &types.Edge{
		ID:      id,
		Type:    edgeType,
		From:    from,
		To:      to,
		Created: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

func buildTestGraph() *mockGraph {
	nodes := []*types.Node{
		makeNode("n1", "Write tests", []string{"task"}, map[string]interface{}{
			"status":   "open",
			"priority": int64(1),
		}),
		makeNode("n2", "Review PR", []string{"task"}, map[string]interface{}{
			"status":   "open",
			"priority": int64(2),
		}),
		makeNode("n3", "Deploy to prod", []string{"task"}, map[string]interface{}{
			"status":   "done",
			"priority": int64(3),
		}),
		makeNode("n4", "Project Alpha", []string{"project"}, map[string]interface{}{
			"status": "active",
		}),
	}

	edges := []*types.Edge{
		makeEdge("e1", "blocks", "n1", "n2"),
		makeEdge("e2", "parent", "n4", "n1"),
		makeEdge("e3", "parent", "n4", "n2"),
	}

	return &mockGraph{nodes: nodes, edges: edges}
}

func newTestEngine(graph types.GraphIndex) *Engine {
	return NewEngine(graph, 5)
}

// ---------------------------------------------------------------------------
// Basic queries
// ---------------------------------------------------------------------------

func TestEngine_AllNodes(t *testing.T) {
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	result, err := e.Run(`MATCH (n) RETURN n`, clock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Rows) != 4 {
		t.Errorf("expected 4 rows, got %d", len(result.Rows))
	}
}

func TestEngine_NodesByType(t *testing.T) {
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	result, err := e.Run(`MATCH (n:task) RETURN n`, clock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Rows) != 3 {
		t.Errorf("expected 3 task nodes, got %d", len(result.Rows))
	}
}

func TestEngine_WhereFilter(t *testing.T) {
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	result, err := e.Run(`MATCH (n:task) WHERE n.status = "open" RETURN n`, clock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 open tasks, got %d", len(result.Rows))
	}
}

func TestEngine_PropertyInReturn(t *testing.T) {
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	result, err := e.Run(`MATCH (n:task) WHERE n.status = "done" RETURN n.body, n.status`, clock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
	row := result.Rows[0]
	if row["n.status"] != "done" {
		t.Errorf("expected status 'done', got %v", row["n.status"])
	}
}

func TestEngine_Columns(t *testing.T) {
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	result, err := e.Run(`MATCH (n:task) RETURN n.body AS title, n.status AS status`, clock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %v", result.Columns)
	}
	if result.Columns[0] != "title" || result.Columns[1] != "status" {
		t.Errorf("unexpected columns: %v", result.Columns)
	}
}

// ---------------------------------------------------------------------------
// Edge traversal
// ---------------------------------------------------------------------------

func TestEngine_OutgoingEdge(t *testing.T) {
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	// n1 blocks n2
	result, err := e.Run(`MATCH (a:task)-[:blocks]->(b:task) RETURN a, b`, clock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row for blocks edge, got %d", len(result.Rows))
	}
}

func TestEngine_IncomingEdge(t *testing.T) {
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	// Tasks that block n2
	result, err := e.Run(`MATCH (t:task)<-[:blocks]-(b) RETURN t, b`, clock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row for incoming blocks edge, got %d", len(result.Rows))
	}
}

func TestEngine_UndirectedEdge(t *testing.T) {
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	// Any edge between tasks
	result, err := e.Run(`MATCH (a:task)--(b:task) RETURN a, b`, clock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// n1-n2 and n2-n1 (undirected)
	if len(result.Rows) < 1 {
		t.Errorf("expected at least 1 row for undirected edge, got %d", len(result.Rows))
	}
}

func TestEngine_ParentEdge(t *testing.T) {
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	// Project's children
	result, err := e.Run(`MATCH (p:project)-[:parent]->(child:task) RETURN p, child`, clock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 child tasks, got %d", len(result.Rows))
	}
}

func TestEngine_AnyEdgeType(t *testing.T) {
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	// n4 has parent edges to n1 and n2
	result, err := e.Run(`MATCH (p:project)-[]->(child) RETURN p, child`, clock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 rows for any-edge traversal from project, got %d", len(result.Rows))
	}
}

// ---------------------------------------------------------------------------
// Variable-length path
// ---------------------------------------------------------------------------

func TestEngine_VarLengthPath(t *testing.T) {
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	// n4 ->parent-> n1 ->blocks-> n2; path length 1 or 2
	result, err := e.Run(`MATCH (p:project)-[*1..2]->(d) RETURN p, d`, clock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should find n1, n2 at depth 1, and n2 again via n1-blocks->n2 at depth 2.
	if len(result.Rows) == 0 {
		t.Error("expected at least one result for variable-length path")
	}
}

func TestEngine_VarLengthPath_DepthExceeded(t *testing.T) {
	g := buildTestGraph()
	e := NewEngine(g, 3) // max depth = 3
	clock := fixedClock(refTime)

	_, err := e.Run(`MATCH (a)-[*1..10]->(b) RETURN a, b`, clock)
	if err == nil {
		t.Fatal("expected PathDepthError when requested depth exceeds max")
	}
	if _, ok := err.(*PathDepthError); !ok {
		t.Errorf("expected PathDepthError, got %T: %v", err, err)
	}
}

// ---------------------------------------------------------------------------
// Aggregation
// ---------------------------------------------------------------------------

func TestEngine_Count(t *testing.T) {
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	result, err := e.Run(`MATCH (n:task) RETURN count(n)`, clock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
	if result.Rows[0]["count()"] != int64(3) {
		t.Errorf("expected count 3, got %v", result.Rows[0]["count()"])
	}
}

func TestEngine_CountGroupBy(t *testing.T) {
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	result, err := e.Run(`MATCH (n:task) RETURN n.status, count(n) AS total`, clock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Two statuses: "open" (2) and "done" (1)
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 groups, got %d", len(result.Rows))
	}
}

func TestEngine_SumAggregation(t *testing.T) {
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	result, err := e.Run(`MATCH (n:task) RETURN sum(n.priority)`, clock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
	// priorities: 1 + 2 + 3 = 6
	if result.Rows[0]["sum()"] != int64(6) {
		t.Errorf("expected sum 6, got %v", result.Rows[0]["sum()"])
	}
}

// ---------------------------------------------------------------------------
// ORDER BY and LIMIT
// ---------------------------------------------------------------------------

func TestEngine_OrderByAscending(t *testing.T) {
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	result, err := e.Run(`MATCH (n:task) RETURN n.body, n.priority ORDER BY n.priority`, clock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Rows) < 2 {
		t.Fatalf("expected at least 2 rows, got %d", len(result.Rows))
	}
	// First row should have priority 1.
	first := result.Rows[0]["n.priority"]
	last := result.Rows[len(result.Rows)-1]["n.priority"]
	if compareLess(last, first) {
		t.Error("rows are not sorted ascending by priority")
	}
}

func TestEngine_OrderByDescending(t *testing.T) {
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	result, err := e.Run(`MATCH (n:task) RETURN n.body, n.priority ORDER BY n.priority DESC`, clock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Rows) < 2 {
		t.Fatalf("expected at least 2 rows, got %d", len(result.Rows))
	}
	first := result.Rows[0]["n.priority"]
	last := result.Rows[len(result.Rows)-1]["n.priority"]
	if compareLess(first, last) {
		t.Error("rows are not sorted descending by priority")
	}
}

func TestEngine_Limit(t *testing.T) {
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	result, err := e.Run(`MATCH (n) RETURN n LIMIT 2`, clock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 rows after LIMIT, got %d", len(result.Rows))
	}
}

// ---------------------------------------------------------------------------
// Built-in date variable filtering
// ---------------------------------------------------------------------------

func TestEngine_WhereBuiltinDate(t *testing.T) {
	// Nodes with a due date before today.
	past := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	future := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	n1 := makeNode("d1", "Past task", []string{"task"}, map[string]interface{}{
		"due": past,
	})
	n2 := makeNode("d2", "Future task", []string{"task"}, map[string]interface{}{
		"due": future,
	})

	g := &mockGraph{nodes: []*types.Node{n1, n2}}
	e := newTestEngine(g)
	clock := fixedClock(refTime) // 2025-03-19

	result, err := e.Run(`MATCH (n:task) WHERE n.due < $today RETURN n.body`, clock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("expected 1 past task, got %d", len(result.Rows))
	}
}

// ---------------------------------------------------------------------------
// Error handling
// ---------------------------------------------------------------------------

func TestEngine_ReadOnlyMutation(t *testing.T) {
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	_, err := e.Run(`CREATE (n:task {body: "new"})`, clock)
	if _, ok := err.(*MutationError); !ok {
		t.Errorf("expected MutationError, got %T: %v", err, err)
	}
}

func TestEngine_UnsupportedWith(t *testing.T) {
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	_, err := e.Run(`MATCH (n) WITH n RETURN n`, clock)
	if _, ok := err.(*UnsupportedClauseError); !ok {
		t.Errorf("expected UnsupportedClauseError, got %T: %v", err, err)
	}
}

func TestEngine_NilClock(t *testing.T) {
	g := buildTestGraph()
	e := newTestEngine(g)

	// A nil clock should fall back to RealClock without panicking.
	result, err := e.Run(`MATCH (n:task) RETURN count(n)`, nil)
	if err != nil {
		t.Fatalf("unexpected error with nil clock: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(result.Rows))
	}
}
