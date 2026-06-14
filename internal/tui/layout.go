package tui

import (
	"charm.land/lipgloss/v2"
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
		statusBarHeight: 2, // 1 separator line + 1 bar line
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

// PaneHeight returns the height for each pane box (excluding the status bar).
// Height() in lipgloss v2 is an outer dimension — it includes the border lines —
// so no separate border subtraction is needed here.
func (l *Layout) PaneHeight() int {
	h := l.totalHeight - l.statusBarHeight
	if h < 1 {
		return 1
	}
	return h
}

// TotalWidth returns the full terminal width.
func (l *Layout) TotalWidth() int { return l.totalWidth }

// TotalHeight returns the full terminal height.
func (l *Layout) TotalHeight() int { return l.totalHeight }

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
// differently based on whether it has focus. The focused pane uses a
// wrapping-set gradient blend (AccentPrimary → AccentSecondary → AccentPrimary)
// so the gradient closes seamlessly at the top-left corner where the perimeter
// ends, per the BorderForegroundBlend docstring guidance for closed borders.
func (l *Layout) paneStyle(width int, focused bool) lipgloss.Style {
	style := lipgloss.NewStyle().
		Width(width).
		Height(l.PaneHeight()).
		MaxHeight(l.PaneHeight()).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderBackground(l.theme.BgPrimary()).
		Background(l.theme.BgPrimary()).
		Foreground(l.theme.FgPrimary())

	if focused {
		style = style.BorderForegroundBlend(
			l.theme.AccentPrimary(),
			l.theme.AccentSecondary(),
			l.theme.AccentPrimary(),
		)
	} else {
		style = style.BorderForeground(l.theme.Border())
	}

	return style
}

// logoStyle returns a fixed-height Lipgloss style for the logo pane that sits
// atop the right-hand detail column.
func (l *Layout) logoStyle(width, height int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Background(l.theme.BgPrimary()).
		Foreground(l.theme.AccentPrimary())
}

// detailPaneStyle returns the pane style for the right-hand detail column,
// sized to PaneHeight() minus the logo height so that logo + detail together
// fill the full pane height.
func (l *Layout) detailPaneStyle(width int, focused bool, logoH int) lipgloss.Style {
	h := l.PaneHeight() - logoH
	if h < 1 {
		h = 1
	}
	style := lipgloss.NewStyle().
		Width(width).
		Height(h).
		MaxHeight(h).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderBackground(l.theme.BgPrimary()).
		Background(l.theme.BgPrimary()).
		Foreground(l.theme.FgPrimary())

	if focused {
		style = style.BorderForegroundBlend(
			l.theme.AccentPrimary(),
			l.theme.AccentSecondary(),
			l.theme.AccentPrimary(),
		)
	} else {
		style = style.BorderForeground(l.theme.Border())
	}

	return style
}

// Render assembles the full TUI frame from the rendered pane strings and the
// status bar. leftView and rightView should already be the content returned by
// PaneModel.View(). logoView sits above rightView in the right column; it is
// produced by RenderLogo(RightWidth(), theme).
func (l *Layout) Render(
	leftView string,
	rightView string,
	logoView string,
	statusBarView string,
	focus FocusedPane,
) string {
	rw := l.RightWidth()
	logoH := LogoHeight(rw)

	leftBox    := l.paneStyle(l.LeftWidth(), focus == FocusLeft).Render(leftView)
	logoBox    := l.logoStyle(rw, logoH).Render(logoView)
	detailBox  := l.detailPaneStyle(rw, focus == FocusRight, logoH).Render(rightView)

	rightColumn := lipgloss.JoinVertical(lipgloss.Left, logoBox, detailBox)
	row := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightColumn)

	return lipgloss.JoinVertical(lipgloss.Left, row, statusBarView)
}
