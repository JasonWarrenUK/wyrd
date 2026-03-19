package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// FocusedPane identifies which pane currently has keyboard focus.
type FocusedPane int

const (
	// FocusLeft means the left pane is active.
	FocusLeft FocusedPane = iota

	// FocusRight means the right pane is active.
	FocusRight
)

// Layout holds the dimensions and styles used to render the two-pane split.
// It is recalculated whenever the terminal window is resized.
type Layout struct {
	totalWidth    int
	totalHeight   int
	statusBarHeight int
	theme         *ActiveTheme
}

// NewLayout creates a Layout for the given terminal dimensions and theme.
func NewLayout(width, height int, theme *ActiveTheme) Layout {
	return Layout{
		totalWidth:      width,
		totalHeight:     height,
		statusBarHeight: 1,
		theme:           theme,
	}
}

// SetTheme swaps the theme after a runtime theme switch.
func (l *Layout) SetTheme(t *ActiveTheme) {
	l.theme = t
}

// Resize updates the stored terminal dimensions.
func (l *Layout) Resize(width, height int) {
	l.totalWidth = width
	l.totalHeight = height
}

// PaneHeight returns the height available for pane content (excluding the
// status bar).
func (l *Layout) PaneHeight() int {
	h := l.totalHeight - l.statusBarHeight
	if h < 1 {
		return 1
	}
	return h
}

// LeftWidth returns the width of the left pane (approximately 50%).
func (l *Layout) LeftWidth() int {
	w := l.totalWidth / 2
	if w < 10 {
		return 10
	}
	return w
}

// RightWidth returns the width of the right pane (the remaining half).
func (l *Layout) RightWidth() int {
	w := l.totalWidth - l.LeftWidth()
	if w < 10 {
		return 10
	}
	return w
}

// paneStyle builds the Lipgloss style for a pane box, highlighting it
// differently based on whether it has focus.
func (l *Layout) paneStyle(width int, focused bool) lipgloss.Style {
	borderColour := l.theme.Border()
	if focused {
		borderColour = l.theme.AccentPrimary()
	}

	return lipgloss.NewStyle().
		Width(width).
		Height(l.PaneHeight()).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColour).
		Background(l.theme.BgPrimary()).
		Foreground(l.theme.FgPrimary())
}

// Render assembles the full TUI frame from the two rendered pane strings and
// the status bar string. leftView and rightView should already be the content
// returned by PaneModel.View(); they are placed inside their respective boxes
// here.
func (l *Layout) Render(
	leftView string,
	rightView string,
	statusBarView string,
	focus FocusedPane,
) string {
	leftBox := l.paneStyle(l.LeftWidth(), focus == FocusLeft).Render(leftView)
	rightBox := l.paneStyle(l.RightWidth(), focus == FocusRight).Render(rightView)

	row := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)

	return lipgloss.JoinVertical(lipgloss.Left, row, statusBarView)
}
