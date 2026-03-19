package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// --- Helpers ---

var testNow = time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)

func newRenderer() *DetailRenderer {
	r := NewDetailRenderer()
	// Disable ANSI styling in tests so we can check plain text.
	// lipgloss respects NO_COLOR; we strip styles another way by checking substrings.
	return r
}

// stripANSI removes ANSI escape codes from a string so tests can check plain text.
func stripANSI(s string) string {
	var result strings.Builder
	inEscape := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if s[i] == 'm' {
				inEscape = false
			}
			continue
		}
		result.WriteByte(s[i])
	}
	return result.String()
}

func simpleNode(id, body string, types_ []string) *types.Node {
	return &types.Node{
		ID:       id,
		Body:     body,
		Types:    types_,
		Created:  testNow.Add(-7 * 24 * time.Hour),
		Modified: testNow,
	}
}

// --- Node detail pane tests ---

func TestRender_TitleAppearsFirst(t *testing.T) {
	node := simpleNode("n1", "My test node\n\nSome body content.", []string{"task"})
	r := newRenderer()
	output := stripANSI(r.Render(node, nil, nil, nil, testNow))

	if !strings.Contains(output, "My test node") {
		t.Errorf("expected title 'My test node' in output, got:\n%s", output)
	}
}

func TestRender_BodyIncluded(t *testing.T) {
	node := simpleNode("n2", "Title\n\nDetailed body text here.", []string{"note"})
	r := newRenderer()
	output := stripANSI(r.Render(node, nil, nil, nil, testNow))

	if !strings.Contains(output, "Detailed body text here.") {
		t.Errorf("expected body content in output, got:\n%s", output)
	}
}

func TestRender_MetadataKeyValue(t *testing.T) {
	node := simpleNode("n3", "Node with meta", []string{"task"})
	node.Properties = map[string]interface{}{
		"status":   "active",
		"priority": "high",
	}
	r := newRenderer()
	output := stripANSI(r.Render(node, nil, nil, nil, testNow))

	if !strings.Contains(output, "status:") {
		t.Errorf("expected 'status:' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "active") {
		t.Errorf("expected 'active' in output, got:\n%s", output)
	}
}

func TestRender_MetadataSkipsNil(t *testing.T) {
	node := simpleNode("n4", "Node", []string{"task"})
	node.Properties = map[string]interface{}{
		"present":  "value",
		"absent":   nil,
	}
	r := newRenderer()
	output := stripANSI(r.Render(node, nil, nil, nil, testNow))

	if strings.Contains(output, "absent:") {
		t.Errorf("expected nil property to be skipped, got:\n%s", output)
	}
}

// --- Edge rendering tests ---

func TestRender_EdgesSection_Blocks_Outgoing(t *testing.T) {
	focalNode := simpleNode("focal", "My task", []string{"task"})
	targetNode := simpleNode("target", "Blocked node", []string{"task"})

	edge := &types.Edge{
		ID:      "e1",
		Type:    "blocks",
		From:    "focal",
		To:      "target",
		Created: testNow,
	}

	nodesByID := map[string]*types.Node{"focal": focalNode, "target": targetNode}
	r := newRenderer()
	output := stripANSI(r.Render(focalNode, []*types.Edge{edge}, nodesByID, nil, testNow))

	if !strings.Contains(output, "EDGES") {
		t.Errorf("expected EDGES section header, got:\n%s", output)
	}
	if !strings.Contains(output, "→") {
		t.Errorf("expected → glyph for outgoing blocks edge, got:\n%s", output)
	}
	if !strings.Contains(output, "Blocked node") {
		t.Errorf("expected target node label in output, got:\n%s", output)
	}
}

func TestRender_EdgesSection_Blocks_Incoming(t *testing.T) {
	focalNode := simpleNode("focal", "My task", []string{"task"})
	sourceNode := simpleNode("source", "Blocking node", []string{"task"})

	edge := &types.Edge{
		ID:      "e2",
		Type:    "blocks",
		From:    "source",
		To:      "focal",
		Created: testNow,
	}

	nodesByID := map[string]*types.Node{"focal": focalNode, "source": sourceNode}
	r := newRenderer()
	output := stripANSI(r.Render(focalNode, []*types.Edge{edge}, nodesByID, nil, testNow))

	if !strings.Contains(output, "←") {
		t.Errorf("expected ← glyph for incoming blocks edge, got:\n%s", output)
	}
}

func TestRender_EdgesSection_Parent(t *testing.T) {
	focalNode := simpleNode("focal", "Child node", []string{"task"})
	parentNode := simpleNode("parent", "Parent node", []string{"task"})

	edge := &types.Edge{
		ID:      "e3",
		Type:    "parent",
		From:    "focal",
		To:      "parent",
		Created: testNow,
	}

	nodesByID := map[string]*types.Node{"focal": focalNode, "parent": parentNode}
	r := newRenderer()
	output := stripANSI(r.Render(focalNode, []*types.Edge{edge}, nodesByID, nil, testNow))

	if !strings.Contains(output, "→") {
		t.Errorf("expected → glyph for parent edge, got:\n%s", output)
	}
	if !strings.Contains(output, "Parent node") {
		t.Errorf("expected parent node label in output, got:\n%s", output)
	}
}

func TestRender_EdgesSection_WaitingOn(t *testing.T) {
	focalNode := simpleNode("focal", "My project", []string{"task"})
	targetNode := simpleNode("target", "Dan (feedback)", []string{"person"})

	edge := &types.Edge{
		ID:      "e4",
		Type:    "waiting_on",
		From:    "focal",
		To:      "target",
		Created: testNow.Add(-12 * 24 * time.Hour),
	}

	nodesByID := map[string]*types.Node{"focal": focalNode, "target": targetNode}
	r := newRenderer()
	output := stripANSI(r.Render(focalNode, []*types.Edge{edge}, nodesByID, nil, testNow))

	if !strings.Contains(output, "⊘") {
		t.Errorf("expected ⊘ glyph for waiting_on edge, got:\n%s", output)
	}
	if !strings.Contains(output, "12d") {
		t.Errorf("expected age suffix '12d' for waiting_on edge, got:\n%s", output)
	}
}

func TestRender_EdgesSection_Related(t *testing.T) {
	focalNode := simpleNode("focal", "My note", []string{"note"})
	relatedNode := simpleNode("related", "Cypher syntax notes", []string{"note"})

	edge := &types.Edge{
		ID:      "e5",
		Type:    "related",
		From:    "focal",
		To:      "related",
		Created: testNow,
	}

	nodesByID := map[string]*types.Node{"focal": focalNode, "related": relatedNode}
	r := newRenderer()
	output := stripANSI(r.Render(focalNode, []*types.Edge{edge}, nodesByID, nil, testNow))

	if !strings.Contains(output, "◇") {
		t.Errorf("expected ◇ glyph for related edge, got:\n%s", output)
	}
	if !strings.Contains(output, "Cypher syntax notes") {
		t.Errorf("expected related node label, got:\n%s", output)
	}
}

func TestRender_EdgesSection_DependsOn(t *testing.T) {
	focalNode := simpleNode("focal", "Feature", []string{"task"})
	depNode := simpleNode("dep", "Auth service", []string{"task"})

	edge := &types.Edge{
		ID:      "e6",
		Type:    "depends_on",
		From:    "focal",
		To:      "dep",
		Created: testNow,
	}

	nodesByID := map[string]*types.Node{"focal": focalNode, "dep": depNode}
	r := newRenderer()
	output := stripANSI(r.Render(focalNode, []*types.Edge{edge}, nodesByID, nil, testNow))

	if !strings.Contains(output, "→") {
		t.Errorf("expected → glyph for depends_on edge, got:\n%s", output)
	}
	if !strings.Contains(output, "Auth service") {
		t.Errorf("expected dep node label in output, got:\n%s", output)
	}
}

// --- Edge age colour tests ---

func TestEdgeAgeColour_Muted_0to7Days(t *testing.T) {
	c := defaultColours()
	for _, days := range []int{0, 3, 7} {
		colour := ageColourForDays(days, c)
		if colour != c.FGMuted {
			t.Errorf("expected muted colour for %d days, got %s", days, colour)
		}
	}
}

func TestEdgeAgeColour_Warn_8to14Days(t *testing.T) {
	c := defaultColours()
	for _, days := range []int{8, 10, 14} {
		colour := ageColourForDays(days, c)
		if colour != c.OverflowWarn {
			t.Errorf("expected warn colour for %d days, got %s", days, colour)
		}
	}
}

func TestEdgeAgeColour_Critical_15PlusDays(t *testing.T) {
	c := defaultColours()
	for _, days := range []int{15, 20, 100} {
		colour := ageColourForDays(days, c)
		if colour != c.OverflowCrit {
			t.Errorf("expected critical colour for %d days, got %s", days, colour)
		}
	}
}

// --- Synced node display tests ---

func TestRender_SyncedNode_ShowsSourceInfo(t *testing.T) {
	node := simpleNode("n-sync", "GitHub issue", []string{"task"})
	node.Source = &types.Source{
		Type: "github",
		Repo: "jasonwarrenuk/wyrd",
		URL:  "https://github.com/jasonwarrenuk/wyrd/issues/42",
	}

	r := newRenderer()
	output := stripANSI(r.Render(node, nil, nil, nil, testNow))

	if !strings.Contains(output, "github") {
		t.Errorf("expected source type 'github' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "jasonwarrenuk/wyrd") {
		t.Errorf("expected source repo in output, got:\n%s", output)
	}
	if !strings.Contains(output, "https://github.com") {
		t.Errorf("expected source URL in output, got:\n%s", output)
	}
}

// --- Archived node tests ---

func TestRender_ArchivedNode_ShowsBanner(t *testing.T) {
	node := simpleNode("n-arch", "Old project", []string{"project"})
	node.Properties = map[string]interface{}{
		"status": "archived",
	}

	r := newRenderer()
	output := stripANSI(r.Render(node, nil, nil, nil, testNow))

	if !strings.Contains(output, "ARCHIVED") {
		t.Errorf("expected ARCHIVED banner in output, got:\n%s", output)
	}
}

func TestRender_ArchivedNode_BodyStillRendered(t *testing.T) {
	node := simpleNode("n-arch2", "Archived project body", []string{"project"})
	node.Properties = map[string]interface{}{
		"status": "archived",
	}

	r := newRenderer()
	output := stripANSI(r.Render(node, nil, nil, nil, testNow))

	if !strings.Contains(output, "Archived project body") {
		t.Errorf("expected body content even for archived node, got:\n%s", output)
	}
}

func TestRender_NonArchivedNode_NoBanner(t *testing.T) {
	node := simpleNode("n5", "Active node", []string{"task"})
	node.Properties = map[string]interface{}{
		"status": "active",
	}

	r := newRenderer()
	output := stripANSI(r.Render(node, nil, nil, nil, testNow))

	if strings.Contains(output, "ARCHIVED") {
		t.Errorf("expected no ARCHIVED banner for non-archived node, got:\n%s", output)
	}
}

// --- Budget section tests ---

func TestRender_BudgetSection_Shown(t *testing.T) {
	node := simpleNode("n-main", "Monthly review", []string{"task"})
	budgetNode := &types.Node{
		ID:    "budget-1",
		Body:  "Groceries budget",
		Types: []string{"budget"},
		Properties: map[string]interface{}{
			"category":  "groceries",
			"allocated": float64(200),
			"warn_at":   0.8,
			"period":    "month",
			"spend_log": []types.SpendEntry{
				{Date: "2026-03-10", Amount: 50},
			},
		},
	}

	r := newRenderer()
	output := stripANSI(r.Render(node, nil, nil, []*types.Node{budgetNode}, testNow))

	if !strings.Contains(output, "BUDGETS") {
		t.Errorf("expected BUDGETS section header, got:\n%s", output)
	}
	if !strings.Contains(output, "groceries") {
		t.Errorf("expected budget category 'groceries' in output, got:\n%s", output)
	}
}

func TestRender_NoBudgetNodes_NoSection(t *testing.T) {
	node := simpleNode("n6", "Simple node", []string{"task"})
	r := newRenderer()
	output := stripANSI(r.Render(node, nil, nil, nil, testNow))

	if strings.Contains(output, "BUDGETS") {
		t.Errorf("expected no BUDGETS section when no budget nodes provided, got:\n%s", output)
	}
}

// --- Progress bar tests ---

func TestBuildProgressBar_Empty(t *testing.T) {
	bar := buildProgressBar(0, 100, 10)
	if !strings.Contains(bar, "░") {
		t.Errorf("expected empty bar to contain ░, got %q", bar)
	}
	if strings.Contains(bar, "█") {
		t.Errorf("expected empty bar to have no filled blocks, got %q", bar)
	}
}

func TestBuildProgressBar_Half(t *testing.T) {
	bar := buildProgressBar(50, 100, 10)
	// Expect 5 filled, 5 empty within brackets.
	inner := strings.TrimPrefix(strings.TrimSuffix(bar, "]"), "[")
	filled := strings.Count(inner, "█")
	empty := strings.Count(inner, "░")
	if filled != 5 || empty != 5 {
		t.Errorf("expected 5 filled and 5 empty, got filled=%d empty=%d bar=%q", filled, empty, bar)
	}
}

func TestBuildProgressBar_Over(t *testing.T) {
	bar := buildProgressBar(150, 100, 10)
	// Over budget — all 10 should be filled.
	inner := strings.TrimPrefix(strings.TrimSuffix(bar, "]"), "[")
	filled := strings.Count(inner, "█")
	if filled != 10 {
		t.Errorf("expected fully filled bar for over-budget, got %q", bar)
	}
}
