package views

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

const (
	// budgetBarWidth is the total number of filled/empty segments in a progress bar.
	budgetBarWidth = 20
	// budgetFilled is the character used for the spent portion of the bar.
	budgetFilled = "█"
	// budgetEmpty is the character used for the remaining portion of the bar.
	budgetEmpty = "░"
)

// BudgetStatus enumerates the three envelope health states.
type BudgetStatus int

const (
	// BudgetOK means spend is below the warn threshold.
	BudgetOK BudgetStatus = iota
	// BudgetCaution means spend has crossed the warn threshold.
	BudgetCaution
	// BudgetOver means spend equals or exceeds the allocated amount.
	BudgetOver
)

// BudgetGlyphs holds the status indicator characters for budget envelopes.
type BudgetGlyphs struct {
	// OK is shown when spend is within safe bounds.
	OK string
	// Caution is shown when spend is above the warn threshold.
	Caution string
	// Over is shown when the envelope is exhausted.
	Over string
}

// DefaultBudgetGlyphs returns the canonical budget status glyphs.
func DefaultBudgetGlyphs() BudgetGlyphs {
	return BudgetGlyphs{
		OK:      "●",
		Caution: "◐",
		Over:    "○",
	}
}

// BudgetPalette holds the colours used by the budget renderer.
type BudgetPalette struct {
	// OK is the colour for envelopes with safe spend levels.
	OK string
	// Caution is the colour for envelopes approaching their limit.
	Caution string
	// Over is the colour for exhausted envelopes.
	Over string
	// Label is the colour for the envelope name.
	Label string
	// Amount is the colour for the spend/allocated text.
	Amount string
	// Muted is used for empty-state messaging.
	Muted string
}

// DefaultBudgetPalette returns the default Cairn-themed budget colours.
func DefaultBudgetPalette() BudgetPalette {
	return BudgetPalette{
		OK:      "#3ddc84",
		Caution: "#b98300",
		Over:    "#d57300",
		Label:   "#e0e0e0",
		Amount:  "#8b8b8b",
		Muted:   "#8b8b8b",
	}
}

// BudgetRenderer renders budget envelope nodes as styled progress bars.
// Each node must carry the properties: allocated (float64), warn_at (float64
// fraction), spend_log (array of {date, amount, note}).
type BudgetRenderer struct {
	// Palette controls the colour scheme.
	Palette BudgetPalette
	// Glyphs holds the status indicator characters.
	Glyphs BudgetGlyphs
}

// NewBudgetRenderer returns a renderer with default palette and glyphs.
func NewBudgetRenderer() *BudgetRenderer {
	return &BudgetRenderer{
		Palette: DefaultBudgetPalette(),
		Glyphs:  DefaultBudgetGlyphs(),
	}
}

// budgetEnvelope holds resolved values for a single budget node.
type budgetEnvelope struct {
	name      string
	allocated float64
	warnAt    float64
	spend     float64
	status    BudgetStatus
}

// Render produces the styled budget envelope list from budgetNodes.
// width is the available terminal width in characters.
func (r *BudgetRenderer) Render(budgetNodes []*types.Node, width int) string {
	if len(budgetNodes) == 0 {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color(r.Palette.Muted)).
			Render("No budget envelopes.")
	}

	envelopes := make([]budgetEnvelope, 0, len(budgetNodes))
	for _, node := range budgetNodes {
		env := r.resolveEnvelope(node)
		envelopes = append(envelopes, env)
	}

	// Find longest name for alignment.
	maxName := 0
	for _, env := range envelopes {
		if len([]rune(env.name)) > maxName {
			maxName = len([]rune(env.name))
		}
	}

	var sb strings.Builder
	for i, env := range envelopes {
		sb.WriteString(r.renderEnvelope(env, maxName))
		if i < len(envelopes)-1 {
			sb.WriteRune('\n')
		}
	}

	return sb.String()
}

// resolveEnvelope extracts and computes the fields needed for rendering.
func (r *BudgetRenderer) resolveEnvelope(node *types.Node) budgetEnvelope {
	name := node.Body
	if node.Title != "" {
		name = node.Title
	}
	env := budgetEnvelope{
		name:   name,
		warnAt: 0.8, // sensible default if not specified
	}

	if node.Properties == nil {
		return env
	}

	if v, ok := node.Properties["allocated"]; ok {
		env.allocated = toFloat64(v)
	}
	if v, ok := node.Properties["warn_at"]; ok {
		env.warnAt = toFloat64(v)
	}

	// Sum up spend_log entries.
	if v, ok := node.Properties["spend_log"]; ok {
		env.spend = sumSpendLog(v)
	}

	// Determine status.
	env.status = computeBudgetStatus(env.spend, env.allocated, env.warnAt)

	return env
}

// computeBudgetStatus returns the budget health state given spend thresholds.
func computeBudgetStatus(spend, allocated, warnAt float64) BudgetStatus {
	if allocated <= 0 {
		return BudgetOK
	}
	if spend >= allocated {
		return BudgetOver
	}
	if spend >= allocated*warnAt {
		return BudgetCaution
	}
	return BudgetOK
}

// renderEnvelope renders a single envelope as a labelled progress-bar line.
func (r *BudgetRenderer) renderEnvelope(env budgetEnvelope, maxNameWidth int) string {
	statusColour, glyph := r.statusStyle(env.status)

	glyphStr := lipgloss.NewStyle().
		Foreground(lipgloss.Color(statusColour)).
		Render(glyph)

	// Pad the name to maxNameWidth for alignment.
	nameRunes := []rune(env.name)
	namePadded := string(nameRunes)
	if len(nameRunes) < maxNameWidth {
		namePadded = env.name + strings.Repeat(" ", maxNameWidth-len(nameRunes))
	}
	labelStr := lipgloss.NewStyle().
		Foreground(lipgloss.Color(r.Palette.Label)).
		Render(namePadded)

	// Build progress bar.
	bar := r.buildBar(env.spend, env.allocated, statusColour)

	// Amount label.
	var amountStr string
	if env.allocated > 0 {
		amountStr = fmt.Sprintf("£%.2f/£%.2f", env.spend, env.allocated)
	} else {
		amountStr = fmt.Sprintf("£%.2f", env.spend)
	}
	amountStyled := lipgloss.NewStyle().
		Foreground(lipgloss.Color(r.Palette.Amount)).
		Render(amountStr)

	return fmt.Sprintf("%s  %s  %s  %s", glyphStr, labelStr, bar, amountStyled)
}

// buildBar constructs the filled/empty progress bar string.
func (r *BudgetRenderer) buildBar(spend, allocated float64, colour string) string {
	filledSegments := 0
	if allocated > 0 {
		ratio := spend / allocated
		if ratio > 1 {
			ratio = 1
		}
		filledSegments = int(ratio * float64(budgetBarWidth))
	}
	emptySegments := budgetBarWidth - filledSegments

	filled := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colour)).
		Render(strings.Repeat(budgetFilled, filledSegments))

	empty := lipgloss.NewStyle().
		Foreground(lipgloss.Color(r.Palette.Amount)).
		Render(strings.Repeat(budgetEmpty, emptySegments))

	return filled + empty
}

// statusStyle returns the hex colour and glyph character for a budget status.
func (r *BudgetRenderer) statusStyle(status BudgetStatus) (colour, glyph string) {
	switch status {
	case BudgetCaution:
		return r.Palette.Caution, r.Glyphs.Caution
	case BudgetOver:
		return r.Palette.Over, r.Glyphs.Over
	default:
		return r.Palette.OK, r.Glyphs.OK
	}
}

// sumSpendLog totals the amount fields from a spend_log property value.
// The value may be []interface{} (from JSON decode) or []types.SpendEntry.
func sumSpendLog(v interface{}) float64 {
	total := 0.0
	switch log := v.(type) {
	case []types.SpendEntry:
		for _, entry := range log {
			total += entry.Amount
		}
	case []interface{}:
		for _, item := range log {
			switch entry := item.(type) {
			case map[string]interface{}:
				if amt, ok := entry["amount"]; ok {
					total += toFloat64(amt)
				}
			case types.SpendEntry:
				total += entry.Amount
			}
		}
	default:
		// Attempt JSON round-trip for unknown types.
		data, err := json.Marshal(v)
		if err != nil {
			return 0
		}
		var entries []types.SpendEntry
		if err := json.Unmarshal(data, &entries); err != nil {
			return 0
		}
		for _, entry := range entries {
			total += entry.Amount
		}
	}
	return total
}

// toFloat64 attempts a best-effort conversion of v to float64.
func toFloat64(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case int32:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	}
	return 0
}
