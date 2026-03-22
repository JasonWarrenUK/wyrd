package tui

import (
	"fmt"
	"strings"
	"time"

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

// nodeListPane is a PaneModel that renders a QueryResult as a scrollable list
// using the charmbracelet/bubbles list component. It provides built-in j/k
// navigation, fuzzy filtering, and mouse support.
type nodeListPane struct {
	list    list.Model
	columns []string
	width   int
	height  int
}

// newNodeListPane constructs a pane from a QueryResult, wiring the theme
// colours into the bubbles/list delegate styles.
func newNodeListPane(result types.QueryResult, theme *ActiveTheme) nodeListPane {
	cols := result.Columns
	if len(cols) == 0 {
		cols = dashboardColumns
	}

	items := rowsToItems(result.Rows, cols)

	delegate := buildDelegate(theme)
	l := list.New(items, delegate, 80, 22)

	// Disable all built-in chrome — we own the surrounding frame.
	l.SetShowTitle(false)
	l.SetShowFilter(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)

	return nodeListPane{
		list:    l,
		columns: cols,
		width:   80,
		height:  22,
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

// Update handles window resize and forwards all other messages to the
// bubbles/list component, which manages its own j/k/mouse/filter state.
func (p nodeListPane) Update(msg tea.Msg) (PaneModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width = msg.Width / 2
		p.height = msg.Height
		p.list.SetSize(p.width, listHeight(msg.Height))
		return p, nil
	}

	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return p, cmd
}

// View renders the column header followed by the bubbles/list content.
func (p nodeListPane) View() string {
	header := renderListHeader(p.columns, p.width)
	content := p.list.View()
	return lipgloss.JoinVertical(lipgloss.Left, header, content)
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
// Each item's title is the formatted display string for that row.
func rowsToItems(rows []map[string]interface{}, cols []string) []list.Item {
	items := make([]list.Item, len(rows))
	for i, row := range rows {
		id, _ := row["id"].(string)
		items[i] = nodeListItem{
			row:   row,
			title: formatRowTitle(row, cols),
			id:    id,
		}
	}
	return items
}

// formatRowTitle produces the single-line display string for a row by joining
// the non-id column values with a separator.
func formatRowTitle(row map[string]interface{}, cols []string) string {
	parts := make([]string, 0, len(cols))
	for _, col := range cols {
		if col == "id" {
			continue
		}
		parts = append(parts, formatCellValue(row[col]))
	}
	return strings.Join(parts, "  ")
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

	// Restyle using theme colours.
	normalStyle := lipgloss.NewStyle().
		Foreground(theme.FgPrimary()).
		Padding(0, 0, 0, 1)

	selectedStyle := lipgloss.NewStyle().
		Background(theme.Selection()).
		Foreground(theme.AccentPrimary()).
		Bold(true).
		Padding(0, 0, 0, 1)

	d.Styles.NormalTitle = normalStyle
	d.Styles.NormalDesc = normalStyle
	d.Styles.SelectedTitle = selectedStyle
	d.Styles.SelectedDesc = selectedStyle
	d.Styles.DimmedTitle = lipgloss.NewStyle().
		Foreground(theme.FgMuted()).
		Padding(0, 0, 0, 1)
	d.Styles.DimmedDesc = d.Styles.DimmedTitle

	return d
}

// renderListHeader renders a styled column-header row matching the pane width.
func renderListHeader(cols []string, width int) string {
	// Show only non-id column names, space-separated.
	headers := make([]string, 0, len(cols))
	for _, col := range cols {
		if col == "id" {
			continue
		}
		headers = append(headers, col)
	}

	style := lipgloss.NewStyle().
		Bold(true).
		Padding(0, 0, 0, 1).
		Width(width)

	return style.Render(strings.Join(headers, "  "))
}
