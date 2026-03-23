// Package tui provides terminal UI rendering components for Wyrd.
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/jasonwarrenuk/wyrd/internal/budget"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// Colour constants used when no theme is injected. These are conservative
// defaults that look reasonable on most dark terminals.
const (
	defaultAccentPrimary   = "#7C8CF8"
	defaultAccentSecondary = "#A78BFA"
	defaultFGPrimary       = "#E2E8F0"
	defaultFGMuted         = "#64748B"
	defaultBudgetOK        = "#34D399"
	defaultBudgetCaution   = "#FBBF24"
	defaultBudgetOver      = "#F87171"
	defaultOverflowWarn    = "#FB923C"
	defaultOverflowCrit    = "#EF4444"
)

// Colours holds the colour values used by DetailRenderer.
// All fields accept lipgloss-compatible colour strings (hex, ANSI, etc.).
type Colours struct {
	AccentPrimary   string
	AccentSecondary string
	BgPrimary       string
	FGPrimary       string
	FGMuted         string
	BudgetOK        string
	BudgetCaution   string
	BudgetOver      string
	OverflowWarn    string
	OverflowCrit    string
}

// defaultColours returns a set of sensible defaults.
func defaultColours() Colours {
	return Colours{
		AccentPrimary:   defaultAccentPrimary,
		AccentSecondary: defaultAccentSecondary,
		FGPrimary:       defaultFGPrimary,
		FGMuted:         defaultFGMuted,
		BudgetOK:        defaultBudgetOK,
		BudgetCaution:   defaultBudgetCaution,
		BudgetOver:      defaultBudgetOver,
		OverflowWarn:    defaultOverflowWarn,
		OverflowCrit:    defaultOverflowCrit,
	}
}

// DetailRenderer renders the right-hand detail pane for a selected node.
type DetailRenderer struct {
	// Width is the column width available for rendering.
	Width int

	// Colours holds the colour palette. Set via NewDetailRenderer or mutated after construction.
	Colours Colours
}

// NewDetailRenderer creates a DetailRenderer with default colours and a sensible width.
func NewDetailRenderer() *DetailRenderer {
	return &DetailRenderer{
		Width:   80,
		Colours: defaultColours(),
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

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.AccentPrimary)).
		Bold(true)

	var bg lipgloss.TerminalColor = lipgloss.NoColor{}
	if c.BgPrimary != "" {
		bg = lipgloss.Color(c.BgPrimary)
	}

	mutedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.FGMuted)).
		Background(bg)

	primaryStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.FGPrimary)).
		Background(bg)

	sectionHeaderStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.AccentSecondary)).
		Bold(true)

	isArchived := isArchivedNode(node)

	var sb strings.Builder

	// --- ARCHIVED banner ---
	if isArchived {
		archivedStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(c.BudgetOver)).
			Bold(true)
		sb.WriteString(archivedStyle.Render("ARCHIVED"))
		sb.WriteString("\n\n")
	}

	// --- Title ---
	title := nodeTitle(node)
	sb.WriteString(titleStyle.Render(title))
	sb.WriteString("\n\n")

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
			sb.WriteString(primaryStyle.Render(body))
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

	return strings.TrimRight(sb.String(), "\n")
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
	outgoing := edge.From == nodeID

	// Resolve the other end of the edge.
	otherID := edge.To
	if !outgoing {
		otherID = edge.From
	}
	otherLabel := otherID
	if other, ok := nodesByID[otherID]; ok {
		otherLabel = nodeTitle(other)
	}

	glyph := edgeGlyph(edge.Type, outgoing)
	glyphStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(c.FGMuted))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(c.FGPrimary))
	typeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(c.FGMuted))

	line := glyphStyle.Render(glyph) + " " +
		typeStyle.Render(edge.Type+":") + " " +
		labelStyle.Render(otherLabel)

	// Append age suffix for waiting_on edges.
	if edge.Type == string(types.EdgeWaitingOn) {
		age := now.Sub(edge.Created)
		days := int(age.Hours() / 24)
		suffix := fmt.Sprintf(" · %dd", days)
		ageColour := ageColourForDays(days, c)
		line += lipgloss.NewStyle().Foreground(lipgloss.Color(ageColour)).Render(suffix)
	}

	return line
}

// ageColourForDays returns the appropriate colour string based on edge age.
//
//   - 0–7 days:  muted
//   - 8–14 days: overflow warn colour
//   - 15+ days:  overflow critical colour
func ageColourForDays(days int, c Colours) string {
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
	summary := budget.Compute(node, now)

	var statusColour string
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

	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(statusColour))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(c.FGMuted))
	primaryStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(c.FGPrimary))

	return statusStyle.Render(statusGlyph) + " " +
		primaryStyle.Render(categoryLabel) + " " +
		mutedStyle.Render(bar) + " " +
		primaryStyle.Render(fmt.Sprintf("£%.2f", summary.Spent)) +
		mutedStyle.Render(fmt.Sprintf("/£%.2f", summary.Allocated))
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
