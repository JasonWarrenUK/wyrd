// Package tui provides terminal UI rendering components for Wyrd.
package tui

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

// Colours holds the colour values used by DetailRenderer.
type Colours struct {
	AccentPrimary   color.Color
	AccentSecondary color.Color
	BgPrimary       color.Color
	FGPrimary       color.Color
	FGMuted         color.Color
	BudgetOK        color.Color
	BudgetCaution   color.Color
	BudgetOver      color.Color
	OverflowWarn    color.Color
	OverflowCrit    color.Color
}

// defaultColours returns a set of sensible defaults.
func defaultColours() Colours {
	return Colours{
		AccentPrimary:   lipgloss.Color("#7C8CF8"),
		AccentSecondary: lipgloss.Color("#A78BFA"),
		FGPrimary:       lipgloss.Color("#E2E8F0"),
		FGMuted:         lipgloss.Color("#64748B"),
		BudgetOK:        lipgloss.Color("#34D399"),
		BudgetCaution:   lipgloss.Color("#FBBF24"),
		BudgetOver:      lipgloss.Color("#F87171"),
		OverflowWarn:    lipgloss.Color("#FB923C"),
		OverflowCrit:    lipgloss.Color("#EF4444"),
	}
}

// DetailRenderer renders the right-hand detail pane for a selected node.
type DetailRenderer struct {
	// Width is the column width available for rendering.
	Width int

	// Colours holds the colour palette. Set via NewDetailRenderer or mutated after construction.
	Colours Colours

	// Kinds is the merged kind registry used to resolve glyphs and colours for node.Kind.
	// May be nil (e.g. in tests); renderKindStageLine guards for nil before calling Lookup.
	Kinds *types.KindRegistry

	// StageGroups is the merged stage-group registry used to resolve blocker
	// terminality (DL.2, mirrors the query engine's WithStageResolver). May be
	// nil (e.g. in tests); a nil registry makes any blocker unresolvable,
	// which "presence blocks" per types.EvalBlockers/Blockers semantics.
	StageGroups *types.StageGroupRegistry

	// BlockedGlyph is the glyph shown in the BLOCKED banner and BLOCKED BY
	// section (DL.2). Defaults to "✖" via NewDetailRenderer, matching
	// types.ThemeGlyphs.Blocked's built-in default; callers wire the live
	// theme's glyph in to respect user overrides.
	BlockedGlyph string

	// StaleGlyph is the glyph shown in the muted staleness suffix (DL.3).
	// Defaults to "◌" via NewDetailRenderer, matching
	// types.ThemeGlyphs.Stale's built-in default; callers wire the live
	// theme's glyph in to respect user overrides.
	StaleGlyph string

	// StaleThresholdDays is the idle-days threshold used to decide whether
	// the staleness suffix shows (DL.3). <= 0 resolves to
	// types.DefaultStalenessThresholdDays via types.IsStale.
	StaleThresholdDays int

	// glamRenderer caches the Glamour terminal renderer so it is only created
	// once (or recreated when Width, dark/light mode, or the active theme changes).
	glamRenderer *glamour.TermRenderer
	glamWidth    int
	glamDark     bool   // tracks whether the last renderer was built for a dark theme
	glamTheme    string // hex fingerprint of AccentPrimary — invalidates cache on theme switch
}

// NewDetailRenderer creates a DetailRenderer with default colours and a sensible width.
func NewDetailRenderer() *DetailRenderer {
	return &DetailRenderer{
		Width:        80,
		Colours:      defaultColours(),
		BlockedGlyph: "✖",
		StaleGlyph:   "◌",
	}
}

// Render produces the full detail pane string for a node.
//
// Parameters:
//   - node        — the node to render
//   - edges       — all edges connected to the node (both incoming and outgoing)
//   - nodesByID   — map of all nodes keyed by UUID, used to resolve edge targets
//   - budgetNodes — budget nodes associated with this node (may be empty)
//   - now         — reference time for age calculations and budget period filtering
func (r *DetailRenderer) Render(
	node *types.Node,
	edges []*types.Edge,
	nodesByID map[string]*types.Node,
	budgetNodes []*types.Node,
	now time.Time,
) string {
	c := r.Colours

	// IMPORTANT: every style must carry Background(bg) to prevent terminal
	// default bleeding through at ANSI reset boundaries between segments.
	// See feedback_statusbar_background_bleed.md — same root cause.
	bg := r.bg()

	titleStyle := lipgloss.NewStyle().
		Foreground(c.AccentPrimary).
		Background(bg).
		Bold(true)

	mutedStyle := lipgloss.NewStyle().
		Foreground(c.FGMuted).
		Background(bg)

	primaryStyle := lipgloss.NewStyle().
		Foreground(c.FGPrimary).
		Background(bg)

	sectionHeaderStyle := lipgloss.NewStyle().
		Foreground(c.AccentSecondary).
		Background(bg).
		Bold(true)

	isArchived := isArchivedNode(node)
	blockers := r.blockers(node, edges, nodesByID)

	var sb strings.Builder

	// --- ARCHIVED banner ---
	if isArchived {
		archivedStyle := lipgloss.NewStyle().
			Foreground(c.BudgetOver).
			Background(bg).
			Bold(true)
		sb.WriteString(archivedStyle.Render("ARCHIVED"))
		sb.WriteString("\n\n")
	}

	// --- BLOCKED banner (DL.2) ---
	if len(blockers) > 0 {
		sb.WriteString(BlockedBadge(r.BlockedGlyph))
		sb.WriteString("\n\n")
	}

	// --- Title ---
	title := nodeTitle(node)
	sb.WriteString(titleStyle.Render(title))

	// --- Staleness suffix (DL.3) ---
	// Deliberately muted (dim foreground, no bold pill) so it reads as a
	// quiet aside next to the title rather than competing with the bold
	// BLOCKED banner above.
	if types.IsStale(node, now, r.StaleThresholdDays) {
		days := types.DaysSince(node.Date.Modified, now)
		suffix := fmt.Sprintf(" %s stale · %dd", r.StaleGlyph, days)
		sb.WriteString(mutedStyle.Render(suffix))
	}
	sb.WriteString("\n\n")

	// --- Type badges ---
	if len(node.Types) > 0 {
		sb.WriteString(TypeBadges(node.Types, bg))
		sb.WriteString("\n\n")
	}

	// --- Kind / Stage ---
	if line, ok := r.renderKindStageLine(node, bg, mutedStyle, primaryStyle); ok {
		sb.WriteString(line)
		sb.WriteString("\n\n")
	}

	// --- Body (full markdown content) ---
	// Strip the first line of body when it duplicates the title to avoid
	// showing the same text twice. This happens when:
	//   (a) node.Title is empty and the title was derived from firstLine(body), or
	//   (b) node.Title is set and body opens with the same text (possibly as a
	//       markdown heading, e.g. "# My Title").
	body := bodyWithoutTitle(node)
	if body != "" {
		if isArchived {
			sb.WriteString(mutedStyle.Render(body))
		} else {
			sb.WriteString(r.renderMarkdown(body, primaryStyle))
		}
		sb.WriteString("\n\n")
	}

	// --- Metadata ---
	if node.Properties != nil {
		metaLines := buildMetadataLines(node, mutedStyle, primaryStyle)
		if len(metaLines) > 0 {
			for _, line := range metaLines {
				sb.WriteString(line)
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}
	}

	// --- Synced node indicator ---
	if node.Source != nil {
		sb.WriteString(renderSourceIndicator(node.Source, mutedStyle))
		sb.WriteString("\n\n")
	}

	// --- BLOCKED BY section (DL.2) ---
	if len(blockers) > 0 {
		sb.WriteString(sectionHeaderStyle.Render("BLOCKED BY"))
		sb.WriteString("\n")
		blockedGlyphStyle := lipgloss.NewStyle().Foreground(c.BudgetOver).Background(bg)
		sp := Spacer(1, bg)
		for _, blocker := range blockers {
			sb.WriteString(blockedGlyphStyle.Render(r.BlockedGlyph) + sp + mutedStyle.Render(nodeTitle(blocker)))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// --- EDGES section ---
	if len(edges) > 0 {
		sb.WriteString(sectionHeaderStyle.Render("EDGES"))
		sb.WriteString("\n")
		for _, e := range edges {
			line := r.renderEdgeLine(e, node.ID, nodesByID, now)
			sb.WriteString(line)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// --- BUDGETS section ---
	if len(budgetNodes) > 0 {
		sb.WriteString(sectionHeaderStyle.Render("BUDGETS"))
		sb.WriteString("\n")
		for _, bn := range budgetNodes {
			sb.WriteString(r.renderBudgetLine(bn, now))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// --- SPEND LOG section (budget nodes only) ---
	// Shows all spend entries sorted by date ascending with a cumulative running
	// total. This is full-history semantics: the total does not filter by period
	// and will not match the budget bar's period-scoped Spent figure.
	if nodeHasType(node, "budget") {
		entries := budget.SpendLog(node)
		if len(entries) > 0 {
			// Copy before sorting to avoid mutating the stored slice.
			sorted := make([]types.SpendEntry, len(entries))
			copy(sorted, entries)
			sort.SliceStable(sorted, func(i, j int) bool {
				return sorted[i].Date < sorted[j].Date
			})

			sb.WriteString(sectionHeaderStyle.Render("SPEND LOG"))
			sb.WriteString("\n")
			var running float64
			for _, e := range sorted {
				running += e.Amount
				sb.WriteString(r.renderSpendLogLine(e, running))
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// bg returns the background colour for all inner styles. Every style in
// the detail renderer must carry this background to prevent terminal default
// bleeding through at ANSI reset boundaries between styled segments.
func (r *DetailRenderer) bg() color.Color {
	if r.Colours.BgPrimary != nil {
		return r.Colours.BgPrimary
	}
	return lipgloss.NoColor{}
}

// renderMarkdown renders body text as markdown using Glamour. The Glamour
// renderer is created once and reused for subsequent calls (recreated when
// Width, dark/light mode, or the active theme changes). On error it falls
// back to plainStyle.Render(body). Trailing newlines from Glamour output are
// trimmed so the caller controls surrounding spacing.
func (r *DetailRenderer) renderMarkdown(body string, plainStyle lipgloss.Style) string {
	dark := isColourDark(r.Colours.BgPrimary)
	themeKey := colourHex(r.Colours.AccentPrimary) // changes when the theme changes

	if r.glamRenderer == nil || r.glamWidth != r.Width || r.glamDark != dark || r.glamTheme != themeKey {
		// Pick the glamour base style to match the active theme's background.
		// Glamour's default configs set Document.Margin and various BackgroundColor
		// fields that bleed through the pane; clear them so the terminal background
		// colour is always inherited from the pane rather than overridden.
		//
		// IMPORTANT: glamourStyles.DarkStyleConfig is a package-level value whose
		// *string fields are shared pointers. We copy by value here (struct
		// assignment), then assign *new* *string values via hexPtr — never mutate
		// through existing pointers or we corrupt the package global.
		style := glamourStyles.DarkStyleConfig
		if !dark {
			style = glamourStyles.LightStyleConfig
		}
		var zero uint = 0
		style.Document.Margin = &zero
		style.Document.BackgroundColor = nil
		style.Document.Color = hexPtr(r.Colours.FGPrimary)

		// Headings: H1 in primary accent, H2/H3 in secondary accent.
		// BackgroundColor left nil so the pane background shows through.
		style.H1.Color = hexPtr(r.Colours.AccentPrimary)
		style.H1.BackgroundColor = nil
		style.H2.Color = hexPtr(r.Colours.AccentSecondary)
		style.H2.BackgroundColor = nil
		style.H3.Color = hexPtr(r.Colours.AccentSecondary)
		style.H3.BackgroundColor = nil

		// Links in secondary accent.
		style.Link.Color = hexPtr(r.Colours.AccentSecondary)
		style.LinkText.Color = hexPtr(r.Colours.FGPrimary)

		// Emphasis: emph (italic) in muted; strong (bold) in primary.
		style.Emph.Color = hexPtr(r.Colours.FGMuted)
		style.Strong.Color = hexPtr(r.Colours.FGPrimary)

		// Inline code in primary accent, no background (inherits pane).
		style.Code.Color = hexPtr(r.Colours.AccentPrimary)
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
	// Glamour emits foreground escapes but no background, so interior cells
	// (blank lines, inter-block padding) inherit the terminal default and bleed
	// through. FillBackground repaints every line start and post-reset cell with
	// the pane background, matching the treatment every other detail block gets
	// via its lipgloss style.
	out = strings.TrimRight(out, "\n")
	return FillBackground(out, r.bg())
}

// isColourDark returns true when c is a dark colour (perceived luminance < 0.5).
// Falls back to true (dark) for nil or transparent colours.
func isColourDark(c color.Color) bool {
	if c == nil {
		return true
	}
	// color.Color.RGBA() returns 16-bit channels (0–65535).
	r16, g16, b16, _ := c.RGBA()
	// Relative luminance approximation (ITU-R BT.709 coefficients).
	lum := (0.2126*float64(r16) + 0.7152*float64(g16) + 0.0722*float64(b16)) / 65535.0
	return lum < 0.5
}

// colourHex converts a color.Color to a "#rrggbb" hex string for use in
// glamour's StylePrimitive.Color / BackgroundColor fields. Returns "" for nil
// or fully-transparent colours so callers can use hexPtr to skip nil fields.
func colourHex(c color.Color) string {
	if c == nil {
		return ""
	}
	// RGBA returns pre-multiplied 16-bit channels (0–65535); shift down to 8-bit.
	r16, g16, b16, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r16>>8, g16>>8, b16>>8)
}

// hexPtr converts a color.Color to the *string form required by glamour's
// StylePrimitive.Color field. Returns nil when the colour is nil or transparent
// so that glamour falls back to its own defaults rather than receiving an empty
// string (which it would interpret as "no colour").
func hexPtr(c color.Color) *string {
	s := colourHex(c)
	if s == "" {
		return nil
	}
	return &s
}

// ---- Internal helpers ----

// isArchivedNode returns true if the node's status property is "archived".
func isArchivedNode(node *types.Node) bool {
	if node.Properties == nil {
		return false
	}
	status, _ := node.Properties["status"].(string)
	return status == "archived"
}

// blockers returns the nodes currently holding a block on node (DL.2),
// mirroring types.Blockers' semantics (non-terminal or unresolvable blockers
// count; terminal blockers don't) but working from the edges/nodesByID the
// caller already gathered rather than a types.GraphIndex — renderDetail
// collects both anyway when building the EDGES section, so no extra
// dependency is threaded into DetailRenderer.
func (r *DetailRenderer) blockers(node *types.Node, edges []*types.Edge, nodesByID map[string]*types.Node) []*types.Node {
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
			// Dangling edge: no node to show, but it does still block per
			// types.EvalBlockers — omitted here since there's nothing to render.
			continue
		}
		group, resolved := types.ResolveStageGroup(r.Kinds, r.StageGroups, source)
		if !resolved || !group.IsTerminal(source.Stage) {
			blockers = append(blockers, source)
		}
	}
	return blockers
}

// nodeTitle returns the node's Title when set, falling back to the first line
// of Body. Use this for all display contexts that show a node's name.
func nodeTitle(node *types.Node) string {
	if node.Title != "" {
		return node.Title
	}
	return firstLine(node.Body)
}

// bodyWithoutTitle returns node.Body with the first line stripped when it
// would duplicate the rendered title. This covers two cases:
//   - node.Title is empty: the title was derived from firstLine(body), so drop
//     that line to avoid repeating it.
//   - node.Title is set and the body's first non-empty line is the same text
//     (possibly as a markdown heading, e.g. "# My Title"): drop that line too.
func bodyWithoutTitle(node *types.Node) string {
	body := strings.TrimRight(node.Body, "\n")
	if body == "" {
		return ""
	}

	lines := strings.SplitN(body, "\n", 2)
	firstRaw := strings.TrimSpace(lines[0])
	// Strip leading markdown heading markers for comparison.
	firstPlain := strings.TrimLeft(firstRaw, "# ")

	var titlePlain string
	if node.Title != "" {
		titlePlain = node.Title
	} else {
		titlePlain = firstPlain
	}

	if strings.EqualFold(firstPlain, titlePlain) {
		if len(lines) < 2 {
			return ""
		}
		return strings.TrimLeft(lines[1], "\n")
	}
	return body
}

// firstLine returns the first non-empty line of s.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return s
}

// renderKindStageLine builds the "◆ Task · doing" line shown immediately after type badges.
//
// Resolution order:
//  1. Registry present + node.Kind resolves → glyph (kind colour) + kind name + stage.
//  2. Kind empty/unresolved, stage present → plain muted stage string.
//  3. Both empty → returns ("", false) so the caller omits the block entirely.
//
// Every style carries Background(bg) to prevent terminal-default bleed (see CLAUDE.md rules).
func (r *DetailRenderer) renderKindStageLine(
	node *types.Node,
	bg color.Color,
	mutedStyle, primaryStyle lipgloss.Style,
) (string, bool) {
	sp := Spacer(1, bg)

	// Attempt a registry lookup when we have both a registry and a kind name.
	if r.Kinds != nil && node.Kind != "" {
		if k, ok := r.Kinds.Lookup(node.Kind); ok {
			kindStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color(k.Colour)).
				Background(bg)
			line := kindStyle.Render(k.Glyph) + sp + primaryStyle.Render(k.Name)
			if node.Stage != "" {
				line += sp + mutedStyle.Render("· "+node.Stage)
			}
			return line, true
		}
	}

	// Fallback: no registry, empty kind, or unresolved kind — show stage alone.
	if node.Stage != "" {
		return mutedStyle.Render(node.Stage), true
	}

	return "", false
}

// buildMetadataLines produces "key: value" strings for non-empty, non-nil properties.
// Keys are rendered in muted colour, values in primary colour.
// Skips internal budget fields and spend_log which have their own section.
func buildMetadataLines(node *types.Node, mutedStyle, primaryStyle lipgloss.Style) []string {
	skipped := map[string]bool{
		"spend_log": true,
		"allocated": true,
		"warn_at":   true,
		"period":    true,
		"category":  true,
	}

	var lines []string
	for k, v := range node.Properties {
		if skipped[k] {
			continue
		}
		if v == nil {
			continue
		}
		str := fmt.Sprintf("%v", v)
		if str == "" || str == "[]" || str == "map[]" {
			continue
		}
		line := mutedStyle.Render(k+": ") + primaryStyle.Render(str)
		lines = append(lines, line)
	}
	return lines
}

// renderSourceIndicator renders the sync source information in muted style.
func renderSourceIndicator(src *types.Source, mutedStyle lipgloss.Style) string {
	parts := []string{src.Type}
	if src.Repo != "" {
		parts = append(parts, src.Repo)
	}
	if src.URL != "" {
		parts = append(parts, src.URL)
	}
	return mutedStyle.Render("synced via " + strings.Join(parts, " · "))
}

// edgeGlyph returns the directional glyph and display label for an edge.
//
// Direction is from the perspective of the focal node (nodeID):
//   - outgoing edge (nodeID == edge.From): the node is the actor
//   - incoming edge (nodeID == edge.To):   the node is the recipient
func edgeGlyph(edgeType string, outgoing bool) string {
	switch edgeType {
	case string(types.EdgeBlocks):
		if outgoing {
			return "→" // this node blocks something
		}
		return "←" // something is blocking this node
	case string(types.EdgeParent):
		return "→"
	case string(types.EdgeWaitingOn):
		return "⊘"
	case string(types.EdgeRelated):
		return "◇"
	case "depends_on":
		return "→"
	default:
		if outgoing {
			return "→"
		}
		return "←"
	}
}

// renderEdgeLine produces a single formatted edge line for the EDGES section.
func (r *DetailRenderer) renderEdgeLine(
	edge *types.Edge,
	nodeID string,
	nodesByID map[string]*types.Node,
	now time.Time,
) string {
	c := r.Colours
	bg := r.bg()
	outgoing := edge.From == nodeID

	// Resolve the other end of the edge.
	otherID := edge.To
	if !outgoing {
		otherID = edge.From
	}
	otherLabel := otherID
	var otherTypes []string
	if other, ok := nodesByID[otherID]; ok {
		otherLabel = nodeTitle(other)
		otherTypes = other.Types
	}

	glyph := edgeGlyph(edge.Type, outgoing)
	glyphStyle := lipgloss.NewStyle().Foreground(c.FGMuted).Background(bg)
	labelStyle := lipgloss.NewStyle().Foreground(c.FGPrimary).Background(bg)
	typeStyle := lipgloss.NewStyle().Foreground(c.FGMuted).Background(bg)

	sp := Spacer(1, bg)
	line := glyphStyle.Render(glyph) + sp +
		typeStyle.Render(edge.Type+":") + sp

	if len(otherTypes) > 0 {
		line += typeStyle.Render("["+strings.Join(otherTypes, ", ")+"]") + sp
	}

	line += labelStyle.Render(otherLabel)

	// Append age suffix for waiting_on edges.
	if edge.Type == string(types.EdgeWaitingOn) {
		age := now.Sub(edge.Date.Created)
		days := int(age.Hours() / 24)
		suffix := fmt.Sprintf(" · %dd", days)
		ageColour := ageColourForDays(days, c)
		line += lipgloss.NewStyle().Foreground(ageColour).Background(bg).Render(suffix)
	}

	return line
}

// ageColourForDays returns the appropriate colour based on edge age.
//
//   - 0–7 days:  muted
//   - 8–14 days: overflow warn colour
//   - 15+ days:  overflow critical colour
func ageColourForDays(days int, c Colours) color.Color {
	switch {
	case days <= 7:
		return c.FGMuted
	case days <= 14:
		return c.OverflowWarn
	default:
		return c.OverflowCrit
	}
}

// renderBudgetLine produces a compact one-line budget summary with a progress bar.
func (r *DetailRenderer) renderBudgetLine(node *types.Node, now time.Time) string {
	c := r.Colours
	bg := r.bg()
	summary := budget.Compute(node, now)

	var statusColour color.Color
	var statusGlyph string
	switch summary.Status {
	case budget.BudgetOK:
		statusColour = c.BudgetOK
		statusGlyph = "●"
	case budget.BudgetCaution:
		statusColour = c.BudgetCaution
		statusGlyph = "◆"
	case budget.BudgetOver:
		statusColour = c.BudgetOver
		statusGlyph = "▲"
	}

	categoryLabel := ""
	if node.Properties != nil {
		if cat, ok := node.Properties["category"].(string); ok {
			categoryLabel = cat
		}
	}
	if categoryLabel == "" {
		categoryLabel = nodeTitle(node)
	}

	// Build a compact progress bar (10 characters wide).
	bar := buildProgressBar(summary.Spent, summary.Allocated, 10)

	statusStyle := lipgloss.NewStyle().Foreground(statusColour).Background(bg)
	mutedStyle := lipgloss.NewStyle().Foreground(c.FGMuted).Background(bg)
	primaryStyle := lipgloss.NewStyle().Foreground(c.FGPrimary).Background(bg)

	sp := Spacer(1, bg)
	return statusStyle.Render(statusGlyph) + sp +
		primaryStyle.Render(categoryLabel) + sp +
		mutedStyle.Render(bar) + sp +
		primaryStyle.Render(fmt.Sprintf("£%.2f", summary.Spent)) +
		mutedStyle.Render(fmt.Sprintf("/£%.2f", summary.Allocated))
}

// nodeHasType reports whether node.Types contains the given type string.
func nodeHasType(node *types.Node, typeName string) bool {
	for _, t := range node.Types {
		if t == typeName {
			return true
		}
	}
	return false
}

// renderSpendLogLine produces a single spend-log entry line:
//
//	date  £amount  note  £running-total
//
// The running total is the cumulative sum of all entries up to and including
// this one, sorted by date. Every style carries Background(bg) to prevent
// terminal background bleed at ANSI reset boundaries.
func (r *DetailRenderer) renderSpendLogLine(e types.SpendEntry, running float64) string {
	c := r.Colours
	bg := r.bg()
	mutedStyle := lipgloss.NewStyle().Foreground(c.FGMuted).Background(bg)
	primaryStyle := lipgloss.NewStyle().Foreground(c.FGPrimary).Background(bg)
	sp := Spacer(1, bg)

	line := mutedStyle.Render(e.Date) +
		sp + primaryStyle.Render(fmt.Sprintf("£%.2f", e.Amount))
	if e.Note != "" {
		line += sp + mutedStyle.Render(e.Note)
	}
	line += sp + mutedStyle.Render(fmt.Sprintf("(£%.2f)", running))
	return line
}

// buildProgressBar returns a fixed-width ASCII progress bar string.
// filled characters represent spent fraction; empty represents the remainder.
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
