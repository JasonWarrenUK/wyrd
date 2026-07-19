package tui

import (
	"image/color"

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

// paneStyle builds the Lipgloss style for a pane box, blending its border
// colour and width between the unfocused and focused rest states according
// to frac (see focusFraction). At frac==0 this renders identically to the
// old flat-border unfocused style; at frac==1, identically to the old
// wrapping-set gradient blend (AccentPrimary → AccentSecondary →
// AccentPrimary) used for the focused pane, which closes seamlessly at the
// top-left corner per the BorderForegroundBlend docstring guidance for
// closed borders. widthNudge is the extra (or fewer, if negative) columns to
// apply to the outer box width at this frac — see Render for why this must
// never feed back into LeftWidth/RightWidth.
func (l *Layout) paneStyle(width int, frac float64, widthNudge int) lipgloss.Style {
	style := lipgloss.NewStyle().
		Width(width + widthNudge).
		Height(l.PaneHeight()).
		MaxHeight(l.PaneHeight()).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderBackground(l.theme.BgPrimary()).
		Background(l.theme.BgPrimary()).
		Foreground(l.theme.FgPrimary())

	border := l.theme.Border()
	a, b, c := blendStopColours(border, l.theme.AccentPrimary(), l.theme.AccentSecondary(), frac)
	if frac <= 0 {
		style = style.BorderForeground(border)
	} else {
		style = style.BorderForegroundBlend(a, b, c)
	}

	return style
}

// blendStopColours interpolates the three BorderForegroundBlend stops
// (accent1 → accent2 → accent1, wrapping) from the flat unfocused border
// colour toward that gradient at fraction t. At t==0 all three stops equal
// `border` (visually a flat border); at t==1 they reproduce the original
// AccentPrimary → AccentSecondary → AccentPrimary gradient exactly.
func blendStopColours(border, accent1, accent2 color.Color, t float64) (color.Color, color.Color, color.Color) {
	a := lerpColour(border, accent1, t)
	b := lerpColour(border, accent2, t)
	return a, b, a
}

// logoStyle returns the bordered Lipgloss style for the logo box that sits atop
// the right-hand detail column. height is the OUTER box height (LogoHeight),
// which includes the 2 border rows; the wordmark content is centred vertically
// and horizontally within the box. The border is always a thick gradient blend
// (AccentPrimary → AccentSecondary → AccentPrimary) — the same treatment as a
// focused pane's border — since the logo is permanently on show rather than
// toggling focused/unfocused like the list and detail panes.
func (l *Layout) logoStyle(width, height int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		MaxHeight(height).
		Align(lipgloss.Center, lipgloss.Center).
		BorderStyle(lipgloss.ThickBorder()).
		BorderForegroundBlend(
			l.theme.AccentPrimary(),
			l.theme.AccentSecondary(),
			l.theme.AccentPrimary(),
		).
		BorderBackground(l.theme.BgPrimary()).
		Background(l.theme.BgPrimary()).
		Foreground(l.theme.AccentPrimary())
}

// detailPaneStyle returns the pane style for the right-hand detail column,
// sized to PaneHeight() minus the logo height so that logo + detail together
// fill the full pane height. frac and widthNudge behave as in paneStyle.
func (l *Layout) detailPaneStyle(width int, frac float64, widthNudge int, logoH int) lipgloss.Style {
	h := l.PaneHeight() - logoH
	if h < 1 {
		h = 1
	}
	style := lipgloss.NewStyle().
		Width(width + widthNudge).
		Height(h).
		MaxHeight(h).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderBackground(l.theme.BgPrimary()).
		Background(l.theme.BgPrimary()).
		Foreground(l.theme.FgPrimary())

	border := l.theme.Border()
	a, b, c := blendStopColours(border, l.theme.AccentPrimary(), l.theme.AccentSecondary(), frac)
	if frac <= 0 {
		style = style.BorderForeground(border)
	} else {
		style = style.BorderForegroundBlend(a, b, c)
	}

	return style
}

// Render assembles the full TUI frame from the rendered pane strings and the
// status bar. leftView and rightView should already be the content returned by
// PaneModel.View(). logoView sits above rightView in the right column; it is
// produced by RenderLogo(RightWidth(), theme).
//
// anim carries the VP.6 focus-transition spring state (nil = no transition
// in flight, renders identically to the pre-VP.6 hard snap). The ±1-column
// width nudge applied to the gaining/losing pane's OUTER box width here is
// deliberately local to this function: LeftWidth()/RightWidth() (and
// therefore RenderLogo's width and the detail viewport's content width,
// which callers compute from those same methods) are never perturbed, so
// leftView/rightView/logoView content never needs to reflow mid-transition —
// only the box each already-rendered string is placed into changes size.
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

	// The gaining pane grows by up to 1 column as it nears full focus; the
	// other pane shrinks by the same amount so the total row width is
	// unchanged (JoinHorizontal requires the row to still fill the terminal).
	leftNudge := 0
	rightNudge := 0
	if anim != nil {
		switch anim.To {
		case FocusLeft:
			leftNudge = roundNudge(anim.Progress)
			rightNudge = -leftNudge
		case FocusRight:
			rightNudge = roundNudge(anim.Progress)
			leftNudge = -rightNudge
		}
	}

	leftBox := l.paneStyle(lw, leftFrac, leftNudge).Render(leftView)
	logoBox := l.logoStyle(rw, logoH).Render(logoView)
	detailBox := l.detailPaneStyle(rw, rightFrac, rightNudge, logoH).Render(rightView)

	rightColumn := lipgloss.JoinVertical(lipgloss.Left, logoBox, detailBox)
	row := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightColumn)

	return lipgloss.JoinVertical(lipgloss.Left, row, statusBarView)
}

// roundNudge converts a [0,1] spring progress into a 0/1 column nudge,
// rounding at the midpoint so the width settles exactly at 1 once the
// transition completes rather than drifting from float accumulation.
func roundNudge(progress float64) int {
	if progress >= 0.5 {
		return 1
	}
	return 0
}
