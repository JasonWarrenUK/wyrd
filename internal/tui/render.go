package tui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// FillBackground rewrites the SGR control stream of an already-styled,
// multi-line string so that bg is asserted at the start of every line and
// re-asserted immediately after every reset (ESC[0m / ESC[m) or default-
// background SGR (ESC[49m). This repaints interior cells that carry a
// terminal-default background — the backgroundless padding cells emitted
// inside components we cannot reach via styling (notably the bubbles viewport
// and huh's field separator) — while preserving every foreground colour,
// bold/italic/underline run, and glyph in the source verbatim.
//
// Use this for content produced by third-party components (huh forms) whose
// interior cells cannot be themed. For plain wyrd-owned content, PadLines is
// sufficient. FillBackground does NOT pad lines to a fixed width; call
// PadLines first and pass its output to FillBackground.
//
// Implementation note: resets in this dependency tree (lipgloss v2, huh v2,
// x/ansi v0.11.6) are always standalone ESC[m / ESC[0m, so a byte-prefix
// scan is sufficient. If a future huh version bundles resets (e.g. ESC[0;32m),
// replace the walkAndReassert loop with one driven by ansi.DecodeSequence to
// classify each sequence rather than prefix-match.
func FillBackground(content string, bg color.Color) string {
	if content == "" || bg == nil {
		return content
	}

	// Derive the exact opening background SGR lipgloss would emit for bg
	// under the current colour profile: render a single space, then take the
	// bytes before the space character. This stays byte-compatible with
	// lipgloss across truecolor / 256 / 16 / no-colour degradation. On a
	// no-colour profile the probe produces no SGR, open is empty, and we
	// return content unchanged.
	probe := lipgloss.NewStyle().Background(bg).Render(" ")
	open := probe
	if i := strings.IndexByte(probe, ' '); i >= 0 {
		open = probe[:i]
	}
	if open == "" {
		return content
	}

	const reset = "\x1b[0m"
	const resetShort = "\x1b[m"
	const defaultBg = "\x1b[49m"

	var b strings.Builder
	b.Grow(len(content) + 32)

	for i, line := range strings.Split(content, "\n") {
		if i > 0 {
			b.WriteByte('\n')
		}
		// Assert bg at the start of the line so leading plain spaces (the
		// viewport's left-pad and separator blank rows) are painted.
		b.WriteString(open)
		walkAndReassert(&b, line, open, reset, resetShort, defaultBg)
		// Close the line cleanly so bg does not bleed past the content.
		b.WriteString(reset)
	}
	return b.String()
}

// walkAndReassert copies src into b, re-emitting open immediately after every
// occurrence of a full reset or an explicit default-background SGR. All other
// bytes — glyphs, foreground/attribute SGRs, OSC-8 hyperlinks — pass through
// unchanged, so foregrounds and glyph styling are fully preserved.
func walkAndReassert(b *strings.Builder, src, open, reset, resetShort, defaultBg string) {
	for len(src) > 0 {
		switch {
		case strings.HasPrefix(src, reset):
			b.WriteString(reset)
			b.WriteString(open)
			src = src[len(reset):]
		case strings.HasPrefix(src, resetShort):
			b.WriteString(resetShort)
			b.WriteString(open)
			src = src[len(resetShort):]
		case strings.HasPrefix(src, defaultBg):
			// Replace the default-bg request with our own bg assertion.
			b.WriteString(open)
			src = src[len(defaultBg):]
		default:
			b.WriteByte(src[0])
			src = src[1:]
		}
	}
}

// Spacer returns n space characters rendered with bg as their background colour.
// Use this instead of bare " " or strings.Repeat(" ", n) when joining the
// output of Lipgloss Render() calls — bare spaces between styled segments
// inherit the terminal default background at ANSI reset boundaries, producing
// visible bleed on coloured backgrounds.
//
// Pass lipgloss.NoColor{} for bg when no theme background is set.
func Spacer(n int, bg color.Color) string {
	if n <= 0 {
		return ""
	}
	return lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", n))
}

// PadLines ensures every line in content is exactly width columns wide with
// bg as the background colour. This must be called before passing content to
// any container component (viewport, list) that adds its own (backgroundless)
// padding — otherwise ANSI resets in styled text segments break the outer
// background before the padding spaces are reached, producing a terminal-colour
// strip at the right edge of each line.
//
// All PaneModel.View() implementations that own their background should pass
// their output through PadLines before returning it.
func PadLines(content string, width int, bg color.Color) string {
	if width <= 0 || content == "" {
		return content
	}
	padStyle := lipgloss.NewStyle().Width(width).Background(bg)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = padStyle.Render(line)
	}
	return strings.Join(lines, "\n")
}
