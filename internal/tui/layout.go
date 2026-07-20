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

// FocusAnimState carries the in-flight VP.6 spring transition into Render so
// the border colour and pane width can be interpolated rather than snapped.
// A nil *FocusAnimState means no transition is in flight; Render then falls
// back to the pre-VP.6 hard-snap behaviour — the correct value for callers
// with reduce_motion enabled, or on any frame between transitions.
type FocusAnimState struct {
	// From and To are the panes losing and gaining focus.
	From, To FocusedPane

	// Progress is the clamped [0,1] spring position: 0 is fully at From,
	// 1 is fully at To.
	Progress float64
}

// Layout holds the dimensions and styles used to render the two-pane split.
// It is recalculated whenever the terminal window is resized.
type Layout struct {
	totalWidth      int
	totalHeight     int
	statusBarHeight int
	theme           *ActiveTheme
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

// focusFraction returns how "focused" the given pane is right now, as a
// [0,1] value used to interpolate its border colour and width nudge. Outside
// of a transition this collapses to the plain binary 0/1 the pre-VP.6 code
// used. During a transition, the pane gaining focus (anim.to) animates
// 0→anim.progress and the pane losing it (anim.from) animates 1→(1-progress);
// any other pane (impossible with only two panes, but defensive) stays put.
func focusFraction(pane FocusedPane, focus FocusedPane, anim *FocusAnimState) float64 {
	if anim == nil {
		if pane == focus {
			return 1
		}
		return 0
	}
	switch pane {
	case anim.To:
		return anim.Progress
	case anim.From:
		return 1 - anim.Progress
	default:
		if pane == focus {
			return 1
		}
		return 0
	}
}

// paneBaseStyle builds the Lipgloss style shared by both of a pane's rest
// states (flat unfocused, gradient focused) — every property except the
// border's foreground colour, which callers set via BorderForeground /
// BorderForegroundBlend before rendering. Splitting this out lets
// renderPaneBorder build the two endpoint strings paneStyle's cross-dissolve
// needs without duplicating the box geometry twice.
func (l *Layout) paneBaseStyle(width, height int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		MaxHeight(height).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderBackground(l.theme.BgPrimary()).
		Background(l.theme.BgPrimary()).
		Foreground(l.theme.FgPrimary())
}

// renderPaneBorder renders content in a pane box of the given dimensions,
// blending between the flat unfocused border and the wrapping-set gradient
// border (AccentPrimary → AccentSecondary → AccentPrimary, closing
// seamlessly at the top-left corner per the BorderForegroundBlend docstring
// guidance for closed borders) at fraction frac.
//
// At frac<=0/>=1 this is a single lipgloss render, identical to the pre-VP.6
// hard-snap output. For an in-between frac it renders BOTH rest states at
// the same dimensions and cross-dissolves them (see crossDissolveRendered)
// rather than interpolating the gradient's 3 colour stops directly — see
// that function's doc for why: stop-interpolation let a theme whose border
// colour sits much closer to one accent than the other (e.g. peat) produce
// a colour patch that visibly detached from the border and travelled around
// it, since the two accent stops moved at very different perceptual speeds.
func (l *Layout) renderPaneBorder(content string, width, height int, frac float64) string {
	base := l.paneBaseStyle(width, height)
	border := l.theme.Border()

	if frac <= 0 {
		return base.BorderForeground(border).Render(content)
	}
	focused := base.BorderForegroundBlend(
		l.theme.AccentPrimary(),
		l.theme.AccentSecondary(),
		l.theme.AccentPrimary(),
	).Render(content)
	if frac >= 1 {
		return focused
	}
	unfocused := base.BorderForeground(border).Render(content)
	return crossDissolveRendered(unfocused, focused, frac)
}

// logoThickBorder is lipgloss's stock thick-weight border: heavy edge/line
// characters paired with square heavy corners, so the box reads as one
// consistent weight all the way round rather than mixing a heavy edge with
// a thin single-stroke curve at the corners.
var logoThickBorder = lipgloss.Border{
	Top:          "━",
	Bottom:       "━",
	Left:         "┃",
	Right:        "┃",
	TopLeft:      "┏",
	TopRight:     "┓",
	BottomLeft:   "┗",
	BottomRight:  "┛",
	MiddleLeft:   "┣",
	MiddleRight:  "┫",
	Middle:       "╋",
	MiddleTop:    "┳",
	MiddleBottom: "┻",
}

// logoStyle returns the bordered Lipgloss style for the logo box that sits atop
// the right-hand detail column. height is the OUTER box height (LogoHeight),
// which includes the 2 border rows; the wordmark content is centred vertically
// and horizontally within the box. The border is always a thick gradient
// blend (AccentPrimary → AccentSecondary → AccentPrimary) — the same colour
// treatment as a focused pane's border, weight matched to the logo's
// permanent-focus status, with square corners matching the thick edge weight
// — since the logo is permanently on show rather than toggling
// focused/unfocused like the list and detail panes.
func (l *Layout) logoStyle(width, height int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		MaxHeight(height).
		Align(lipgloss.Center, lipgloss.Center).
		BorderStyle(logoThickBorder).
		BorderForegroundBlend(
			l.theme.AccentPrimary(),
			l.theme.AccentSecondary(),
			l.theme.AccentPrimary(),
		).
		BorderBackground(l.theme.BgPrimary()).
		Background(l.theme.BgPrimary()).
		Foreground(l.theme.AccentPrimary())
}

// detailPaneHeight returns the height for the right-hand detail column,
// sized to PaneHeight() minus the logo height so that logo + detail
// together fill the full pane height.
func (l *Layout) detailPaneHeight(logoH int) int {
	h := l.PaneHeight() - logoH
	if h < 1 {
		h = 1
	}
	return h
}

// Render assembles the full TUI frame from the rendered pane strings and the
// status bar. leftView and rightView should already be the content returned by
// PaneModel.View(). logoView sits above rightView in the right column; it is
// produced by RenderLogo(RightWidth(), theme).
//
// anim carries the VP.6 focus-transition spring state (nil = no transition
// in flight, renders identically to the pre-VP.6 hard snap). Only the
// border colour animates — an earlier version also nudged the focused
// pane's width by 1 column, but that grow/shrink read as visually
// distracting rather than as focus affordance, so pane widths are always
// exactly LeftWidth()/RightWidth() regardless of focus or transition state.
func (l *Layout) Render(
	leftView string,
	rightView string,
	logoView string,
	statusBarView string,
	focus FocusedPane,
	anim *FocusAnimState,
) string {
	rw := l.RightWidth()
	lw := l.LeftWidth()
	logoH := LogoHeight(rw)

	leftFrac := focusFraction(FocusLeft, focus, anim)
	rightFrac := focusFraction(FocusRight, focus, anim)

	leftBox := l.renderPaneBorder(leftView, lw, l.PaneHeight(), leftFrac)
	logoBox := l.logoStyle(rw, logoH).Render(logoView)
	detailBox := l.renderPaneBorder(rightView, rw, l.detailPaneHeight(logoH), rightFrac)

	rightColumn := lipgloss.JoinVertical(lipgloss.Left, logoBox, detailBox)
	row := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightColumn)

	return lipgloss.JoinVertical(lipgloss.Left, row, statusBarView)
}
