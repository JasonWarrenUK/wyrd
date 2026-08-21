package views

import (
	"strings"
	"testing"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// stripANSI removes ANSI escape codes from a string so tests can check plain
// text — needed once ProseRenderer started emitting Glamour-rendered
// markdown (TD.20), which word-wraps and splits plain-text runs across
// separate styled segments. Mirrors internal/tui/detail_test.go's helper of
// the same name and purpose.
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

func TestProseRenderer_NilNode(t *testing.T) {
	r := NewProseRenderer()
	output := stripANSI(r.Render(nil, nil, nil, nil, time.Now(), 80))
	if !strings.Contains(output, "No node selected") {
		t.Errorf("expected empty-state message, got: %q", output)
	}
}

func TestProseRenderer_RendersBody(t *testing.T) {
	r := NewProseRenderer()
	node := &types.Node{
		ID:    "abc-123",
		Body:  "# My Node\n\nThis is the body content.",
		Types: []string{"note"},
		Date: types.DateFields{
			Created:  time.Now(),
			Modified: time.Now(),
		},
	}

	output := stripANSI(r.Render(node, nil, nil, nil, time.Now(), 80))
	if !strings.Contains(output, "My Node") {
		t.Error("expected node body to appear in output")
	}
	if !strings.Contains(output, "body content") {
		t.Error("expected node body content in output")
	}
}

func TestProseRenderer_RendersMetadata(t *testing.T) {
	r := NewProseRenderer()
	node := &types.Node{
		ID:    "abc-123",
		Body:  "Test",
		Types: []string{"task", "project"},
		Date: types.DateFields{
			Created:  time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC),
			Modified: time.Date(2024, 3, 16, 12, 0, 0, 0, time.UTC),
		},
	}

	output := stripANSI(r.Render(node, nil, nil, nil, time.Now(), 80))

	if !strings.Contains(output, "abc-123") {
		t.Error("expected node ID in metadata")
	}
	if !strings.Contains(output, "task") {
		t.Error("expected node types in metadata")
	}
	if !strings.Contains(output, "2024-03-15") {
		t.Error("expected created date in metadata")
	}
}

func TestProseRenderer_RendersEdges(t *testing.T) {
	r := NewProseRenderer()
	node := &types.Node{
		ID:    "node-1",
		Body:  "Source node",
		Types: []string{"task"},
		Date: types.DateFields{
			Created:  time.Now(),
			Modified: time.Now(),
		},
	}
	edges := []*types.Edge{
		{
			ID:   "edge-1",
			Type: "blocks",
			From: "node-1",
			To:   "node-2",
		},
		{
			ID:   "edge-2",
			Type: "related",
			From: "node-3",
			To:   "node-1",
		},
	}

	output := stripANSI(r.Render(node, edges, nil, nil, time.Now(), 80))

	if !strings.Contains(output, "blocks") {
		t.Error("expected 'blocks' edge type in output")
	}
	// No nodesByID supplied, so peers fall back to raw IDs.
	if !strings.Contains(output, "node-2") {
		t.Error("expected peer node ID in edge output when unresolved")
	}
	if !strings.Contains(output, "related") {
		t.Error("expected 'related' edge type in output")
	}
}

func TestProseRenderer_ResolvesEdgePeerTitle(t *testing.T) {
	// TD.20: edge peers resolve to their title via nodesByID, matching
	// DetailRenderer.renderEdgeLine, instead of always showing a raw UUID.
	r := NewProseRenderer()
	node := &types.Node{
		ID:    "node-1",
		Body:  "Source node",
		Types: []string{"task"},
		Date:  types.DateFields{Created: time.Now(), Modified: time.Now()},
	}
	peer := &types.Node{ID: "node-2", Title: "Peer Task"}
	edges := []*types.Edge{
		{ID: "edge-1", Type: "blocks", From: "node-1", To: "node-2"},
	}

	output := stripANSI(r.Render(node, edges, map[string]*types.Node{"node-2": peer}, nil, time.Now(), 80))

	if !strings.Contains(output, "Peer Task") {
		t.Errorf("expected resolved peer title 'Peer Task' in output, got: %q", output)
	}
	if strings.Contains(output, "node-2") {
		t.Errorf("expected raw peer ID NOT to appear once resolved, got: %q", output)
	}
}

func TestProseRenderer_WaitingOnEdgeShowsAgeSuffix(t *testing.T) {
	// TD.20: a waiting_on edge gets a " · Nd" ageing suffix, matching
	// DetailRenderer.renderEdgeLine.
	r := NewProseRenderer()
	now := time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC)
	node := &types.Node{ID: "node-1", Body: "Source", Date: types.DateFields{Created: now, Modified: now}}
	edges := []*types.Edge{
		{
			ID: "edge-1", Type: string(types.EdgeWaitingOn), From: "node-1", To: "node-2",
			Date: types.EdgeDates{Created: now.Add(-10 * 24 * time.Hour)},
		},
	}

	output := stripANSI(r.Render(node, edges, nil, nil, now, 80))
	if !strings.Contains(output, "10d") {
		t.Errorf("expected a 10d age suffix in output, got: %q", output)
	}
}

func TestProseRenderer_ParentAndRelatedEdgesUseDedicatedGlyphs(t *testing.T) {
	// TD.20 review fix: buildEdgeLines must read r.Glyphs.EdgeParent/
	// EdgeRelated for those edge types, not fall through to the plain
	// outgoing/incoming glyph pair (this was accidentally dropped, then
	// restored, during TD.20's parity work).
	r := NewProseRenderer()
	node := &types.Node{ID: "node-1", Body: "Source", Date: types.DateFields{Created: time.Now(), Modified: time.Now()}}
	edges := []*types.Edge{
		{ID: "e1", Type: "parent", From: "node-1", To: "node-2"},
		{ID: "e2", Type: "related", From: "node-1", To: "node-3"},
	}

	output := stripANSI(r.Render(node, edges, nil, nil, time.Now(), 80))
	if !strings.Contains(output, r.Glyphs.EdgeParent) {
		t.Errorf("expected dedicated parent glyph %q, got: %q", r.Glyphs.EdgeParent, output)
	}
	if !strings.Contains(output, r.Glyphs.EdgeRelated) {
		t.Errorf("expected dedicated related glyph %q, got: %q", r.Glyphs.EdgeRelated, output)
	}
}

func TestProseRenderer_CustomGlyphsAreHonoured(t *testing.T) {
	// TD.20 review fix: a caller-customised Glyphs value must actually
	// change the rendered output, proving ProseGlyphs isn't dead.
	r := NewProseRenderer()
	r.Glyphs.EdgeFrom = "▶"
	node := &types.Node{ID: "node-1", Body: "Source", Date: types.DateFields{Created: time.Now(), Modified: time.Now()}}
	edges := []*types.Edge{
		{ID: "e1", Type: "blocks", From: "node-1", To: "node-2"},
	}

	output := stripANSI(r.Render(node, edges, nil, nil, time.Now(), 80))
	if !strings.Contains(output, "▶") {
		t.Errorf("expected the customised EdgeFrom glyph ▶ in output, got: %q", output)
	}
	if strings.Contains(output, DefaultProseGlyphs().EdgeFrom) {
		t.Errorf("expected the default glyph to be replaced, not both present, got: %q", output)
	}
}

func TestProseRenderer_OutgoingEdgeGlyph(t *testing.T) {
	r := NewProseRenderer()
	node := &types.Node{
		ID:    "node-1",
		Body:  "Source",
		Types: []string{"task"},
		Date: types.DateFields{
			Created:  time.Now(),
			Modified: time.Now(),
		},
	}
	edges := []*types.Edge{
		{
			ID:   "edge-1",
			Type: "blocks",
			From: "node-1", // outgoing from this node
			To:   "node-2",
		},
	}

	output := stripANSI(r.Render(node, edges, nil, nil, time.Now(), 80))
	if !strings.Contains(output, r.Glyphs.EdgeFrom) {
		t.Errorf("expected outgoing glyph %q for outgoing edge", r.Glyphs.EdgeFrom)
	}
}

func TestProseRenderer_IncomingEdgeGlyph(t *testing.T) {
	r := NewProseRenderer()
	node := &types.Node{
		ID:    "node-1",
		Body:  "Target",
		Types: []string{"task"},
		Date: types.DateFields{
			Created:  time.Now(),
			Modified: time.Now(),
		},
	}
	edges := []*types.Edge{
		{
			ID:   "edge-1",
			Type: "blocks",
			From: "node-2", // incoming to this node
			To:   "node-1",
		},
	}

	output := stripANSI(r.Render(node, edges, nil, nil, time.Now(), 80))
	if !strings.Contains(output, r.Glyphs.EdgeTo) {
		t.Errorf("expected incoming glyph %q for incoming edge", r.Glyphs.EdgeTo)
	}
}

func TestProseRenderer_NoEdges(t *testing.T) {
	r := NewProseRenderer()
	node := &types.Node{
		ID:    "node-1",
		Body:  "Isolated node",
		Types: []string{"note"},
		Date: types.DateFields{
			Created:  time.Now(),
			Modified: time.Now(),
		},
	}

	output := stripANSI(r.Render(node, nil, nil, nil, time.Now(), 80))
	// Should render without errors; body should still be present.
	if !strings.Contains(output, "Isolated node") {
		t.Error("expected node body in output when no edges present")
	}
}

func TestProseRenderer_NodeWithSource(t *testing.T) {
	r := NewProseRenderer()
	node := &types.Node{
		ID:    "node-1",
		Body:  "Synced node",
		Types: []string{"task"},
		Source: &types.Source{
			Type: "github",
			URL:  "https://github.com/owner/repo/issues/42",
		},
		Date: types.DateFields{
			Created:  time.Now(),
			Modified: time.Now(),
		},
	}

	output := stripANSI(r.Render(node, nil, nil, nil, time.Now(), 80))
	if !strings.Contains(output, "github") {
		t.Error("expected source type 'github' in metadata")
	}
	if !strings.Contains(output, "https://github.com") {
		t.Error("expected source URL in metadata")
	}
}

// --- TD.20a: parity additions (ARCHIVED/BLOCKED banners, staleness,
// kind/stage, BUDGETS, SPEND LOG) ---

func TestProseRenderer_ArchivedBanner(t *testing.T) {
	r := NewProseRenderer()
	node := &types.Node{
		ID:         "node-1",
		Body:       "Archived content",
		Types:      []string{"note"},
		Properties: map[string]interface{}{"status": "archived"},
		Date:       types.DateFields{Created: time.Now(), Modified: time.Now()},
	}

	output := stripANSI(r.Render(node, nil, nil, nil, time.Now(), 80))
	if !strings.Contains(output, "ARCHIVED") {
		t.Errorf("expected ARCHIVED banner, got: %q", output)
	}
}

func TestProseRenderer_NoArchivedBannerWhenNotArchived(t *testing.T) {
	r := NewProseRenderer()
	node := &types.Node{
		ID:   "node-1",
		Body: "Regular content",
		Date: types.DateFields{Created: time.Now(), Modified: time.Now()},
	}

	output := stripANSI(r.Render(node, nil, nil, nil, time.Now(), 80))
	if strings.Contains(output, "ARCHIVED") {
		t.Errorf("expected no ARCHIVED banner for a non-archived node, got: %q", output)
	}
}

func TestProseRenderer_BlockedBannerAndSection(t *testing.T) {
	r := NewProseRenderer()
	node := &types.Node{ID: "node-1", Body: "Blocked target", Date: types.DateFields{Created: time.Now(), Modified: time.Now()}}
	blocker := &types.Node{ID: "blocker-1", Title: "Unfinished Prereq", Stage: "doing"}
	edges := []*types.Edge{
		{ID: "e1", Type: string(types.EdgeBlocks), From: "blocker-1", To: "node-1"},
	}
	nodesByID := map[string]*types.Node{"blocker-1": blocker}

	output := stripANSI(r.Render(node, edges, nodesByID, nil, time.Now(), 80))
	if !strings.Contains(output, "BLOCKED") {
		t.Errorf("expected BLOCKED banner, got: %q", output)
	}
	if !strings.Contains(output, "BLOCKED BY") {
		t.Errorf("expected BLOCKED BY section, got: %q", output)
	}
	if !strings.Contains(output, "Unfinished Prereq") {
		t.Errorf("expected blocker title in BLOCKED BY section, got: %q", output)
	}
}

func TestProseRenderer_TerminalBlockerDoesNotBlock(t *testing.T) {
	// A blocker whose stage is terminal (per its StageGroup) no longer
	// blocks, mirroring DetailRenderer.blockers' own semantics.
	r := NewProseRenderer()
	r.StageGroups = types.NewStageGroupRegistry([]types.StageGroup{
		{Name: "task-flow", Stages: []string{"todo", "doing", "done"}, Cycle: types.CycleTerminate},
	})
	r.Kinds = types.NewKindRegistry([]types.Kind{
		{Name: "Task", StageGroup: "task-flow", Glyph: "◆", Colour: "#794aff"},
	})

	node := &types.Node{ID: "node-1", Body: "Target", Date: types.DateFields{Created: time.Now(), Modified: time.Now()}}
	blocker := &types.Node{ID: "blocker-1", Title: "Finished Prereq", Kind: "Task", Stage: "done"}
	edges := []*types.Edge{
		{ID: "e1", Type: string(types.EdgeBlocks), From: "blocker-1", To: "node-1"},
	}
	nodesByID := map[string]*types.Node{"blocker-1": blocker}

	output := stripANSI(r.Render(node, edges, nodesByID, nil, time.Now(), 80))
	if strings.Contains(output, "BLOCKED") {
		t.Errorf("expected no BLOCKED banner when the only blocker is terminal, got: %q", output)
	}
}

func TestProseRenderer_NilStageGroupsMakesBlockerUnresolvableAndBlocking(t *testing.T) {
	// A nil StageGroups registry means blocker terminality can never be
	// resolved, so the blocker still blocks — "presence blocks" per
	// types.EvalBlockers/Blockers semantics, mirrored from DetailRenderer.
	r := NewProseRenderer()
	node := &types.Node{ID: "node-1", Body: "Target", Date: types.DateFields{Created: time.Now(), Modified: time.Now()}}
	blocker := &types.Node{ID: "blocker-1", Title: "Prereq", Kind: "Task", Stage: "done"}
	edges := []*types.Edge{
		{ID: "e1", Type: string(types.EdgeBlocks), From: "blocker-1", To: "node-1"},
	}
	nodesByID := map[string]*types.Node{"blocker-1": blocker}

	output := stripANSI(r.Render(node, edges, nodesByID, nil, time.Now(), 80))
	if !strings.Contains(output, "BLOCKED") {
		t.Errorf("expected BLOCKED banner when StageGroups is nil (unresolvable blocker), got: %q", output)
	}
}

func TestProseRenderer_StalenessSuffix(t *testing.T) {
	r := NewProseRenderer()
	now := time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC)
	node := &types.Node{
		ID:   "node-1",
		Body: "Stale content",
		Date: types.DateFields{Created: now.Add(-60 * 24 * time.Hour), Modified: now.Add(-60 * 24 * time.Hour)},
	}

	output := stripANSI(r.Render(node, nil, nil, nil, now, 80))
	if !strings.Contains(output, "stale") {
		t.Errorf("expected staleness suffix, got: %q", output)
	}
	if !strings.Contains(output, "60d") {
		t.Errorf("expected 60d in staleness suffix, got: %q", output)
	}
}

func TestProseRenderer_NoStalenessSuffixWhenFresh(t *testing.T) {
	r := NewProseRenderer()
	now := time.Now()
	node := &types.Node{
		ID:   "node-1",
		Body: "Fresh content",
		Date: types.DateFields{Created: now, Modified: now},
	}

	output := stripANSI(r.Render(node, nil, nil, nil, now, 80))
	if strings.Contains(output, "stale") {
		t.Errorf("expected no staleness suffix for a freshly modified node, got: %q", output)
	}
}

func TestProseRenderer_KindStageLine(t *testing.T) {
	r := NewProseRenderer()
	r.Kinds = types.NewKindRegistry([]types.Kind{
		{Name: "Task", StageGroup: "task-flow", Glyph: "◆", Colour: "#794aff"},
	})
	node := &types.Node{
		ID: "node-1", Body: "Content", Kind: "Task", Stage: "doing",
		Date: types.DateFields{Created: time.Now(), Modified: time.Now()},
	}

	output := stripANSI(r.Render(node, nil, nil, nil, time.Now(), 80))
	if !strings.Contains(output, "Task") {
		t.Errorf("expected kind name 'Task' in output, got: %q", output)
	}
	if !strings.Contains(output, "doing") {
		t.Errorf("expected stage 'doing' in output, got: %q", output)
	}
}

func TestProseRenderer_KindStageLineFallsBackToStageOnly(t *testing.T) {
	// No registry, but the node still has a Stage — the plain muted stage
	// string is shown, matching DetailRenderer.renderKindStageLine's
	// fallback rung.
	r := NewProseRenderer()
	node := &types.Node{
		ID: "node-1", Body: "Content", Stage: "in-review",
		Date: types.DateFields{Created: time.Now(), Modified: time.Now()},
	}

	output := stripANSI(r.Render(node, nil, nil, nil, time.Now(), 80))
	if !strings.Contains(output, "in-review") {
		t.Errorf("expected fallback stage string 'in-review' in output, got: %q", output)
	}
}

func TestProseRenderer_BudgetsSection(t *testing.T) {
	r := NewProseRenderer()
	node := &types.Node{ID: "node-1", Body: "Content", Date: types.DateFields{Created: time.Now(), Modified: time.Now()}}
	budgetNode := makeBudgetNode("Groceries", 200.0, 0.8, []types.SpendEntry{
		{Date: "2024-03-01", Amount: 30.0, Note: "Supermarket"},
	})

	output := stripANSI(r.Render(node, nil, nil, []*types.Node{budgetNode}, time.Now(), 80))
	if !strings.Contains(output, "BUDGETS") {
		t.Errorf("expected BUDGETS section header, got: %q", output)
	}
	if !strings.Contains(output, "Groceries") {
		t.Errorf("expected budget node label 'Groceries', got: %q", output)
	}
	if !strings.Contains(output, "£30.00") {
		t.Errorf("expected spent total £30.00, got: %q", output)
	}
}

func TestProseRenderer_SpendLogSection(t *testing.T) {
	r := NewProseRenderer()
	node := makeBudgetNode("Groceries", 200.0, 0.8, []types.SpendEntry{
		{Date: "2024-03-01", Amount: 30.0, Note: "Supermarket"},
		{Date: "2024-03-07", Amount: 25.0, Note: "Market"},
	})

	output := stripANSI(r.Render(node, nil, nil, nil, time.Now(), 80))
	if !strings.Contains(output, "SPEND LOG") {
		t.Errorf("expected SPEND LOG section header, got: %q", output)
	}
	if !strings.Contains(output, "Supermarket") {
		t.Errorf("expected first spend note, got: %q", output)
	}
	if !strings.Contains(output, "£55.00") {
		t.Errorf("expected cumulative running total £55.00, got: %q", output)
	}
}

func TestProseRenderer_NoSpendLogSectionForNonBudgetNode(t *testing.T) {
	r := NewProseRenderer()
	node := &types.Node{ID: "node-1", Body: "Content", Types: []string{"note"}, Date: types.DateFields{Created: time.Now(), Modified: time.Now()}}

	output := stripANSI(r.Render(node, nil, nil, nil, time.Now(), 80))
	if strings.Contains(output, "SPEND LOG") {
		t.Errorf("expected no SPEND LOG section for a non-budget node, got: %q", output)
	}
}
