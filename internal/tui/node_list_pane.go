package tui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// nodeListItem wraps a single row from a QueryResult so it satisfies the
// list.DefaultItem interface required by bubbles/list.
type nodeListItem struct {
	// row holds the raw column→value map from the query result.
	row map[string]interface{}

	// title is the pre-formatted single-line string shown in the list.
	title string

	// id is the node UUID carried for future selection/navigation.
	id string
}

// Title implements list.DefaultItem. Returns the pre-formatted display string.
func (i nodeListItem) Title() string { return i.title }

// Description implements list.DefaultItem. Unused — items render as single lines.
func (i nodeListItem) Description() string { return "" }

// FilterValue implements list.Item. The full title string is searchable.
func (i nodeListItem) FilterValue() string { return i.title }

// NodeID returns the UUID of the underlying node for navigation.
func (i nodeListItem) NodeID() string { return i.id }

const (
	// listColPadding is the number of spaces inserted between columns.
	listColPadding = 2
	// listColEllipsis is appended to truncated cell values.
	listColEllipsis = "…"
	// listMinColWidth is the smallest a column will be rendered before truncation.
	listMinColWidth = 4
)

// nodeListPane is a PaneModel that renders a QueryResult as a scrollable list
// using the charmbracelet/bubbles list component. It provides built-in j/k
// navigation, fuzzy filtering, and mouse support.
type nodeListPane struct {
	list      list.Model
	columns   []string
	colWidths []int
	rows      []map[string]interface{}
	width     int
	height    int
	theme     *ActiveTheme
}

// newNodeListPane constructs a pane from a QueryResult, wiring the theme
// colours into the bubbles/list delegate styles.
func newNodeListPane(result types.QueryResult, theme *ActiveTheme) nodeListPane {
	cols := result.Columns
	if len(cols) == 0 {
		cols = dashboardColumns
	}

	const initialWidth = 80
	const delegatePad = 1 // left padding added by the delegate style
	widths := calculateColWidths(result.Rows, cols, initialWidth-delegatePad)
	items := rowsToItems(result.Rows, cols, widths)

	delegate := buildDelegate(theme)
	l := list.New(items, delegate, initialWidth, 22)

	// Disable all built-in chrome — we own the surrounding frame.
	l.SetShowTitle(false)
	l.SetShowFilter(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)

	return nodeListPane{
		list:      l,
		columns:   cols,
		colWidths: widths,
		rows:      result.Rows,
		width:     initialWidth,
		height:    22,
		theme:     theme,
	}
}

// listHeight computes the number of rows available for the bubbles/list
// component. It mirrors Layout.PaneHeight() and subtracts one additional line
// for the column-header row rendered above the list.
func listHeight(terminalHeight int) int {
	const borderLines = 2 // rounded border top + bottom
	const statusBar = 1
	const headerLine = 1
	h := terminalHeight - borderLines - statusBar - headerLine
	if h < 1 {
		return 1
	}
	return h
}

// nodeSelectedMsg is emitted by nodeListPane whenever the cursor moves to a
// different item. app.go catches this and populates the right pane.
type nodeSelectedMsg struct {
	nodeID string
}

// Update handles window resize and forwards all other messages to the
// bubbles/list component, which manages its own j/k/mouse/filter state.
func (p nodeListPane) Update(msg tea.Msg) (PaneModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		const delegatePad = 1
		p.width = msg.Width / 2
		p.height = msg.Height
		p.list.SetSize(p.width, listHeight(msg.Height))
		// Recompute column widths for the new inner pane width and rebuild items.
		p.colWidths = calculateColWidths(p.rows, p.columns, p.width-delegatePad)
		p.list.SetItems(rowsToItems(p.rows, p.columns, p.colWidths))
		return p, nil
	}

	prevIndex := p.list.Index()
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)

	// Emit a selection message when the cursor moves to a different item.
	if p.list.Index() != prevIndex {
		if id := p.SelectedNodeID(); id != "" {
			selCmd := func() tea.Msg { return nodeSelectedMsg{nodeID: id} }
			cmd = tea.Batch(cmd, selCmd)
		}
	}

	return p, cmd
}

// View renders the column header followed by the bubbles/list content.
// The combined output is padded via PadLines so every line reaches the pane
// edge with the correct background colour, preventing terminal bleed.
func (p nodeListPane) View() string {
	var bg lipgloss.Color
	var fg lipgloss.Color
	if p.theme != nil {
		bg = p.theme.BgPrimary()
		fg = p.theme.FgPrimary()
	}
	header := renderListHeader(p.columns, p.colWidths, fg, bg)
	content := p.list.View()
	raw := lipgloss.JoinVertical(lipgloss.Left, header, content)
	return PadLines(raw, p.width, bg)
}

// KeyBindings advertises the navigation keys this pane handles.
func (p nodeListPane) KeyBindings() []KeyBinding {
	return []KeyBinding{
		{Key: "j / ↓", Description: "Move down"},
		{Key: "k / ↑", Description: "Move up"},
		{Key: "g / G", Description: "Jump to top / bottom"},
	}
}

// SelectedNodeID returns the UUID of the currently highlighted node, or an
// empty string when the list is empty.
func (p nodeListPane) SelectedNodeID() string {
	item := p.list.SelectedItem()
	if item == nil {
		return ""
	}
	if n, ok := item.(nodeListItem); ok {
		return n.NodeID()
	}
	return ""
}

// --- helpers -----------------------------------------------------------------

// rowsToItems converts QueryResult rows into bubbles/list items.
// Each item's title is the formatted, column-aligned display string for that row.
// colWidths must be the same length as the non-id columns in cols.
func rowsToItems(rows []map[string]interface{}, cols []string, colWidths []int) []list.Item {
	items := make([]list.Item, len(rows))
	for i, row := range rows {
		id, _ := row["id"].(string)
		items[i] = nodeListItem{
			row:   row,
			title: formatRowTitle(row, cols, colWidths),
			id:    id,
		}
	}
	return items
}

// formatRowTitle produces the single-line display string for a row.
// Each non-id cell is padded or truncated to its column width, then joined
// with listColPadding spaces so all rows align with the header.
func formatRowTitle(row map[string]interface{}, cols []string, colWidths []int) string {
	parts := make([]string, 0, len(cols))
	wi := 0
	for _, col := range cols {
		if col == "id" {
			continue
		}
		w := 0
		if wi < len(colWidths) {
			w = colWidths[wi]
		}
		wi++
		parts = append(parts, listPadOrTruncate(formatCellValue(row[col]), w))
	}
	return strings.Join(parts, strings.Repeat(" ", listColPadding))
}

// calculateColWidths computes the display width for each non-id column.
// Widths are sized to fit the widest header or cell value; if the total exceeds
// totalWidth, columns are scaled down proportionally (floor listMinColWidth).
func calculateColWidths(rows []map[string]interface{}, cols []string, totalWidth int) []int {
	// Collect non-id columns only.
	displayCols := make([]string, 0, len(cols))
	for _, c := range cols {
		if c != "id" {
			displayCols = append(displayCols, c)
		}
	}
	n := len(displayCols)
	if n == 0 {
		return nil
	}

	// Compute natural widths from header names and cell values.
	natural := make([]int, n)
	for i, col := range displayCols {
		natural[i] = utf8.RuneCountInString(col)
	}
	for _, row := range rows {
		for i, col := range displayCols {
			w := utf8.RuneCountInString(formatCellValue(row[col]))
			if w > natural[i] {
				natural[i] = w
			}
		}
	}

	// Account for padding between columns.
	padding := (n - 1) * listColPadding
	available := totalWidth - padding
	if available < n*listMinColWidth {
		available = n * listMinColWidth
	}

	totalNatural := 0
	for _, w := range natural {
		totalNatural += w
	}

	widths := make([]int, n)
	if totalNatural <= available {
		// Content fits — distribute remaining space proportionally to natural widths.
		copy(widths, natural)
		slack := available - totalNatural
		if slack > 0 && totalNatural > 0 {
			distributed := 0
			for i, w := range natural {
				extra := slack * w / totalNatural
				widths[i] = w + extra
				distributed += extra
			}
			// Give any rounding remainder to the last column.
			widths[n-1] += slack - distributed
		}
	} else {
		// Scale down proportionally, floor at listMinColWidth.
		for i, w := range natural {
			scaled := w * available / totalNatural
			if scaled < listMinColWidth {
				scaled = listMinColWidth
			}
			widths[i] = scaled
		}
	}

	return widths
}

// listPadOrTruncate pads s with spaces to exactly width runes, or truncates
// it with an ellipsis if it exceeds width.
func listPadOrTruncate(s string, width int) string {
	if width <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s + strings.Repeat(" ", width-len(runes))
	}
	ellipsisWidth := utf8.RuneCountInString(listColEllipsis)
	cutAt := width - ellipsisWidth
	if cutAt < 0 {
		cutAt = 0
	}
	return string(runes[:cutAt]) + listColEllipsis
}

// formatCellValue converts a cell value to a display string.
// time.Time values are formatted as YYYY-MM-DD. Nil values show an em dash.
func formatCellValue(v interface{}) string {
	if v == nil {
		return "—"
	}
	switch t := v.(type) {
	case time.Time:
		return t.Format("2006-01-02")
	case *time.Time:
		if t == nil {
			return "—"
		}
		return t.Format("2006-01-02")
	}
	s := fmt.Sprintf("%v", v)
	if s == "" || s == "<nil>" {
		return "—"
	}
	return s
}

// buildDelegate constructs a single-line DefaultDelegate styled to the active
// theme colours.
func buildDelegate(theme *ActiveTheme) list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.ShowDescription = false
	d.SetHeight(1)
	d.SetSpacing(0)

	// Restyle using theme colours. Background must be set on all styles so
	// that the right-hand padding spaces are coloured rather than defaulting
	// to the terminal background, which produces a black strip on the right.
	normalStyle := lipgloss.NewStyle().
		Background(theme.BgPrimary()).
		Foreground(theme.FgPrimary()).
		Padding(0, 0, 0, 1)

	selectedStyle := lipgloss.NewStyle().
		Background(theme.Selection()).
		Foreground(theme.AccentPrimary()).
		Bold(true).
		Padding(0, 0, 0, 1)

	dimmedStyle := lipgloss.NewStyle().
		Background(theme.BgPrimary()).
		Foreground(theme.FgMuted()).
		Padding(0, 0, 0, 1)

	d.Styles.NormalTitle = normalStyle
	d.Styles.NormalDesc = normalStyle
	d.Styles.SelectedTitle = selectedStyle
	d.Styles.SelectedDesc = selectedStyle
	d.Styles.DimmedTitle = dimmedStyle
	d.Styles.DimmedDesc = dimmedStyle

	return d
}

// renderListHeader renders a styled column-header row with each header name
// padded to its column width, matching the alignment of the data rows below.
// fg and bg are applied to every cell so colours match the active theme.
// Inter-column gaps use Spacer() so they carry the background colour — bare
// string separators would bleed the terminal default at ANSI reset boundaries.
func renderListHeader(cols []string, colWidths []int, fg, bg lipgloss.Color) string {
	style := lipgloss.NewStyle().Bold(true).Foreground(fg).Background(bg).Padding(0, 0, 0, 1)

	cells := make([]string, 0, len(cols))
	wi := 0
	for _, col := range cols {
		if col == "id" {
			continue
		}
		w := 0
		if wi < len(colWidths) {
			w = colWidths[wi]
		}
		wi++
		cells = append(cells, style.Render(listPadOrTruncate(col, w)))
	}

	return strings.Join(cells, Spacer(listColPadding, bg))
}
