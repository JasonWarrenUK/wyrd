package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// ProseGlyphs holds the directional arrow characters used in the edge section.
type ProseGlyphs struct {
	// EdgeFrom is shown for edges originating from this node (outgoing).
	EdgeFrom string
	// EdgeTo is shown for edges targeting this node (incoming).
	EdgeTo string
	// EdgeRelated is shown for loosely-associated edges.
	EdgeRelated string
	// EdgeParent is shown for containment relationships.
	EdgeParent string
}

// DefaultProseGlyphs returns the canonical prose edge glyphs.
func DefaultProseGlyphs() ProseGlyphs {
	return ProseGlyphs{
		EdgeFrom:    "→",
		EdgeTo:      "←",
		EdgeRelated: "◇",
		EdgeParent:  "⊘",
	}
}

// ProsePalette holds the colours used by the prose renderer.
type ProsePalette struct {
	// Title is the colour for the node body heading.
	Title string
	// Body is the default body text colour.
	Body string
	// MetaKey is the colour for metadata field labels.
	MetaKey string
	// MetaValue is the colour for metadata field values.
	MetaValue string
	// EdgeType is the colour for the edge-type label.
	EdgeType string
	// EdgeGlyph is the colour for directional arrow glyphs.
	EdgeGlyph string
	// Separator is the colour for horizontal rules.
	Separator string
	// Muted is used for empty-state messaging.
	Muted string
}

// DefaultProsePalette returns the default Cairn-themed prose colours.
func DefaultProsePalette() ProsePalette {
	return ProsePalette{
		Title:     "#e0e0e0",
		Body:      "#c8c8c8",
		MetaKey:   "#b98300",
		MetaValue: "#e0e0e0",
		EdgeType:  "#794aff",
		EdgeGlyph: "#6f6f6f",
		Separator: "#3a3a4a",
		Muted:     "#8b8b8b",
	}
}

// ProseRenderer renders a single node as a wiki-style detail view. The output
// shows the body as the primary content, followed by metadata fields, then
// connected edges with directional glyphs.
type ProseRenderer struct {
	// Palette controls the colour scheme.
	Palette ProsePalette
	// Glyphs holds the directional arrow characters.
	Glyphs ProseGlyphs
}

// NewProseRenderer returns a renderer with default palette and glyphs.
func NewProseRenderer() *ProseRenderer {
	return &ProseRenderer{
		Palette: DefaultProsePalette(),
		Glyphs:  DefaultProseGlyphs(),
	}
}

// Render produces a styled prose string for node with its connected edges.
// width is the available terminal width in characters.
func (r *ProseRenderer) Render(node *types.Node, edges []*types.Edge, width int) string {
	if node == nil {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color(r.Palette.Muted)).
			Render("No node selected.")
	}

	sep := strings.Repeat("─", width)
	separatorStyled := lipgloss.NewStyle().
		Foreground(lipgloss.Color(r.Palette.Separator)).
		Render(sep)

	var sb strings.Builder

	// Body section.
	bodyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(r.Palette.Body))
	sb.WriteString(bodyStyle.Render(node.Body))

	// Metadata section.
	meta := r.buildMetaLines(node)
	if len(meta) > 0 {
		sb.WriteRune('\n')
		sb.WriteString(separatorStyled)
		sb.WriteRune('\n')
		for _, line := range meta {
			sb.WriteString(line)
			sb.WriteRune('\n')
		}
	}

	// Edges section.
	edgeLines := r.buildEdgeLines(node, edges)
	if len(edgeLines) > 0 {
		sb.WriteRune('\n')
		sb.WriteString(separatorStyled)
		sb.WriteRune('\n')
		for _, line := range edgeLines {
			sb.WriteString(line)
			sb.WriteRune('\n')
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// buildMetaLines produces the styled metadata key-value lines for a node.
func (r *ProseRenderer) buildMetaLines(node *types.Node) []string {
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(r.Palette.MetaKey))
	valStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(r.Palette.MetaValue))

	var lines []string

	// Always show ID and types.
	lines = append(lines,
		fmt.Sprintf("%s  %s", keyStyle.Render("id"), valStyle.Render(node.ID)),
		fmt.Sprintf("%s  %s", keyStyle.Render("types"), valStyle.Render(strings.Join(node.Types, ", "))),
		fmt.Sprintf("%s  %s", keyStyle.Render("created"), valStyle.Render(node.Created.Format("2006-01-02 15:04"))),
		fmt.Sprintf("%s  %s", keyStyle.Render("modified"), valStyle.Render(node.Modified.Format("2006-01-02 15:04"))),
	)

	// Show source if present.
	if node.Source != nil {
		lines = append(lines,
			fmt.Sprintf("%s  %s", keyStyle.Render("source"), valStyle.Render(node.Source.Type)),
		)
		if node.Source.URL != "" {
			lines = append(lines,
				fmt.Sprintf("%s  %s", keyStyle.Render("url"), valStyle.Render(node.Source.URL)),
			)
		}
	}

	// Show user-defined properties.
	for k, v := range node.Properties {
		lines = append(lines,
			fmt.Sprintf("%s  %s", keyStyle.Render(k), valStyle.Render(fmt.Sprintf("%v", v))),
		)
	}

	return lines
}

// buildEdgeLines produces the styled edge description lines.
// Edges from node are outgoing (→); edges to node are incoming (←).
func (r *ProseRenderer) buildEdgeLines(node *types.Node, edges []*types.Edge) []string {
	if len(edges) == 0 {
		return nil
	}

	glyphStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(r.Palette.EdgeGlyph))
	typeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(r.Palette.EdgeType))

	var lines []string
	for _, edge := range edges {
		var glyph, direction, peer string

		switch edge.Type {
		case string(types.EdgeParent):
			glyph = r.Glyphs.EdgeParent
		case string(types.EdgeRelated):
			glyph = r.Glyphs.EdgeRelated
		default:
			if edge.From == node.ID {
				glyph = r.Glyphs.EdgeFrom
			} else {
				glyph = r.Glyphs.EdgeTo
			}
		}

		if edge.From == node.ID {
			direction = "→"
			peer = edge.To
		} else {
			direction = "←"
			peer = edge.From
		}
		_ = direction

		lines = append(lines, fmt.Sprintf("%s  %s  %s",
			glyphStyle.Render(glyph),
			typeStyle.Render(edge.Type),
			peer,
		))
	}

	return lines
}
