package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// wordmarkFiglet is the "wyrd" wordmark generated via figlet font "ANSI Shadow".
// 6 glyph rows + 1 trailing blank (= LogoHeight for wide terminals).
// Uses box-drawing / block characters (same Unicode family as the pane borders).
const wordmarkFiglet = `██╗    ██╗██╗   ██╗██████╗ ██████╗
██║    ██║╚██╗ ██╔╝██╔══██╗██╔══██╗
██║ █╗ ██║ ╚████╔╝ ██████╔╝██║  ██║
██║███╗██║  ╚██╔╝  ██╔══██╗██║  ██║
╚███╔███╔╝   ██║   ██║  ██║██████╔╝
 ╚══╝╚══╝    ╚═╝   ╚═╝  ╚═╝╚═════╝
`

// wordmarkInnerWidth is the column width of the ANSI Shadow figlet wordmark.
// The logo box must offer at least this many inner columns (its outer width
// minus the 2 border columns) for the art to render without clipping.
const wordmarkInnerWidth = 35

// logoBorderRows is the number of rows the logo box's rounded border consumes
// (top + bottom). The logo box is bordered to match the list and detail panes.
const logoBorderRows = 2

// figletContentRows is the inner content height of the figlet wordmark
// (6 glyph rows; no trailing gap — the box border provides the framing).
const figletContentRows = 6

// fallbackContentRows is the inner content height of the narrow-terminal
// text fallback (a single styled "WYRD" line).
const fallbackContentRows = 1

// logoContentHeight returns the inner content height of the logo (glyph rows
// only, excluding the box border) for the given OUTER right-column width.
func logoContentHeight(width int) int {
	if width-logoBorderRows >= wordmarkInnerWidth {
		return figletContentRows
	}
	return fallbackContentRows
}

// LogoHeight returns the OUTER height of the bordered logo box for the given
// right-column width: the inner content height plus the box's border rows. It
// is the single source of truth consulted by the layout and viewport sizing so
// the logo height can never desync between them.
//
//   - figlet form: 6 glyph rows + 2 border rows = 8
//   - text fallback: 1 title row + 2 border rows = 3
func LogoHeight(width int) int {
	return logoContentHeight(width) + logoBorderRows
}

// RenderLogo returns the styled logo CONTENT (the inner glyph rows only; the
// bordered logo box is applied by the layout). The content is coloured from the
// supplied theme and centred horizontally within the inner width. width is the
// OUTER right-column width; the inner width used for centring is width minus the
// 2 border columns.
//
// Wide (inner width >= wordmarkInnerWidth): the ANSI Shadow figlet wordmark in
// AccentPrimary over BgPrimary, centred.
//
// Narrow: a single bold "WYRD" title line in AccentPrimary, centred.
func RenderLogo(width int, theme *ActiveTheme) string {
	bg := theme.BgPrimary()
	fg := theme.AccentPrimary()

	innerWidth := width - logoBorderRows
	if innerWidth < 1 {
		innerWidth = 1
	}

	if innerWidth >= wordmarkInnerWidth {
		// Centre the whole figlet block within the inner width. Align(Center)
		// pads each line on both sides with the background colour so the glyphs
		// sit in the middle of the box without bleed.
		style := lipgloss.NewStyle().
			Foreground(fg).
			Background(bg).
			Width(innerWidth).
			Align(lipgloss.Center)
		block := strings.TrimRight(wordmarkFiglet, "\n")
		return style.Render(block)
	}

	// Narrow fallback: a single bold "WYRD" title, centred.
	style := lipgloss.NewStyle().
		Foreground(fg).
		Background(bg).
		Bold(true).
		Width(innerWidth).
		Align(lipgloss.Center)
	return style.Render("WYRD")
}
