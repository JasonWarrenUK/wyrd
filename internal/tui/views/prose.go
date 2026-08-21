package views

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour"
	glamourStyles "github.com/charmbracelet/glamour/styles"
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
	Title color.Color
	// Body is the default body text colour.
	Body color.Color
	// MetaKey is the colour for metadata field labels.
	MetaKey color.Color
	// MetaValue is the colour for metadata field values.
	MetaValue color.Color
	// EdgeType is the colour for the edge-type label.
	EdgeType color.Color
	// EdgeGlyph is the colour for directional arrow glyphs.
	EdgeGlyph color.Color
	// Separator is the colour for horizontal rules.
	Separator color.Color
	// Muted is used for empty-state messaging, and as the youngest-tier edge
	// age colour (mirrors DetailRenderer's FGMuted, TD.20).
	Muted color.Color
	// AgeWarn is the edge-age colour for the 8-14 day tier (TD.20, mirrors
	// DetailRenderer's OverflowWarn).
	AgeWarn color.Color
	// AgeCritical is the edge-age colour for the 15+ day tier (TD.20,
	// mirrors DetailRenderer's OverflowCrit).
	AgeCritical color.Color
	// Background is the pane background colour, needed so Glamour's
	// rendered markdown doesn't bleed the terminal default at inter-block
	// gaps (TD.20, mirrors DetailRenderer.bg()'s same fix).
	Background color.Color
}

// DefaultProsePalette returns the default Cairn-themed prose colours.
func DefaultProsePalette() ProsePalette {
	return ProsePalette{
		Title:       lipgloss.Color("#e0e0e0"),
		Body:        lipgloss.Color("#c8c8c8"),
		MetaKey:     lipgloss.Color("#b98300"),
		MetaValue:   lipgloss.Color("#e0e0e0"),
		EdgeType:    lipgloss.Color("#794aff"),
		EdgeGlyph:   lipgloss.Color("#6f6f6f"),
		Separator:   lipgloss.Color("#3a3a4a"),
		Muted:       lipgloss.Color("#8b8b8b"),
		AgeWarn:     lipgloss.Color("#d57300"),
		AgeCritical: lipgloss.Color("#e0002b"),
		Background:  lipgloss.NoColor{},
	}
}

// ProseRenderer renders a single node as a wiki-style detail view. The output
// shows the body as Glamour-rendered markdown, followed by metadata fields,
// then connected edges with directional glyphs and resolved peer titles.
//
// TD.20 brought this to parity with internal/tui.DetailRenderer on the
// dimensions that renderer's own contract covers for a plain single-node
// view: Glamour markdown, resolved edge-peer titles, an ageing suffix on
// waiting_on edges, and theme-wired colours instead of a hardcoded palette.
// DetailRenderer's node-class-specific sections (ARCHIVED/BLOCKED banners,
// staleness suffix, kind/stage line, BUDGETS, SPEND LOG) are out of scope
// here — see TD.20b.
type ProseRenderer struct {
	// Palette controls the colour scheme.
	Palette ProsePalette
	// Glyphs holds the directional arrow characters.
	Glyphs ProseGlyphs
	// Width is the available column width, used for Glamour's word wrap.
	Width int

	// glamRenderer caches the Glamour terminal renderer (mirrors
	// DetailRenderer.renderMarkdown's own cache), recreated when Width,
	// dark/light mode, or the active theme changes.
	glamRenderer *glamour.TermRenderer
	glamWidth    int
	glamDark     bool
	glamTheme    string
}

// NewProseRenderer returns a renderer with default palette, glyphs, and width.
func NewProseRenderer() *ProseRenderer {
	return &ProseRenderer{
		Palette: DefaultProsePalette(),
		Glyphs:  DefaultProseGlyphs(),
		Width:   80,
	}
}

// Render produces a styled prose string for node with its connected edges.
// nodesByID resolves each edge's peer to its title (TD.20) — mirroring
// DetailRenderer.Render's own nodesByID parameter, since without it edge
// peers can only be shown as raw UUIDs. now is the reference time for the
// waiting_on ageing suffix. width is the available terminal width in
// characters, also used as r.Width for Glamour's word wrap.
func (r *ProseRenderer) Render(node *types.Node, edges []*types.Edge, nodesByID map[string]*types.Node, now time.Time, width int) string {
	bg := r.bg()

	if node == nil {
		return lipgloss.NewStyle().
			Foreground(r.Palette.Muted).
			Background(bg).
			Render("No node selected.")
	}

	r.Width = width

	sep := strings.Repeat("─", width)
	separatorStyled := lipgloss.NewStyle().
		Foreground(r.Palette.Separator).
		Background(bg).
		Render(sep)

	var sb strings.Builder

	// Body section: Glamour-rendered markdown (TD.20), matching
	// DetailRenderer.renderMarkdown — falls back to plain styled text on a
	// Glamour construction/render error.
	bodyStyle := lipgloss.NewStyle().Foreground(r.Palette.Body).Background(bg)
	sb.WriteString(r.renderMarkdown(node.Body, bodyStyle))

	// Metadata section.
	meta := r.buildMetaLines(node, bg)
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
	edgeLines := r.buildEdgeLines(node, edges, nodesByID, now, bg)
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

// bg returns the pane background colour, matching DetailRenderer.bg's own
// nil-tolerant default.
func (r *ProseRenderer) bg() color.Color {
	if r.Palette.Background != nil {
		return r.Palette.Background
	}
	return lipgloss.NoColor{}
}

// renderMarkdown renders body text as markdown using Glamour, mirroring
// DetailRenderer.renderMarkdown (TD.20): same style-config shape (margin
// zeroed, background cleared so the pane background shows through, accent
// colours wired from the palette), same dark/light + width + theme cache
// invalidation, same FillBackground pass to stop Glamour's un-backgrounded
// blank lines bleeding the terminal default. On any renderer construction
// or render error, falls back to plainStyle.Render(body).
func (r *ProseRenderer) renderMarkdown(body string, plainStyle lipgloss.Style) string {
	dark := IsColourDark(r.bg())
	themeKey := ColourHex(r.Palette.Title)

	if r.glamRenderer == nil || r.glamWidth != r.Width || r.glamDark != dark || r.glamTheme != themeKey {
		style := glamourStyles.DarkStyleConfig
		if !dark {
			style = glamourStyles.LightStyleConfig
		}
		var zero uint = 0
		style.Document.Margin = &zero
		style.Document.BackgroundColor = nil
		style.Document.Color = HexPtr(r.Palette.Body)

		style.H1.Color = HexPtr(r.Palette.Title)
		style.H1.BackgroundColor = nil
		style.H2.Color = HexPtr(r.Palette.MetaKey)
		style.H2.BackgroundColor = nil
		style.H3.Color = HexPtr(r.Palette.MetaKey)
		style.H3.BackgroundColor = nil

		style.Link.Color = HexPtr(r.Palette.MetaKey)
		style.LinkText.Color = HexPtr(r.Palette.Body)

		style.Emph.Color = HexPtr(r.Palette.Muted)
		style.Strong.Color = HexPtr(r.Palette.Body)

		style.Code.Color = HexPtr(r.Palette.Title)
		style.Code.BackgroundColor = nil
		style.CodeBlock.BackgroundColor = nil

		renderer, err := glamour.NewTermRenderer(
			glamour.WithStyles(style),
			glamour.WithWordWrap(r.Width),
		)
		if err != nil {
			return plainStyle.Render(body)
		}
		r.glamRenderer = renderer
		r.glamWidth = r.Width
		r.glamDark = dark
		r.glamTheme = themeKey
	}

	out, err := r.glamRenderer.Render(body)
	if err != nil {
		return plainStyle.Render(body)
	}
	// Unlike DetailRenderer.renderMarkdown, this does not call FillBackground
	// on Glamour's output here: FillBackground lives in package tui (used
	// pane-wide per CLAUDE.md's TUI styling rules), and views cannot import
	// tui. The caller (viewPane.renderProse, TD.20) applies it to this
	// renderer's full Render() output instead, achieving the same
	// background-bleed fix at the one point both packages can reach.
	return strings.TrimRight(out, "\n")
}

// buildMetaLines produces the styled metadata key-value lines for a node.
func (r *ProseRenderer) buildMetaLines(node *types.Node, bg color.Color) []string {
	keyStyle := lipgloss.NewStyle().Foreground(r.Palette.MetaKey).Background(bg)
	valStyle := lipgloss.NewStyle().Foreground(r.Palette.MetaValue).Background(bg)

	var lines []string

	// Always show ID and types.
	lines = append(lines,
		fmt.Sprintf("%s  %s", keyStyle.Render("id"), valStyle.Render(node.ID)),
		fmt.Sprintf("%s  %s", keyStyle.Render("types"), valStyle.Render(strings.Join(node.Types, ", "))),
		fmt.Sprintf("%s  %s", keyStyle.Render("created"), valStyle.Render(node.Date.Created.Format("2006-01-02 15:04"))),
		fmt.Sprintf("%s  %s", keyStyle.Render("modified"), valStyle.Render(node.Date.Modified.Format("2006-01-02 15:04"))),
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

// buildEdgeLines produces the styled edge description lines. Edges from
// node are outgoing; edges to node are incoming. Peer nodes resolve to
// their title via nodesByID (TD.20) rather than showing a raw UUID, and
// waiting_on edges carry an ageing suffix coloured per AgeColourForDays —
// both matching DetailRenderer.renderEdgeLine's contract.
func (r *ProseRenderer) buildEdgeLines(node *types.Node, edges []*types.Edge, nodesByID map[string]*types.Node, now time.Time, bg color.Color) []string {
	if len(edges) == 0 {
		return nil
	}

	glyphStyle := lipgloss.NewStyle().Foreground(r.Palette.EdgeGlyph).Background(bg)
	typeStyle := lipgloss.NewStyle().Foreground(r.Palette.EdgeType).Background(bg)
	peerStyle := lipgloss.NewStyle().Foreground(r.Palette.MetaValue).Background(bg)

	var lines []string
	for _, edge := range edges {
		outgoing := edge.From == node.ID
		glyph := r.edgeGlyph(edge.Type, outgoing)

		otherID := edge.To
		if !outgoing {
			otherID = edge.From
		}
		peerLabel := otherID
		if other, ok := nodesByID[otherID]; ok {
			peerLabel = NodeTitle(other)
		}

		line := fmt.Sprintf("%s  %s  %s",
			glyphStyle.Render(glyph),
			typeStyle.Render(edge.Type),
			peerStyle.Render(peerLabel),
		)

		if edge.Type == string(types.EdgeWaitingOn) {
			age := now.Sub(edge.Date.Created)
			days := int(age.Hours() / 24)
			ageColour := AgeColourForDays(days, AgeColours{
				Muted:    r.Palette.Muted,
				Warn:     r.Palette.AgeWarn,
				Critical: r.Palette.AgeCritical,
			})
			line += lipgloss.NewStyle().Foreground(ageColour).Background(bg).
				Render(fmt.Sprintf(" · %dd", days))
		}

		lines = append(lines, line)
	}

	return lines
}

// edgeGlyph selects the glyph for an edge, preferring r.Glyphs' own
// dedicated EdgeParent/EdgeRelated glyphs for those edge types (this is
// ProseGlyphs' whole purpose — a caller-customisable glyph set, per its own
// doc comment), and falling back to the shared views.EdgeGlyph direction
// logic (EdgeFrom/EdgeTo, matching DetailRenderer's own arrows) for every
// other edge type. Restores the pre-parity behaviour that TD.20's initial
// pass accidentally dropped when it switched buildEdgeLines over to
// views.EdgeGlyph unconditionally, silently making r.Glyphs dead.
func (r *ProseRenderer) edgeGlyph(edgeType string, outgoing bool) string {
	switch edgeType {
	case string(types.EdgeParent):
		return r.Glyphs.EdgeParent
	case string(types.EdgeRelated):
		return r.Glyphs.EdgeRelated
	default:
		if outgoing {
			return r.Glyphs.EdgeFrom
		}
		return r.Glyphs.EdgeTo
	}
}
