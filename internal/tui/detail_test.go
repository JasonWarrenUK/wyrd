package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
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
		"present": "value",
		"absent":  nil,
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

// --- Blocked node tests (DL.2) ---

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

func TestRender_BlockedNode_ShowsBannerAndBlockedBy(t *testing.T) {
	blocker := simpleNode("blocker-1", "Blocking task", []string{"task"})
	blocker.Title = "Blocking task"
	blocker.Kind = "task"
	blocker.Stage = "Now" // non-terminal

	node := simpleNode("n-blocked", "Blocked task body", []string{"task"})
	edges := []*types.Edge{
		{ID: "e1", Type: "blocks", From: blocker.ID, To: node.ID},
	}
	nodesByID := map[string]*types.Node{blocker.ID: blocker}

	kinds, groups := blockedTestRegistries()
	r := newRenderer()
	r.Kinds = kinds
	r.StageGroups = groups

	output := stripANSI(r.Render(node, edges, nodesByID, nil, testNow))

	if !strings.Contains(output, "BLOCKED") {
		t.Errorf("expected BLOCKED banner in output, got:\n%s", output)
	}
	if !strings.Contains(output, "BLOCKED BY") {
		t.Errorf("expected BLOCKED BY section in output, got:\n%s", output)
	}
	if !strings.Contains(output, "Blocking task") {
		t.Errorf("expected blocker title in BLOCKED BY section, got:\n%s", output)
	}
}

func TestRender_TerminalBlocker_NoBanner(t *testing.T) {
	blocker := simpleNode("blocker-2", "Done task", []string{"task"})
	blocker.Title = "Done task"
	blocker.Kind = "task"
	blocker.Stage = "Done" // terminal — lifts the block

	node := simpleNode("n-unblocked", "Free task body", []string{"task"})
	edges := []*types.Edge{
		{ID: "e2", Type: "blocks", From: blocker.ID, To: node.ID},
	}
	nodesByID := map[string]*types.Node{blocker.ID: blocker}

	kinds, groups := blockedTestRegistries()
	r := newRenderer()
	r.Kinds = kinds
	r.StageGroups = groups

	output := stripANSI(r.Render(node, edges, nodesByID, nil, testNow))

	if strings.Contains(output, "BLOCKED") {
		t.Errorf("expected no BLOCKED banner when blocker is terminal, got:\n%s", output)
	}
	if strings.Contains(output, "BLOCKED BY") {
		t.Errorf("expected no BLOCKED BY section when blocker is terminal, got:\n%s", output)
	}
}

func TestRender_NoBlocksEdges_NoBanner(t *testing.T) {
	node := simpleNode("n-noedges", "Standalone task", []string{"task"})
	r := newRenderer()
	output := stripANSI(r.Render(node, nil, nil, nil, testNow))

	if strings.Contains(output, "BLOCKED") {
		t.Errorf("expected no BLOCKED banner with no blocks edges, got:\n%s", output)
	}
}

func TestRender_UnresolvableBlocker_StillShowsBanner(t *testing.T) {
	// Blocker has no Kind/Stage set — terminality can't be resolved, so
	// "presence blocks" applies (mirrors types.EvalBlockers/Blockers).
	blocker := simpleNode("blocker-3", "Untriaged blocker", nil)
	blocker.Title = "Untriaged blocker"

	node := simpleNode("n-unresolved", "Task blocked by untriaged node", []string{"task"})
	edges := []*types.Edge{
		{ID: "e3", Type: "blocks", From: blocker.ID, To: node.ID},
	}
	nodesByID := map[string]*types.Node{blocker.ID: blocker}

	r := newRenderer() // no Kinds/StageGroups wired — registries absent too
	output := stripANSI(r.Render(node, edges, nodesByID, nil, testNow))

	if !strings.Contains(output, "BLOCKED") {
		t.Errorf("expected BLOCKED banner for unresolvable blocker (presence blocks), got:\n%s", output)
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

// --- NV.10: markdown rendering tests ---

func TestRender_MarkdownHeading(t *testing.T) {
	node := simpleNode("n-md1", "## Heading\n\nText paragraph here.", []string{"note"})
	r := newRenderer()
	output := stripANSI(r.Render(node, nil, nil, nil, testNow))

	if !strings.Contains(output, "Heading") {
		t.Errorf("expected heading text in output, got:\n%s", output)
	}
	if !strings.Contains(output, "Text paragraph here.") {
		t.Errorf("expected body text in output, got:\n%s", output)
	}
}

func TestRender_MarkdownCodeBlock(t *testing.T) {
	node := simpleNode("n-md2", "Title\n\n```\nconst x = 42\n```", []string{"note"})
	r := newRenderer()
	output := stripANSI(r.Render(node, nil, nil, nil, testNow))

	if !strings.Contains(output, "x = 42") {
		t.Errorf("expected code content in output, got:\n%s", output)
	}
}

func TestRender_ArchivedNodePlainText(t *testing.T) {
	node := simpleNode("n-md3", "## Heading\n\nSome text.", []string{"note"})
	node.Properties = map[string]interface{}{"status": "archived"}
	r := newRenderer()
	output := stripANSI(r.Render(node, nil, nil, nil, testNow))

	// Archived nodes bypass Glamour — the raw body (after title strip) is rendered
	// in muted style. The heading markers may appear as-is or the text may appear.
	if !strings.Contains(output, "Some text.") {
		t.Errorf("expected body text in archived output, got:\n%s", output)
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

// --- SPEND LOG section tests ---

func newBudgetNodeWithSpend(id string, entries []types.SpendEntry) *types.Node {
	props := map[string]interface{}{
		"category":  "groceries",
		"allocated": float64(200),
		"warn_at":   0.8,
		"period":    "month",
	}
	if len(entries) > 0 {
		props["spend_log"] = entries
	}
	return &types.Node{
		ID:         id,
		Body:       "Groceries budget",
		Types:      []string{"budget"},
		Properties: props,
		Created:    testNow,
		Modified:   testNow,
	}
}

func TestRender_SpendLog_ShowsSection(t *testing.T) {
	entries := []types.SpendEntry{
		{Date: "2026-03-10", Amount: 42.50, Note: "weekly shop"},
		{Date: "2026-03-12", Amount: 15.00, Note: "top-up"},
	}
	node := newBudgetNodeWithSpend("b1", entries)
	r := newRenderer()
	output := stripANSI(r.Render(node, nil, nil, nil, testNow))

	if !strings.Contains(output, "SPEND LOG") {
		t.Errorf("expected 'SPEND LOG' section header, got:\n%s", output)
	}
	if !strings.Contains(output, "2026-03-10") {
		t.Errorf("expected first entry date in output, got:\n%s", output)
	}
	if !strings.Contains(output, "weekly shop") {
		t.Errorf("expected first entry note in output, got:\n%s", output)
	}
	if !strings.Contains(output, "42.50") {
		t.Errorf("expected first entry amount in output, got:\n%s", output)
	}
}

func TestRender_SpendLog_RunningTotal(t *testing.T) {
	entries := []types.SpendEntry{
		{Date: "2026-03-10", Amount: 42.50, Note: "weekly shop"},
		{Date: "2026-03-12", Amount: 15.00, Note: "top-up"},
	}
	node := newBudgetNodeWithSpend("b2", entries)
	r := newRenderer()
	output := stripANSI(r.Render(node, nil, nil, nil, testNow))

	// Running total after both entries should be 57.50.
	if !strings.Contains(output, "57.50") {
		t.Errorf("expected running total 57.50 in output, got:\n%s", output)
	}
}

func TestRender_SpendLog_SortedByDate(t *testing.T) {
	// Entries stored out of date order — should appear sorted ascending.
	entries := []types.SpendEntry{
		{Date: "2026-03-15", Amount: 20.00, Note: "last"},
		{Date: "2026-01-05", Amount: 10.00, Note: "first"},
		{Date: "2026-02-20", Amount: 5.00, Note: "middle"},
	}
	node := newBudgetNodeWithSpend("b3", entries)
	r := newRenderer()
	output := stripANSI(r.Render(node, nil, nil, nil, testNow))

	firstIdx := strings.Index(output, "2026-01-05")
	middleIdx := strings.Index(output, "2026-02-20")
	lastIdx := strings.Index(output, "2026-03-15")

	if firstIdx < 0 || middleIdx < 0 || lastIdx < 0 {
		t.Fatalf("one or more date entries missing from output:\n%s", output)
	}
	if !(firstIdx < middleIdx && middleIdx < lastIdx) {
		t.Errorf("entries not in ascending date order: first=%d middle=%d last=%d\noutput:\n%s",
			firstIdx, middleIdx, lastIdx, output)
	}
}

func TestRender_SpendLog_EmptyLog_NoSection(t *testing.T) {
	node := newBudgetNodeWithSpend("b4", nil) // no entries
	r := newRenderer()
	output := stripANSI(r.Render(node, nil, nil, nil, testNow))

	if strings.Contains(output, "SPEND LOG") {
		t.Errorf("expected no 'SPEND LOG' section for empty spend_log, got:\n%s", output)
	}
}

func TestRender_SpendLog_NonBudgetNode_NoSection(t *testing.T) {
	node := simpleNode("n-task", "A regular task", []string{"task"})
	r := newRenderer()
	output := stripANSI(r.Render(node, nil, nil, nil, testNow))

	if strings.Contains(output, "SPEND LOG") {
		t.Errorf("expected no 'SPEND LOG' section for non-budget node, got:\n%s", output)
	}
}

func TestRender_SpendLog_NoteOptional(t *testing.T) {
	entries := []types.SpendEntry{
		{Date: "2026-06-01", Amount: 9.99, Note: ""},
	}
	node := newBudgetNodeWithSpend("b5", entries)
	r := newRenderer()
	// Should not panic and should render the amount.
	output := stripANSI(r.Render(node, nil, nil, nil, testNow))

	if !strings.Contains(output, "9.99") {
		t.Errorf("expected amount 9.99 in output, got:\n%s", output)
	}
}

// --- Kind/Stage line tests ---

// newRendererWithKinds builds a renderer with a minimal kind registry for tests.
func newRendererWithKinds() *DetailRenderer {
	r := newRenderer()
	r.Kinds = types.NewKindRegistry([]types.Kind{
		{Name: "Task", StageGroup: "task-flow", Glyph: "◆", Colour: "#9b70ff"},
		{Name: "Goblin", StageGroup: "task-flow", Glyph: "◈", Colour: "#f97316"},
	})
	return r
}

func TestRender_KindStage_Resolved(t *testing.T) {
	node := simpleNode("ks1", "Some task", []string{"task"})
	node.Kind = "Task"
	node.Stage = "doing"
	r := newRendererWithKinds()
	output := stripANSI(r.Render(node, nil, nil, nil, testNow))

	if !strings.Contains(output, "Task") {
		t.Errorf("expected kind name 'Task' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "◆") {
		t.Errorf("expected glyph '◆' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "doing") {
		t.Errorf("expected stage 'doing' in output, got:\n%s", output)
	}
}

func TestRender_KindStage_StageOnly_NoKind(t *testing.T) {
	node := simpleNode("ks2", "Node without kind", []string{"task"})
	node.Kind = ""
	node.Stage = "doing"
	r := newRendererWithKinds()
	output := stripANSI(r.Render(node, nil, nil, nil, testNow))

	// Stage should appear; no glyph from a kind.
	if !strings.Contains(output, "doing") {
		t.Errorf("expected stage 'doing' in output, got:\n%s", output)
	}
	if strings.Contains(output, "◆") || strings.Contains(output, "◈") {
		t.Errorf("expected no kind glyph when Kind is empty, got:\n%s", output)
	}
}

func TestRender_KindStage_Unresolved(t *testing.T) {
	node := simpleNode("ks3", "Old node", []string{"task"})
	node.Kind = "OldKind"
	node.Stage = "doing"
	r := newRendererWithKinds()
	output := stripANSI(r.Render(node, nil, nil, nil, testNow))

	// Unresolved kind falls back to stage-only; no panic.
	if !strings.Contains(output, "doing") {
		t.Errorf("expected stage 'doing' in output for unresolved kind, got:\n%s", output)
	}
	if strings.Contains(output, "◆") || strings.Contains(output, "◈") {
		t.Errorf("expected no kind glyph for unresolved kind 'OldKind', got:\n%s", output)
	}
}

func TestRender_KindStage_BothEmpty(t *testing.T) {
	node := simpleNode("ks4", "Plain node", []string{"note"})
	node.Kind = ""
	node.Stage = ""
	r := newRendererWithKinds()
	output := stripANSI(r.Render(node, nil, nil, nil, testNow))

	// Neither kind nor stage — the line should be absent. Check no stray glyphs.
	if strings.Contains(output, "◆") || strings.Contains(output, "◈") {
		t.Errorf("expected no kind/stage line for empty kind+stage, got:\n%s", output)
	}
}

func TestRender_KindStage_NilRegistry(t *testing.T) {
	node := simpleNode("ks5", "Legacy node", []string{"task"})
	node.Kind = "Task"
	node.Stage = "todo"
	// newRenderer() leaves Kinds nil — must not panic; falls back to stage-only.
	r := newRenderer()
	output := stripANSI(r.Render(node, nil, nil, nil, testNow))

	// Stage should still render; no panic.
	if !strings.Contains(output, "todo") {
		t.Errorf("expected stage 'todo' in output with nil registry, got:\n%s", output)
	}
}

// --- Body background tests ---

// TestRender_BodyMarkdown_NoDefaultBgEscape verifies that the glamour-rendered
// body does not contain a raw default-background SGR escape (\x1b[49m) when
// BgPrimary is set. Without FillBackground, glamour emits \x1b[49m at ANSI
// reset boundaries, causing the terminal default to bleed through the body
// while every other detail block carries the theme background.
func TestRender_BodyMarkdown_NoDefaultBgEscape(t *testing.T) {
	node := simpleNode("bg1", "Header line\n\nPlain paragraph body.", []string{"note"})
	r := newRenderer()
	// Set a non-nil BgPrimary so FillBackground can repaint the body.
	r.Colours.BgPrimary = lipgloss.Color("#000000")

	output := r.Render(node, nil, nil, nil, testNow)

	// The raw default-background SGR (\x1b[49m) must not appear: FillBackground
	// replaces it with the theme background on every line.
	if strings.Contains(output, "\x1b[49m") {
		t.Error("body markdown output contains \\x1b[49m (default-bg escape); expected FillBackground to have repainted it")
	}
}
