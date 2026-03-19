package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// StatusBar holds the state for the full-width status bar rendered at the
// bottom of the TUI. Phase 4 agents update the capture and tracker fields
// via the CaptureText and TrackerText setters.
type StatusBar struct {
	// captureText is the capture-bar placeholder on the left side.
	captureText string

	// trackerText is the time-tracker placeholder on the right side.
	trackerText string

	// width is the terminal width, updated on WindowSizeMsg.
	width int

	// theme provides all colour values used in rendering.
	theme *ActiveTheme
}

// NewStatusBar creates a StatusBar with the given theme and a default width.
func NewStatusBar(theme *ActiveTheme) StatusBar {
	return StatusBar{
		captureText: "Capture…",
		trackerText: "No active tracker",
		theme:       theme,
	}
}

// SetWidth updates the terminal width so the bar can fill the full line.
func (sb *StatusBar) SetWidth(w int) {
	sb.width = w
}

// SetCaptureText updates the left-hand capture bar placeholder.
func (sb *StatusBar) SetCaptureText(s string) {
	sb.captureText = s
}

// SetTrackerText updates the right-hand time tracker placeholder.
func (sb *StatusBar) SetTrackerText(s string) {
	sb.trackerText = s
}

// SetTheme swaps the theme after a runtime theme change.
func (sb *StatusBar) SetTheme(t *ActiveTheme) {
	sb.theme = t
}

// View renders the status bar as a full-width string using Lipgloss.
// The layout is:
//
//	[capture text]           [time]  [tracker text]
func (sb *StatusBar) View() string {
	if sb.theme == nil {
		return ""
	}

	baseStyle := sb.theme.StyleStatusBar().
		Width(sb.width).
		MaxWidth(sb.width)

	// Left section: capture placeholder.
	captureStyle := lipgloss.NewStyle().
		Foreground(sb.theme.FgMuted()).
		Italic(true)
	leftContent := captureStyle.Render(sb.captureText)

	// Right section: clock + tracker.
	clockStr := time.Now().Format("15:04")
	trackerStyle := lipgloss.NewStyle().
		Foreground(sb.theme.FgMuted())
	rightContent := trackerStyle.Render(clockStr + "  " + sb.trackerText)

	// Calculate padding to push the right content to the far edge.
	leftWidth := lipgloss.Width(leftContent)
	rightWidth := lipgloss.Width(rightContent)
	padding := sb.width - leftWidth - rightWidth
	if padding < 1 {
		padding = 1
	}

	bar := leftContent + strings.Repeat(" ", padding) + rightContent
	return baseStyle.Render(bar)
}

// SectionHeader returns a formatted section header string using the
// section header style from the theme. Headers are uppercase throughout
// the TUI with a muted, bold appearance.
func SectionHeader(theme *ActiveTheme, title string) string {
	if theme == nil {
		return strings.ToUpper(title)
	}
	return theme.StyleSectionHeader().Render(strings.ToUpper(title))
}
