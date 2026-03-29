package tui

import (
	"fmt"
	"image/color"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// groupHeaderItem is a non-selectable separator item rendered as a bold group
// heading in the node list. The custom delegate renders it distinctly from
// data rows, and Update() skips over it during cursor movement.
type groupHeaderItem struct {
	label string
}

// FilterValue returns "" so group headers are invisible to the bubbles/list
// filter — they are never matched and always hidden when filtering is active.
func (g groupHeaderItem) FilterValue() string { return "" }

// Title implements list.DefaultItem (used by the delegate's type switch).
func (g groupHeaderItem) Title() string { return g.label }

// Description implements list.DefaultItem.
func (g groupHeaderItem) Description() string { return "" }

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

// filterStateChangedMsg is emitted when the node list filter state transitions
// so that app.go can sync key hints in the status bar.
type filterStateChangedMsg struct{}

// groupedDelegate is a list.ItemDelegate that renders nodeListItem rows as
// data rows and groupHeaderItem rows as bold section headings. It replaces
// the standard DefaultDelegate when grouping is active.
type groupedDelegate struct {
	normalStyle   lipgloss.Style
	selectedStyle lipgloss.Style
	dimmedStyle   lipgloss.Style
	headerStyle   lipgloss.Style
}

func newGroupedDelegate(theme *ActiveTheme) groupedDelegate {
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

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.AccentPrimary()).
		Background(theme.BgPrimary()).
		Padding(0, 0, 0, 1)

	return groupedDelegate{
		normalStyle:   normalStyle,
		selectedStyle: selectedStyle,
		dimmedStyle:   dimmedStyle,
		headerStyle:   headerStyle,
	}
}

func (d groupedDelegate) Height() int  { return 1 }
func (d groupedDelegate) Spacing() int { return 0 }

func (d groupedDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d groupedDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	switch it := item.(type) {
	case groupHeaderItem:
		fmt.Fprint(w, d.headerStyle.Render(it.label))
	case nodeListItem:
		style := d.normalStyle
		if index == m.Index() {
			style = d.selectedStyle
		} else if m.FilterState() == list.FilterApplied {
			style = d.dimmedStyle
		}
		title := it.Title()
		// Truncate if wider than available space (width minus left padding).
		maxW := m.Width() - style.GetPaddingLeft() - style.GetPaddingRight()
		if maxW > 0 {
			runes := []rune(title)
			if len(runes) > maxW {
				ellipsisWidth := utf8.RuneCountInString(listColEllipsis)
				cut := maxW - ellipsisWidth
				if cut < 0 {
					cut = 0
				}
				title = string(runes[:cut]) + listColEllipsis
			}
		}
		fmt.Fprint(w, style.Render(title))
	}
}

// nodeListPane is a PaneModel that renders a QueryResult as a scrollable list
// using the charmbracelet/bubbles list component. Navigation uses arrow keys,
// pgup/pgdn, and home/end; vim-style single-char bindings are disabled.
type nodeListPane struct {
	list      list.Model
	columns   []string
	colWidths []int
	rows      []map[string]interface{}
	// groupCol is the column name used to group rows into sections (e.g.
	// "category"). Empty string means flat list with no group headers.
	groupCol string
	width    int
	height   int
	theme    *ActiveTheme
}

// newNodeListPane constructs a pane from a QueryResult, wiring the theme
// colours into the bubbles/list delegate styles.
func newNodeListPane(result types.QueryResult, theme *ActiveTheme) nodeListPane {
	cols := result.Columns
	if len(cols) == 0 {
		cols = dashboardColumns
	}

	// Detect the group column: use "category" when present.
	groupCol := detectGroupCol(cols)

	const initialWidth = 80
	const delegatePad = 1 // left padding added by the delegate style
	widths := calculateColWidths(result.Rows, cols, initialWidth-delegatePad)
	items := rowsToItems(result.Rows, cols, widths, groupCol)

	// Use groupedDelegate when grouping is active, DefaultDelegate otherwise.
	var delegate list.ItemDelegate
	if groupCol != "" {
		delegate = newGroupedDelegate(theme)
	} else {
		delegate = buildDelegate(theme)
	}
	l := list.New(items, delegate, initialWidth, 22)

	// Disable all built-in chrome — we own the surrounding frame.
	// SetShowFilter(false) keeps the title-bar area hidden; we render the
	// filter input ourselves in View() so we control its position and styling.
	l.SetShowTitle(false)
	l.SetShowFilter(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)

	// Reconfigure key bindings to match Wyrd's preferences. The canonical
	// Bubble Tea pattern is to configure the child component's KeyMap rather
	// than intercepting keys at the parent level.
	//
	// Keep arrow/pgup/pgdn/home/end; remove all single-char vim keys so they
	// can never collide with terminal protocol response bytes.
	// Filter/ClearFilter are left enabled — updateKeybindings() in bubbles/list
	// manages them dynamically based on filter state.
	l.KeyMap.CursorUp.SetKeys("up")
	l.KeyMap.CursorDown.SetKeys("down")
	l.KeyMap.PrevPage.SetKeys("left", "pgup")
	l.KeyMap.NextPage.SetKeys("right", "pgdown")
	l.KeyMap.GoToStart.SetKeys("home", "alt+shift+up")
	l.KeyMap.GoToEnd.SetKeys("end", "alt+shift+down")
	// Remove ctrl+k (conflicts with app-level FuzzyPalette) and ctrl+j (vim-adjacent).
	l.KeyMap.AcceptWhileFiltering.SetKeys("enter", "tab", "shift+tab", "up", "down")
	l.KeyMap.ShowFullHelp.SetEnabled(false)
	l.KeyMap.CloseFullHelp.SetEnabled(false)
	l.KeyMap.Quit.SetEnabled(false)
	l.KeyMap.ForceQuit.SetEnabled(false)

	// Style the filter text input with theme colours.
	filterStyles := textinput.DefaultStyles(true)
	filterStyles.Focused.Prompt = lipgloss.NewStyle().
		Foreground(theme.AccentPrimary()).
		Background(theme.BgPrimary())
	filterStyles.Focused.Text = lipgloss.NewStyle().
		Foreground(theme.FgPrimary()).
		Background(theme.BgPrimary())
	filterStyles.Cursor.Color = theme.AccentPrimary()
	l.FilterInput.SetStyles(filterStyles)
	l.FilterInput.Prompt = "/ "

	// If the first item is a group header, advance the cursor to the first
	// data item so ctrl+o / ctrl+d work immediately without requiring a
	// manual cursor move.
	skipInitialHeaders(&l)

	return nodeListPane{
		list:      l,
		columns:   cols,
		colWidths: widths,
		rows:      result.Rows,
		groupCol:  groupCol,
		width:     initialWidth,
		height:    22,
		theme:     theme,
	}
}

// listHeight computes the number of rows available for the bubbles/list
// component. The status bar is 2 lines (separator + bar); the pane border
// adds 2 lines (top + bottom); one additional line is the column header.
func listHeight(terminalHeight int) int {
	const borderLines = 2 // rounded border top + bottom
	const statusBar = 2   // separator line + bar line
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
		p.list.SetItems(rowsToItems(p.rows, p.columns, p.colWidths, p.groupCol))
		skipInitialHeaders(&p.list)
		return p, nil

	}

	prevIndex := p.list.Index()
	prevState := p.list.FilterState()
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)

	// Skip group header items — nudge cursor to the nearest data item.
	if _, isHeader := p.list.SelectedItem().(groupHeaderItem); isHeader {
		direction := 1
		if p.list.Index() < prevIndex {
			direction = -1
		}
		items := p.list.Items()
		newIdx := p.list.Index() + direction
		for newIdx >= 0 && newIdx < len(items) {
			if _, ok := items[newIdx].(groupHeaderItem); !ok {
				break
			}
			newIdx += direction
		}
		// If we ran off the end, try the other direction.
		if newIdx < 0 || newIdx >= len(items) {
			newIdx = p.list.Index() - direction
			for newIdx >= 0 && newIdx < len(items) {
				if _, ok := items[newIdx].(groupHeaderItem); !ok {
					break
				}
				newIdx -= direction
			}
		}
		if newIdx >= 0 && newIdx < len(items) {
			p.list.Select(newIdx)
		}
	}

	// Emit a selection message when the cursor moves to a different item.
	if p.list.Index() != prevIndex {
		if id := p.SelectedNodeID(); id != "" {
			selCmd := func() tea.Msg { return nodeSelectedMsg{nodeID: id} }
			cmd = tea.Batch(cmd, selCmd)
		}
	}

	// Notify app when filter state transitions so key hints can be synced.
	if p.list.FilterState() != prevState {
		cmd = tea.Batch(cmd, func() tea.Msg { return filterStateChangedMsg{} })
	}

	return p, cmd
}

// View renders the column header (or filter input while filtering) followed by
// the bubbles/list content. The combined output is padded via PadLines so every
// line reaches the pane edge with the correct background colour, preventing
// terminal bleed.
func (p nodeListPane) View() string {
	var bg color.Color
	var fg color.Color
	if p.theme != nil {
		bg = p.theme.BgPrimary()
		fg = p.theme.FgPrimary()
	}

	var header string
	if p.list.FilterState() == list.Filtering {
		// Replace the column header with the themed filter input row.
		filterStyle := lipgloss.NewStyle().
			Background(bg).
			Foreground(fg).
			Padding(0, 0, 0, 1)
		header = filterStyle.Render(p.list.FilterInput.View())
	} else {
		header = renderListHeader(p.columns, p.colWidths, fg, bg)
	}

	content := p.list.View()
	raw := lipgloss.JoinVertical(lipgloss.Left, header, content)
	return PadLines(raw, p.width, bg)
}

// KeyBindings advertises the navigation keys this pane handles.
// Returns context-sensitive hints based on the current filter state.
func (p nodeListPane) KeyBindings() []KeyBinding {
	switch p.list.FilterState() {
	case list.Filtering:
		return []KeyBinding{
			{Key: "esc", Description: "Cancel filter"},
			{Key: "enter", Description: "Apply filter"},
		}
	case list.FilterApplied:
		return []KeyBinding{
			{Key: "↓/↑", Description: "Navigate"},
			{Key: "←/→", Description: "Page"},
			{Key: "home/end", Description: "Start/end"},
			{Key: "alt+shift+↑/↓", Description: "Jump top/bottom"},
			{Key: "esc", Description: "Clear filter"},
		}
	default:
		return []KeyBinding{
			{Key: "↓/↑", Description: "Navigate"},
			{Key: "←/→", Description: "Page"},
			{Key: "home/end", Description: "Start/end"},
			{Key: "alt+shift+↑/↓", Description: "Jump top/bottom"},
			{Key: "/", Description: "Filter"},
		}
	}
}

// IsFiltering reports whether the list is actively in filter-input mode.
func (p nodeListPane) IsFiltering() bool {
	return p.list.FilterState() == list.Filtering
}

// HandleFocusLost is a no-op for the node list pane.
func (p nodeListPane) HandleFocusLost() tea.Cmd { return nil }

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

// skipInitialHeaders advances the list cursor past any leading groupHeaderItem
// entries so that the first selected item is always a data row. This is called
// after initial construction and after SetItems to ensure ctrl+o / ctrl+d work
// immediately without requiring a manual cursor move.
func skipInitialHeaders(l *list.Model) {
	items := l.Items()
	for i, item := range items {
		if _, ok := item.(groupHeaderItem); !ok {
			if i != l.Index() {
				l.Select(i)
			}
			return
		}
	}
	// All items are headers (or list is empty) — leave cursor as-is.
}

// detectGroupCol returns the first column in cols that should be used for
// grouping. Currently "category" is the only supported group column.
func detectGroupCol(cols []string) string {
	for _, c := range cols {
		if c == "category" {
			return "category"
		}
	}
	return ""
}

// groupLabel converts a raw category value into a display heading.
// Known values are mapped to capitalised plural labels; unknowns are
// capitalised with a trailing "s".
var groupLabelMap = map[string]string{
	"task":    "Tasks",
	"note":    "Notes",
	"journal": "Journals",
}

func toGroupLabel(raw string) string {
	if label, ok := groupLabelMap[strings.ToLower(raw)]; ok {
		return label
	}
	if len(raw) == 0 {
		return raw
	}
	return strings.ToUpper(raw[:1]) + raw[1:] + "s"
}

// rowsToItems converts QueryResult rows into bubbles/list items.
// When groupCol is non-empty, rows are grouped by that column and
// groupHeaderItem separators are inserted at each group boundary.
// colWidths must be the same length as the non-id, non-groupCol display columns.
func rowsToItems(rows []map[string]interface{}, cols []string, colWidths []int, groupCol string) []list.Item {
	if groupCol == "" {
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

	// Group rows by groupCol, preserving first-appearance order.
	type group struct {
		label string
		rows  []map[string]interface{}
	}
	var groups []group
	groupIndex := map[string]int{}
	for _, row := range rows {
		raw := fmt.Sprintf("%v", row[groupCol])
		if idx, ok := groupIndex[raw]; ok {
			groups[idx].rows = append(groups[idx].rows, row)
		} else {
			groupIndex[raw] = len(groups)
			groups = append(groups, group{label: toGroupLabel(raw), rows: []map[string]interface{}{row}})
		}
	}

	var items []list.Item
	for _, g := range groups {
		items = append(items, groupHeaderItem{label: g.label})
		for _, row := range g.rows {
			id, _ := row["id"].(string)
			items = append(items, nodeListItem{
				row:   row,
				title: formatRowTitle(row, cols, colWidths),
				id:    id,
			})
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
func renderListHeader(cols []string, colWidths []int, fg, bg color.Color) string {
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
