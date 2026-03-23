package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Spacer returns n space characters rendered with bg as their background colour.
// Use this instead of bare " " or strings.Repeat(" ", n) when joining the
// output of Lipgloss Render() calls — bare spaces between styled segments
// inherit the terminal default background at ANSI reset boundaries, producing
// visible bleed on coloured backgrounds.
//
// Pass lipgloss.NoColor{} for bg when no theme background is set.
func Spacer(n int, bg lipgloss.TerminalColor) string {
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
func PadLines(content string, width int, bg lipgloss.Color) string {
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
