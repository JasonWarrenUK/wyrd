package tui

import (
	"image/color"
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
// Wide (inner width >= wordmarkInnerWidth): the ANSI Shadow figlet wordmark
// carries a left-to-right AccentSecondary → AccentPrimary → AccentSecondary
// gradient over BgPrimary, centred. This is the pane border gradient's stop
// sequence reversed (the border runs AccentPrimary → AccentSecondary →
// AccentPrimary) — same spatial sweep direction, inverted colour order, so
// the wordmark reads as answering the border rather than repeating it.
//
// Narrow: a single bold "WYRD" title line, flat AccentPrimary (too few
// columns for a 3-character-wide gradient to read as anything but noise),
// centred.
func RenderLogo(width int, theme *ActiveTheme) string {
	bg := theme.BgPrimary()
	fg := theme.AccentPrimary()

	innerWidth := width - logoBorderRows
	if innerWidth < 1 {
		innerWidth = 1
	}

	if innerWidth >= wordmarkInnerWidth {
		block := strings.TrimRight(wordmarkFiglet, "\n")
		gradient := gradientText(block, bg,
			theme.AccentSecondary(), theme.AccentPrimary(), theme.AccentSecondary())
		// Centre the pre-coloured block within the inner width. Align(Center)
		// pads each line on both sides with the background colour so the
		// glyphs sit in the middle of the box without bleed; it operates on
		// the rendered string's visible width so it composes correctly with
		// gradient's already-ANSI-coloured text.
		return lipgloss.NewStyle().
			Background(bg).
			Width(innerWidth).
			Align(lipgloss.Center).
			Render(gradient)
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

// gradientText colours block (a multi-line, monospace ANSI-art string) with
// a left-to-right horizontal gradient through stops, every row sharing the
// SAME per-column colour so the gradient reads as one flat horizontal sweep
// across the whole block rather than varying by row. bg is applied as every
// character's background so the glyphs don't bleed into the surrounding box.
//
// Deliberately uses lipgloss.Blend1D(width, stops...) — ONE gradient sample
// per column, reused for every row — rather than Blend2D. Blend2D's 2D
// sampling is centred and diagonal-length-normalised for arbitrary angles,
// which for a plain horizontal (angle 0) sweep over a narrow block maps the
// last column to a gradientPos just short of 1.0: the gradient would reach
// its middle stop exactly but never quite return to the first stop by the
// final column (verified: Blend2D's last column landed ~84 units off the
// expected stop on the blue channel in a 10-wide test, while Blend1D's
// matched the stop exactly). Blend1D has no such centring step — column i
// maps directly to sample i — so it starts and ends exactly on the stops.
func gradientText(block string, bg color.Color, stops ...color.Color) string {
	lines := strings.Split(block, "\n")
	width := 0
	for _, line := range lines {
		if w := lipgloss.Width(line); w > width {
			width = w
		}
	}
	if width == 0 {
		return block
	}

	colours := lipgloss.Blend1D(width, stops...)

	var out strings.Builder
	for row, line := range lines {
		col := 0
		for _, r := range line {
			c := colours[col]
			out.WriteString(lipgloss.NewStyle().Foreground(c).Background(bg).Render(string(r)))
			col++
		}
		if row < len(lines)-1 {
			out.WriteByte('\n')
		}
	}
	return out.String()
}
