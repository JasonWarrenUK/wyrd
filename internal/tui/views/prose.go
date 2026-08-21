package views

import (
	"fmt"
	"image/color"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour"
	glamourStyles "github.com/charmbracelet/glamour/styles"
	"github.com/jasonwarrenuk/wyrd/internal/budget"
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

	// Archived is the colour for the ARCHIVED banner (TD.20a, mirrors
	// DetailRenderer's BudgetOver — the same "attention" red used there).
	Archived color.Color
	// BudgetOK, BudgetCaution, BudgetOver are the BUDGETS section's
	// status-glyph colours (TD.20a, mirror DetailRenderer's Colours fields
	// of the same names).
	BudgetOK      color.Color
	BudgetCaution color.Color
	BudgetOver    color.Color
}

// DefaultProsePalette returns the default Cairn-themed prose colours.
func DefaultProsePalette() ProsePalette {
	return ProsePalette{
		Title:         lipgloss.Color("#e0e0e0"),
		Body:          lipgloss.Color("#c8c8c8"),
		MetaKey:       lipgloss.Color("#b98300"),
		MetaValue:     lipgloss.Color("#e0e0e0"),
		EdgeType:      lipgloss.Color("#794aff"),
		EdgeGlyph:     lipgloss.Color("#6f6f6f"),
		Separator:     lipgloss.Color("#3a3a4a"),
		Muted:         lipgloss.Color("#8b8b8b"),
		AgeWarn:       lipgloss.Color("#d57300"),
		AgeCritical:   lipgloss.Color("#e0002b"),
		Background:    lipgloss.NoColor{},
		Archived:      lipgloss.Color("#e0002b"),
		BudgetOK:      lipgloss.Color("#34D399"),
		BudgetCaution: lipgloss.Color("#FBBF24"),
		BudgetOver:    lipgloss.Color("#F87171"),
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
// TD.20a completed parity: ARCHIVED/BLOCKED banners, staleness suffix,
// kind/stage line, BUDGETS section, SPEND LOG section.
type ProseRenderer struct {
	// Palette controls the colour scheme.
	Palette ProsePalette
	// Glyphs holds the directional arrow characters.
	Glyphs ProseGlyphs
	// Width is the available column width, used for Glamour's word wrap.
	Width int

	// Kinds is the merged kind registry used to resolve glyphs and colours
	// for node.Kind (TD.20a, mirrors DetailRenderer.Kinds). May be nil;
	// renderKindStageLine guards for nil before calling Lookup.
	Kinds *types.KindRegistry

	// StageGroups is the merged stage-group registry used to resolve
	// blocker terminality (TD.20a, mirrors DetailRenderer.StageGroups). May
	// be nil; a nil registry makes any blocker unresolvable, which
	// "presence blocks" per types.EvalBlockers/Blockers semantics.
	StageGroups *types.StageGroupRegistry

	// BlockedGlyph is the glyph shown in the BLOCKED banner and BLOCKED BY
	// section (TD.20a, mirrors DetailRenderer.BlockedGlyph). Defaults to
	// "✖" via NewProseRenderer; callers wire the live theme's glyph in to
	// respect user overrides.
	BlockedGlyph string

	// StaleGlyph is the glyph shown in the muted staleness suffix (TD.20a,
	// mirrors DetailRenderer.StaleGlyph). Defaults to "◌" via
	// NewProseRenderer.
	StaleGlyph string

	// StaleThresholdDays is the idle-days threshold used to decide whether
	// the staleness suffix shows (TD.20a, mirrors
	// DetailRenderer.StaleThresholdDays). <= 0 resolves to
	// types.DefaultStalenessThresholdDays via types.IsStale.
	StaleThresholdDays int

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
		Palette:      DefaultProsePalette(),
		Glyphs:       DefaultProseGlyphs(),
		Width:        80,
		BlockedGlyph: "✖",
		StaleGlyph:   "◌",
	}
}

// Render produces a styled prose string for node with its connected edges.
// nodesByID resolves each edge's peer to its title (TD.20) — mirroring
// DetailRenderer.Render's own nodesByID parameter, since without it edge
// peers can only be shown as raw UUIDs. budgetNodes are budget nodes
// associated with this node (TD.20a, mirrors DetailRenderer.Render's own
// parameter of the same name; may be empty). now is the reference time for
// the waiting_on ageing suffix, staleness, and budget period filtering.
// width is the available terminal width in characters, also used as
// r.Width for Glamour's word wrap.
func (r *ProseRenderer) Render(node *types.Node, edges []*types.Edge, nodesByID map[string]*types.Node, budgetNodes []*types.Node, now time.Time, width int) string {
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

	sectionHeaderStyle := lipgloss.NewStyle().
		Foreground(r.Palette.MetaKey).
		Background(bg).
		Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(r.Palette.Muted).Background(bg)

	isArchived := isArchivedNode(node)
	blockers := r.blockers(node, edges, nodesByID)

	var sb strings.Builder

	// --- ARCHIVED banner (TD.20a, mirrors DetailRenderer.Render) ---
	if isArchived {
		archivedStyle := lipgloss.NewStyle().Foreground(r.Palette.Archived).Background(bg).Bold(true)
		sb.WriteString(archivedStyle.Render("ARCHIVED"))
		sb.WriteString("\n\n")
	}

	// --- BLOCKED banner (TD.20a, mirrors DetailRenderer.Render) ---
	if len(blockers) > 0 {
		sb.WriteString(BlockedBadge(r.BlockedGlyph))
		sb.WriteString("\n\n")
	}

	// --- Staleness suffix (TD.20a, mirrors DetailRenderer.Render) ---
	if types.IsStale(node, now, r.StaleThresholdDays) {
		days := types.DaysSince(node.Date.Modified, now)
		suffix := fmt.Sprintf("%s stale · %dd", r.StaleGlyph, days)
		sb.WriteString(mutedStyle.Render(suffix))
		sb.WriteString("\n\n")
	}

	// --- Kind / Stage (TD.20a, mirrors DetailRenderer.Render) ---
	if line, ok := r.renderKindStageLine(node, bg); ok {
		sb.WriteString(line)
		sb.WriteString("\n\n")
	}

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

	// --- BLOCKED BY section (TD.20a, mirrors DetailRenderer.Render) ---
	if len(blockers) > 0 {
		sb.WriteRune('\n')
		sb.WriteString(sectionHeaderStyle.Render("BLOCKED BY"))
		sb.WriteRune('\n')
		blockedGlyphStyle := lipgloss.NewStyle().Foreground(r.Palette.Archived).Background(bg)
		for _, blocker := range blockers {
			sb.WriteString(blockedGlyphStyle.Render(r.BlockedGlyph) + "  " + mutedStyle.Render(NodeTitle(blocker)))
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

	// --- BUDGETS section (TD.20a, mirrors DetailRenderer.Render) ---
	if len(budgetNodes) > 0 {
		sb.WriteRune('\n')
		sb.WriteString(sectionHeaderStyle.Render("BUDGETS"))
		sb.WriteRune('\n')
		for _, bn := range budgetNodes {
			sb.WriteString(r.renderBudgetLine(bn, now, bg))
			sb.WriteRune('\n')
		}
	}

	// --- SPEND LOG section (TD.20a, mirrors DetailRenderer.Render;
	// budget nodes only, full-history running total, not period-scoped) ---
	if nodeHasType(node, "budget") {
		entries := budget.SpendLog(node)
		if len(entries) > 0 {
			sorted := make([]types.SpendEntry, len(entries))
			copy(sorted, entries)
			sort.SliceStable(sorted, func(i, j int) bool {
				return sorted[i].Date < sorted[j].Date
			})

			sb.WriteRune('\n')
			sb.WriteString(sectionHeaderStyle.Render("SPEND LOG"))
			sb.WriteRune('\n')
			var running float64
			for _, e := range sorted {
				running += e.Amount
				sb.WriteString(r.renderSpendLogLine(e, running, bg))
				sb.WriteRune('\n')
			}
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// isArchivedNode reports whether node's status property is "archived"
// (TD.20a, mirrors internal/tui.isArchivedNode).
func isArchivedNode(node *types.Node) bool {
	if node.Properties == nil {
		return false
	}
	status, _ := node.Properties["status"].(string)
	return status == "archived"
}

// nodeHasType reports whether node.Types contains typeName (TD.20a, mirrors
// internal/tui.nodeHasType).
func nodeHasType(node *types.Node, typeName string) bool {
	for _, t := range node.Types {
		if t == typeName {
			return true
		}
	}
	return false
}

// blockers returns the nodes currently holding a block on node (TD.20a,
// mirrors internal/tui.DetailRenderer.blockers exactly).
func (r *ProseRenderer) blockers(node *types.Node, edges []*types.Edge, nodesByID map[string]*types.Node) []*types.Node {
	if node == nil {
		return nil
	}
	var blockers []*types.Node
	for _, edge := range edges {
		if edge.Type != string(types.EdgeBlocks) || edge.To != node.ID {
			continue
		}
		source, ok := nodesByID[edge.From]
		if !ok || source == nil {
			continue
		}
		group, resolved := types.ResolveStageGroup(r.Kinds, r.StageGroups, source)
		if !resolved || !group.IsTerminal(source.Stage) {
			blockers = append(blockers, source)
		}
	}
	return blockers
}

// renderKindStageLine builds the "◆ Task · doing" line shown immediately
// after the ARCHIVED/BLOCKED/staleness lines (TD.20a, mirrors
// internal/tui.DetailRenderer.renderKindStageLine).
//
// Resolution order:
//  1. Registry present + node.Kind resolves → glyph (kind colour) + kind name + stage.
//  2. Kind empty/unresolved, stage present → plain muted stage string.
//  3. Both empty → returns ("", false) so the caller omits the block entirely.
func (r *ProseRenderer) renderKindStageLine(node *types.Node, bg color.Color) (string, bool) {
	mutedStyle := lipgloss.NewStyle().Foreground(r.Palette.Muted).Background(bg)
	primaryStyle := lipgloss.NewStyle().Foreground(r.Palette.Body).Background(bg)

	if r.Kinds != nil && node.Kind != "" {
		if k, ok := r.Kinds.Lookup(node.Kind); ok {
			kindStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(k.Colour)).Background(bg)
			line := kindStyle.Render(k.Glyph) + "  " + primaryStyle.Render(k.Name)
			if node.Stage != "" {
				line += "  " + mutedStyle.Render("· "+node.Stage)
			}
			return line, true
		}
	}

	if node.Stage != "" {
		return mutedStyle.Render(node.Stage), true
	}

	return "", false
}

// renderBudgetLine produces a compact one-line budget summary with a
// progress bar (TD.20a, mirrors internal/tui.DetailRenderer.renderBudgetLine).
func (r *ProseRenderer) renderBudgetLine(node *types.Node, now time.Time, bg color.Color) string {
	summary := budget.Compute(node, now)

	var statusColour color.Color
	var statusGlyph string
	switch summary.Status {
	case budget.BudgetOK:
		statusColour = r.Palette.BudgetOK
		statusGlyph = "●"
	case budget.BudgetCaution:
		statusColour = r.Palette.BudgetCaution
		statusGlyph = "◆"
	case budget.BudgetOver:
		statusColour = r.Palette.BudgetOver
		statusGlyph = "▲"
	}

	categoryLabel := ""
	if node.Properties != nil {
		if cat, ok := node.Properties["category"].(string); ok {
			categoryLabel = cat
		}
	}
	if categoryLabel == "" {
		categoryLabel = NodeTitle(node)
	}

	bar := buildProgressBar(summary.Spent, summary.Allocated, 10)

	statusStyle := lipgloss.NewStyle().Foreground(statusColour).Background(bg)
	mutedStyle := lipgloss.NewStyle().Foreground(r.Palette.Muted).Background(bg)
	primaryStyle := lipgloss.NewStyle().Foreground(r.Palette.Body).Background(bg)

	return statusStyle.Render(statusGlyph) + "  " +
		primaryStyle.Render(categoryLabel) + "  " +
		mutedStyle.Render(bar) + "  " +
		primaryStyle.Render(fmt.Sprintf("£%.2f", summary.Spent)) +
		mutedStyle.Render(fmt.Sprintf("/£%.2f", summary.Allocated))
}

// buildProgressBar returns a fixed-width ASCII progress bar string
// (TD.20a, mirrors internal/tui.buildProgressBar exactly).
func buildProgressBar(spent, allocated float64, width int) string {
	if allocated <= 0 || width <= 0 {
		return strings.Repeat("─", width)
	}
	fraction := spent / allocated
	if fraction > 1 {
		fraction = 1
	}
	filled := int(fraction * float64(width))
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "]"
}

// renderSpendLogLine produces a single spend-log entry line: date, amount,
// note, running total (TD.20a, mirrors
// internal/tui.DetailRenderer.renderSpendLogLine).
func (r *ProseRenderer) renderSpendLogLine(e types.SpendEntry, running float64, bg color.Color) string {
	mutedStyle := lipgloss.NewStyle().Foreground(r.Palette.Muted).Background(bg)
	primaryStyle := lipgloss.NewStyle().Foreground(r.Palette.Body).Background(bg)

	line := mutedStyle.Render(e.Date) + "  " + primaryStyle.Render(fmt.Sprintf("£%.2f", e.Amount))
	if e.Note != "" {
		line += "  " + mutedStyle.Render(e.Note)
	}
	line += "  " + mutedStyle.Render(fmt.Sprintf("(£%.2f)", running))
	return line
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
