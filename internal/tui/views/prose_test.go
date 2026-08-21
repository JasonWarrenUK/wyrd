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
	output := stripANSI(r.Render(nil, nil, nil, time.Now(), 80))
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

	output := stripANSI(r.Render(node, nil, nil, time.Now(), 80))
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

	output := stripANSI(r.Render(node, nil, nil, time.Now(), 80))

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

	output := stripANSI(r.Render(node, edges, nil, time.Now(), 80))

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

	output := stripANSI(r.Render(node, edges, map[string]*types.Node{"node-2": peer}, time.Now(), 80))

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

	output := stripANSI(r.Render(node, edges, nil, now, 80))
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

	output := stripANSI(r.Render(node, edges, nil, time.Now(), 80))
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

	output := stripANSI(r.Render(node, edges, nil, time.Now(), 80))
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

	output := stripANSI(r.Render(node, edges, nil, time.Now(), 80))
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

	output := stripANSI(r.Render(node, edges, nil, time.Now(), 80))
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

	output := stripANSI(r.Render(node, nil, nil, time.Now(), 80))
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

	output := stripANSI(r.Render(node, nil, nil, time.Now(), 80))
	if !strings.Contains(output, "github") {
		t.Error("expected source type 'github' in metadata")
	}
	if !strings.Contains(output, "https://github.com") {
		t.Error("expected source URL in metadata")
	}
}
