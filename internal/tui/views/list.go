package views

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

const (
	// listMinColWidth is the smallest a column will be rendered before truncation.
	listMinColWidth = 4
	// listColPadding is the number of spaces inserted between columns.
	listColPadding = 2
	// listEllipsis is the suffix appended to truncated cell values.
	listEllipsis = "…"
)

// ListPalette holds the colours used by the list renderer.
type ListPalette struct {
	// Header is the foreground colour for column header text.
	Header string
	// Selection is the background colour applied to the selected row.
	Selection string
	// Foreground is the default text colour for data rows.
	Foreground string
	// Muted is used for empty-state messaging.
	Muted string
}

// DefaultListPalette returns the default Cairn-themed list colours.
func DefaultListPalette() ListPalette {
	return ListPalette{
		Header:     "#b98300",
		Selection:  "#2a2a3d",
		Foreground: "#e0e0e0",
		Muted:      "#8b8b8b",
	}
}

// ListRenderer renders a types.QueryResult as a tabular list.
// Columns are auto-sized to their content and the terminal width.
// The selected row is highlighted using the Selection palette colour.
type ListRenderer struct {
	// Palette controls the colour scheme.
	Palette ListPalette
	// Columns is the ordered list of column names to display.
	// When empty, all columns from the QueryResult are shown.
	Columns []string
}

// NewListRenderer returns a renderer with default palette settings.
func NewListRenderer(columns []string) *ListRenderer {
	return &ListRenderer{
		Palette: DefaultListPalette(),
		Columns: columns,
	}
}

// Render produces a styled tabular string from result.
// selectedIdx is the zero-based index of the highlighted row (-1 for none).
// width is the available terminal width in characters.
func (r *ListRenderer) Render(result types.QueryResult, selectedIdx int, width int) string {
	cols := r.resolveColumns(result)

	if len(result.Rows) == 0 {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color(r.Palette.Muted)).
			Render("No results.")
	}

	colWidths := r.calculateColumnWidths(result, cols, width)

	var sb strings.Builder

	// Render header row.
	sb.WriteString(r.renderHeaderRow(cols, colWidths))
	sb.WriteRune('\n')

	// Render data rows.
	for i, row := range result.Rows {
		line := r.renderDataRow(row, cols, colWidths)
		if i == selectedIdx {
			line = lipgloss.NewStyle().
				Background(lipgloss.Color(r.Palette.Selection)).
				Render(line)
		}
		sb.WriteString(line)
		if i < len(result.Rows)-1 {
			sb.WriteRune('\n')
		}
	}

	return sb.String()
}

// resolveColumns returns the ordered column list to render.
// Uses r.Columns when set, otherwise falls back to result.Columns.
func (r *ListRenderer) resolveColumns(result types.QueryResult) []string {
	if len(r.Columns) > 0 {
		return r.Columns
	}
	return result.Columns
}

// calculateColumnWidths distributes the available width evenly across columns,
// taking into account header and content lengths. Each column is at least
// listMinColWidth wide.
func (r *ListRenderer) calculateColumnWidths(result types.QueryResult, cols []string, totalWidth int) []int {
	n := len(cols)
	if n == 0 {
		return nil
	}

	// Calculate natural widths from header and content.
	natural := make([]int, n)
	for i, col := range cols {
		natural[i] = utf8.RuneCountInString(col)
	}
	for _, row := range result.Rows {
		for i, col := range cols {
			val := formatCellValue(row[col])
			w := utf8.RuneCountInString(val)
			if w > natural[i] {
				natural[i] = w
			}
		}
	}

	// Compute total padding overhead.
	padding := (n - 1) * listColPadding
	available := totalWidth - padding
	if available < n*listMinColWidth {
		available = n * listMinColWidth
	}

	// Distribute available width proportionally, respecting minimums.
	widths := make([]int, n)
	totalNatural := 0
	for _, w := range natural {
		totalNatural += w
	}

	if totalNatural <= available {
		// Everything fits naturally.
		copy(widths, natural)
	} else {
		// Scale down proportionally, floor at listMinColWidth.
		remaining := available
		for i, w := range natural {
			scaled := w * available / totalNatural
			if scaled < listMinColWidth {
				scaled = listMinColWidth
			}
			widths[i] = scaled
			remaining -= scaled
		}
		_ = remaining
	}

	return widths
}

// renderHeaderRow produces the styled column-header line.
func (r *ListRenderer) renderHeaderRow(cols []string, widths []int) string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(r.Palette.Header)).Bold(true)
	cells := make([]string, len(cols))
	for i, col := range cols {
		cells[i] = style.Render(padOrTruncate(col, widths[i]))
	}
	return strings.Join(cells, strings.Repeat(" ", listColPadding))
}

// renderDataRow produces a single un-highlighted data row string.
func (r *ListRenderer) renderDataRow(row map[string]interface{}, cols []string, widths []int) string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(r.Palette.Foreground))
	cells := make([]string, len(cols))
	for i, col := range cols {
		val := formatCellValue(row[col])
		cells[i] = style.Render(padOrTruncate(val, widths[i]))
	}
	return strings.Join(cells, strings.Repeat(" ", listColPadding))
}

// padOrTruncate pads a string with spaces to width, or truncates it with an
// ellipsis if it exceeds width. The result is always exactly width runes wide.
func padOrTruncate(s string, width int) string {
	runes := []rune(s)
	if len(runes) <= width {
		return s + strings.Repeat(" ", width-len(runes))
	}
	// Truncate, leaving room for the ellipsis.
	ellipsisWidth := utf8.RuneCountInString(listEllipsis)
	cutAt := width - ellipsisWidth
	if cutAt < 0 {
		cutAt = 0
	}
	return string(runes[:cutAt]) + listEllipsis
}

// formatCellValue converts an interface{} cell value to a display string.
func formatCellValue(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}
