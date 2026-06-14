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

// wordmarkMinWidth is the minimum right-column inner width at which the figlet
// wordmark (35 cols) renders without clipping. Below this threshold the logo
// falls back to a single styled-text line so it always fits.
const wordmarkMinWidth = 37

// LogoHeight returns the number of rows reserved for the logo pane given the
// available inner width. It is the single source of truth consulted by the
// layout and viewport sizing so the logo height can never desync between them.
//
//   - width >= wordmarkMinWidth: figlet wordmark (6 glyph rows + 1 gap = 7)
//   - width < wordmarkMinWidth:  styled-text fallback (1 title row + 1 gap = 2)
func LogoHeight(width int) int {
	if width >= wordmarkMinWidth {
		return 7
	}
	return 2
}

// RenderLogo returns the styled logo string for the given width, coloured from
// the supplied theme. Every line is padded to width with BgPrimary so the
// background never bleeds through at the pane edge.
//
// Wide (>= wordmarkMinWidth): renders the ANSI Shadow figlet wordmark in
// AccentPrimary over BgPrimary.
//
// Narrow (< wordmarkMinWidth): renders a single bold "WYRD" title line in
// AccentPrimary, padded to width.
func RenderLogo(width int, theme *ActiveTheme) string {
	bg := theme.BgPrimary()
	fg := theme.AccentPrimary()

	if width >= wordmarkMinWidth {
		style := lipgloss.NewStyle().
			Foreground(fg).
			Background(bg)

		// Render each line of the figlet individually so background padding is
		// applied per-line rather than to the whole block at once (which would
		// expand the string width beyond the box).
		lines := strings.Split(strings.TrimRight(wordmarkFiglet, "\n"), "\n")
		var sb strings.Builder
		for _, line := range lines {
			sb.WriteString(style.Render(line))
			sb.WriteString("\n")
		}
		// Trailing gap row (the 7th row) — a blank line padded to width.
		sb.WriteString(Spacer(width, bg))
		return sb.String()
	}

	// Narrow fallback: bold title line + gap row.
	style := lipgloss.NewStyle().
		Foreground(fg).
		Background(bg).
		Bold(true)
	title := style.Render("WYRD")
	gap := Spacer(width, bg)
	return title + "\n" + gap
}
