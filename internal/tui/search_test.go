package tui

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// stubIndex is a lightweight GraphIndex for search unit tests.
type stubIndex struct {
	nodes []*types.Node
	edges []*types.Edge
}

func (s *stubIndex) GetNode(id string) (*types.Node, error) {
	for _, n := range s.nodes {
		if n.ID == id {
			return n, nil
		}
	}
	return nil, &types.NotFoundError{Kind: "node", ID: id}
}

func (s *stubIndex) GetEdge(id string) (*types.Edge, error) {
	for _, e := range s.edges {
		if e.ID == id {
			return e, nil
		}
	}
	return nil, &types.NotFoundError{Kind: "edge", ID: id}
}

func (s *stubIndex) AllNodes() []*types.Node { return s.nodes }
func (s *stubIndex) AllEdges() []*types.Edge { return s.edges }

func (s *stubIndex) EdgesFrom(nodeID string) []*types.Edge {
	var out []*types.Edge
	for _, e := range s.edges {
		if e.From == nodeID {
			out = append(out, e)
		}
	}
	return out
}

func (s *stubIndex) EdgesTo(nodeID string) []*types.Edge {
	var out []*types.Edge
	for _, e := range s.edges {
		if e.To == nodeID {
			out = append(out, e)
		}
	}
	return out
}

func (s *stubIndex) NodesByType(typeName string) []*types.Node {
	var out []*types.Node
	for _, n := range s.nodes {
		for _, t := range n.Types {
			if t == typeName {
				out = append(out, n)
				break
			}
		}
	}
	return out
}

// helpers

func makeNode(id, title, body string, types_ []string) *types.Node {
	return &types.Node{
		ID:         id,
		Title:      title,
		Body:       body,
		Types:      types_,
		Properties: map[string]interface{}{},
	}
}

func makeEdge(id, edgeType, from, to string) *types.Edge {
	return &types.Edge{
		ID:   id,
		Type: edgeType,
		From: from,
		To:   to,
	}
}

func commands() []Command {
	return []Command{
		{Name: "quit", Description: "Exit Wyrd", Hint: "ctrl+c"},
		{Name: "theme", Description: "Switch to a named theme"},
		{Name: "sync", Description: "Sync with remote"},
		{Name: "help", Description: "Show help overlay"},
	}
}

// ── searchAll tests ─────────────────────────────────────────────────────────

func TestSearchAllEmptyQuery(t *testing.T) {
	idx := &stubIndex{
		nodes: []*types.Node{makeNode("n1", "Buy groceries", "", []string{"task"})},
	}
	results := searchAll("", commands(), idx)

	// All commands returned, no nodes.
	commandCount := 0
	nodeCount := 0
	for _, r := range results {
		switch r.Kind {
		case SearchResultCommand:
			commandCount++
		case SearchResultNode:
			nodeCount++
		}
	}
	if commandCount != 4 {
		t.Errorf("expected 4 commands, got %d", commandCount)
	}
	if nodeCount != 0 {
		t.Errorf("expected 0 nodes on empty query, got %d", nodeCount)
	}
}

func TestSearchAllCommandMatchByName(t *testing.T) {
	results := searchAll("quit", commands(), nil)
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if results[0].Title != "quit" {
		t.Errorf("expected top result to be 'quit', got %q", results[0].Title)
	}
	if results[0].Score != 100 {
		t.Errorf("expected score 100 for exact name match, got %d", results[0].Score)
	}
}

func TestSearchAllCommandMatchByDescription(t *testing.T) {
	results := searchAll("exit", commands(), nil)
	found := false
	for _, r := range results {
		if r.Title == "quit" {
			found = true
			if r.Score != 40 {
				t.Errorf("expected score 40 for description match, got %d", r.Score)
			}
		}
	}
	if !found {
		t.Error("expected 'quit' to be matched via description 'Exit Wyrd'")
	}
}

func TestSearchAllNodeTitleMatch(t *testing.T) {
	idx := &stubIndex{
		nodes: []*types.Node{makeNode("n1", "Buy groceries", "Pick up milk", []string{"task"})},
	}
	results := searchAll("groceries", commands(), idx)
	found := false
	for _, r := range results {
		if r.Kind == SearchResultNode && r.NodeID == "n1" {
			found = true
			if r.Score != 70 {
				t.Errorf("expected score 70 for title match, got %d", r.Score)
			}
		}
	}
	if !found {
		t.Error("expected node to be matched by title")
	}
}

func TestSearchAllNodeBodyMatch(t *testing.T) {
	idx := &stubIndex{
		nodes: []*types.Node{makeNode("n1", "", "Meeting notes from Monday", []string{"note"})},
	}
	results := searchAll("monday", commands(), idx)
	found := false
	for _, r := range results {
		if r.Kind == SearchResultNode && r.NodeID == "n1" {
			found = true
			if r.Score != 30 {
				t.Errorf("expected score 30 for body match, got %d", r.Score)
			}
		}
	}
	if !found {
		t.Error("expected node to be matched by body")
	}
}

func TestSearchAllNodeBodyScoreLowerThanTitle(t *testing.T) {
	idx := &stubIndex{
		nodes: []*types.Node{
			makeNode("n1", "Monday standup", "", []string{"event"}),
			makeNode("n2", "", "Notes from monday", []string{"note"}),
		},
	}
	results := searchAll("monday", commands(), idx)
	var titleScore, bodyScore int
	for _, r := range results {
		if r.NodeID == "n1" {
			titleScore = r.Score
		}
		if r.NodeID == "n2" {
			bodyScore = r.Score
		}
	}
	if titleScore <= bodyScore {
		t.Errorf("expected title score (%d) > body score (%d)", titleScore, bodyScore)
	}
}

func TestSearchAllNodeTypeMatch(t *testing.T) {
	idx := &stubIndex{
		nodes: []*types.Node{makeNode("n1", "", "Some body text", []string{"task"})},
	}
	results := searchAll("task", commands(), idx)
	found := false
	for _, r := range results {
		if r.Kind == SearchResultNode && r.NodeID == "n1" {
			found = true
			if r.Score != 50 {
				t.Errorf("expected score 50 for type match, got %d", r.Score)
			}
		}
	}
	if !found {
		t.Error("expected node to be matched by type")
	}
}

func TestSearchAllEdgeMatch(t *testing.T) {
	idx := &stubIndex{
		nodes: []*types.Node{
			makeNode("n1", "Fix the bug", "", []string{"task"}),
			makeNode("n2", "Deploy release", "", []string{"task"}),
		},
		edges: []*types.Edge{makeEdge("e1", "blocks", "n1", "n2")},
	}
	results := searchAll("blocks", commands(), idx)
	found := false
	for _, r := range results {
		if r.Kind == SearchResultEdge && r.EdgeID == "e1" {
			found = true
			if r.Score != 50 {
				t.Errorf("expected score 50 for edge type match, got %d", r.Score)
			}
		}
	}
	if !found {
		t.Error("expected edge to be matched by type")
	}
}

func TestSearchAllEdgeMatchByConnectedNodeTitle(t *testing.T) {
	idx := &stubIndex{
		nodes: []*types.Node{
			makeNode("n1", "Fix the bug", "", []string{"task"}),
			makeNode("n2", "Deploy release", "", []string{"task"}),
		},
		edges: []*types.Edge{makeEdge("e1", "precedes", "n1", "n2")},
	}
	results := searchAll("deploy", commands(), idx)
	found := false
	for _, r := range results {
		if r.Kind == SearchResultEdge && r.EdgeID == "e1" {
			found = true
			if r.Score != 40 {
				t.Errorf("expected score 40 for edge match via connected node title, got %d", r.Score)
			}
		}
	}
	if !found {
		t.Error("expected edge to be matched via connected node title")
	}
}

func TestSearchAllRanking(t *testing.T) {
	idx := &stubIndex{
		nodes: []*types.Node{
			makeNode("n1", "theme settings", "", []string{"config"}),
			makeNode("n2", "", "change the theme here", []string{"note"}),
		},
	}
	// "theme" is an exact command name match (score 100).
	results := searchAll("theme", commands(), idx)

	if len(results) < 3 {
		t.Fatalf("expected at least 3 results, got %d", len(results))
	}
	// Verify ordering: command first (score 100), then node title (score 70), then body (score 30).
	if results[0].Kind != SearchResultCommand || results[0].Title != "theme" {
		t.Errorf("expected first result to be 'theme' command, got kind=%d title=%q", results[0].Kind, results[0].Title)
	}
	if results[1].Kind != SearchResultNode || results[1].NodeID != "n1" {
		t.Errorf("expected second result to be node n1 (title match), got kind=%d nodeID=%q", results[1].Kind, results[1].NodeID)
	}
	if results[2].Kind != SearchResultNode || results[2].NodeID != "n2" {
		t.Errorf("expected third result to be node n2 (body match), got kind=%d nodeID=%q", results[2].Kind, results[2].NodeID)
	}
}

func TestSearchAllSkipsArchived(t *testing.T) {
	archived := makeNode("n1", "Archived task", "", []string{"task"})
	archived.Properties["status"] = "archived"

	active := makeNode("n2", "Active task", "", []string{"task"})

	idx := &stubIndex{nodes: []*types.Node{archived, active}}
	results := searchAll("task", commands(), idx)

	for _, r := range results {
		if r.Kind == SearchResultNode && r.NodeID == "n1" {
			t.Error("expected archived node to be excluded from results")
		}
	}
	found := false
	for _, r := range results {
		if r.Kind == SearchResultNode && r.NodeID == "n2" {
			found = true
		}
	}
	if !found {
		t.Error("expected active node to appear in results")
	}
}

func TestSearchAllNilIndex(t *testing.T) {
	// Should not panic and should return only command results.
	results := searchAll("quit", commands(), nil)
	for _, r := range results {
		if r.Kind != SearchResultCommand {
			t.Errorf("expected only commands with nil index, got kind=%d", r.Kind)
		}
	}
}

func TestSearchAllMaxResults(t *testing.T) {
	// Build 60 nodes all matching "alpha".
	nodes := make([]*types.Node, 60)
	for i := range nodes {
		nodes[i] = makeNode(
			fmt.Sprintf("n%d", i),
			fmt.Sprintf("alpha node %d", i),
			"",
			[]string{"task"},
		)
	}
	idx := &stubIndex{nodes: nodes}
	results := searchAll("alpha", commands(), idx)
	if len(results) > 50 {
		t.Errorf("expected at most 50 results, got %d", len(results))
	}
}

// ── confirm dispatch tests ───────────────────────────────────────────────────

func TestPaletteConfirmCommand(t *testing.T) {
	executed := false
	idx := &stubIndex{}
	ps := NewPaletteState(nil, idx)
	// Inject a testable command directly into results.
	ps.results = []SearchResult{
		{
			Kind:  SearchResultCommand,
			Title: "test",
			Command: &Command{
				Name: "test",
				Execute: func(_ []string) tea.Cmd {
					executed = true
					return nil
				},
			},
		},
	}
	ps.mode = PaletteModeFuzzy
	ps.cursor = 0

	cmd := ps.confirm()
	if cmd != nil {
		cmd() // execute the returned tea.Cmd to trigger Execute
	}
	if !executed {
		t.Error("expected command Execute to be called")
	}
}

func TestPaletteConfirmNode(t *testing.T) {
	ps := NewPaletteState(nil, nil)
	ps.results = []SearchResult{
		{Kind: SearchResultNode, NodeID: "abc-123"},
	}
	ps.mode = PaletteModeFuzzy
	ps.cursor = 0

	cmd := ps.confirm()
	if cmd == nil {
		t.Fatal("expected a tea.Cmd, got nil")
	}
	msg := cmd()
	sel, ok := msg.(nodeSelectedMsg)
	if !ok {
		t.Fatalf("expected nodeSelectedMsg, got %T", msg)
	}
	if sel.nodeID != "abc-123" {
		t.Errorf("expected nodeID 'abc-123', got %q", sel.nodeID)
	}
}

func TestPaletteConfirmEdge(t *testing.T) {
	// Edge confirm should navigate to the From node.
	ps := NewPaletteState(nil, nil)
	ps.results = []SearchResult{
		{Kind: SearchResultEdge, NodeID: "from-node-id", EdgeID: "e1"},
	}
	ps.mode = PaletteModeFuzzy
	ps.cursor = 0

	cmd := ps.confirm()
	if cmd == nil {
		t.Fatal("expected a tea.Cmd, got nil")
	}
	msg := cmd()
	sel, ok := msg.(nodeSelectedMsg)
	if !ok {
		t.Fatalf("expected nodeSelectedMsg, got %T", msg)
	}
	if sel.nodeID != "from-node-id" {
		t.Errorf("expected nodeID 'from-node-id', got %q", sel.nodeID)
	}
}
