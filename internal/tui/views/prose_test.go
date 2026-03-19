package views

import (
	"strings"
	"testing"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

func TestProseRenderer_NilNode(t *testing.T) {
	r := NewProseRenderer()
	output := r.Render(nil, nil, 80)
	if !strings.Contains(output, "No node selected") {
		t.Errorf("expected empty-state message, got: %q", output)
	}
}

func TestProseRenderer_RendersBody(t *testing.T) {
	r := NewProseRenderer()
	node := &types.Node{
		ID:       "abc-123",
		Body:     "# My Node\n\nThis is the body content.",
		Types:    []string{"note"},
		Created:  time.Now(),
		Modified: time.Now(),
	}

	output := r.Render(node, nil, 80)
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
		ID:       "abc-123",
		Body:     "Test",
		Types:    []string{"task", "project"},
		Created:  time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC),
		Modified: time.Date(2024, 3, 16, 12, 0, 0, 0, time.UTC),
	}

	output := r.Render(node, nil, 80)

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
		ID:       "node-1",
		Body:     "Source node",
		Types:    []string{"task"},
		Created:  time.Now(),
		Modified: time.Now(),
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

	output := r.Render(node, edges, 80)

	if !strings.Contains(output, "blocks") {
		t.Error("expected 'blocks' edge type in output")
	}
	if !strings.Contains(output, "node-2") {
		t.Error("expected peer node ID in edge output")
	}
	if !strings.Contains(output, "related") {
		t.Error("expected 'related' edge type in output")
	}
}

func TestProseRenderer_OutgoingEdgeGlyph(t *testing.T) {
	r := NewProseRenderer()
	node := &types.Node{
		ID:       "node-1",
		Body:     "Source",
		Types:    []string{"task"},
		Created:  time.Now(),
		Modified: time.Now(),
	}
	edges := []*types.Edge{
		{
			ID:   "edge-1",
			Type: "blocks",
			From: "node-1", // outgoing from this node
			To:   "node-2",
		},
	}

	output := r.Render(node, edges, 80)
	if !strings.Contains(output, r.Glyphs.EdgeFrom) {
		t.Errorf("expected outgoing glyph %q for outgoing edge", r.Glyphs.EdgeFrom)
	}
}

func TestProseRenderer_IncomingEdgeGlyph(t *testing.T) {
	r := NewProseRenderer()
	node := &types.Node{
		ID:       "node-1",
		Body:     "Target",
		Types:    []string{"task"},
		Created:  time.Now(),
		Modified: time.Now(),
	}
	edges := []*types.Edge{
		{
			ID:   "edge-1",
			Type: "blocks",
			From: "node-2", // incoming to this node
			To:   "node-1",
		},
	}

	output := r.Render(node, edges, 80)
	if !strings.Contains(output, r.Glyphs.EdgeTo) {
		t.Errorf("expected incoming glyph %q for incoming edge", r.Glyphs.EdgeTo)
	}
}

func TestProseRenderer_NoEdges(t *testing.T) {
	r := NewProseRenderer()
	node := &types.Node{
		ID:       "node-1",
		Body:     "Isolated node",
		Types:    []string{"note"},
		Created:  time.Now(),
		Modified: time.Now(),
	}

	output := r.Render(node, nil, 80)
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
		Created:  time.Now(),
		Modified: time.Now(),
	}

	output := r.Render(node, nil, 80)
	if !strings.Contains(output, "github") {
		t.Error("expected source type 'github' in metadata")
	}
	if !strings.Contains(output, "https://github.com") {
		t.Error("expected source URL in metadata")
	}
}
