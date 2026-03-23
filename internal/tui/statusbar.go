package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

	// nodeID is the short ID (first 8 chars) of the focused node.
	nodeID string

	// nodeTypes are the type badges for the focused node.
	nodeTypes []string

	// edgeCount is the number of edges connected to the focused node.
	edgeCount int

	// spinner is the bubbles spinner component for async operations.
	spinner spinner.Model

	// spinning indicates whether the spinner is currently active.
	spinning bool

	// spinnerMsg is the message shown alongside the spinner.
	spinnerMsg string
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

// SetNodeInfo updates the status bar with the focused node's details.
// id is truncated to 8 characters for display.
func (sb *StatusBar) SetNodeInfo(id string, nodeTypes []string, edgeCount int) {
	if len(id) > 8 {
		id = id[:8]
	}
	sb.nodeID    = id
	sb.nodeTypes = nodeTypes
	sb.edgeCount = edgeCount
}

// Update forwards tick messages to the spinner when it is spinning.
// Returns any command the spinner produces (e.g. the next tick).
func (sb *StatusBar) Update(msg tea.Msg) tea.Cmd {
	if !sb.spinning {
		return nil
	}
	var cmd tea.Cmd
	sb.spinner, cmd = sb.spinner.Update(msg)
	return cmd
}

// StartSpinner initialises the spinner with the given message, styles it to
// the current theme, and returns the first tick command to start the animation.
func (sb *StatusBar) StartSpinner(msg string) tea.Cmd {
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	if sb.theme != nil {
		s.Style = lipgloss.NewStyle().
			Foreground(sb.theme.AccentPrimary()).
			Background(sb.theme.StatusBar())
	}
	sb.spinner = s
	sb.spinning = true
	sb.spinnerMsg = msg
	return sb.spinner.Tick
}

// StopSpinner clears the spinner state, returning the status bar to its
// normal capture-text display.
func (sb *StatusBar) StopSpinner() {
	sb.spinning = false
	sb.spinnerMsg = ""
}

// View renders the status bar as a full-width string using Lipgloss.
// The layout is:
//
//	[capture text]   [node id  [types]  N edges]   [time]  [tracker text]
//
// IMPORTANT: every inner style must carry the StatusBar background colour.
// Bare strings or styles without Background() will bleed through to the
// terminal default at ANSI reset boundaries. This is the source of repeated
// background-bleed bugs in this file — do not remove Background() from any
// style below, and do not use strings.Repeat for spacers without bg.
func (sb *StatusBar) View() string {
	if sb.theme == nil {
		return ""
	}

	bg := sb.theme.StatusBar()

	baseStyle := sb.theme.StyleStatusBar().
		Width(sb.width).
		MaxWidth(sb.width)

	// All inner styles carry the same background to prevent bleed at ANSI
	// reset boundaries between concatenated styled segments.
	captureStyle := lipgloss.NewStyle().
		Foreground(sb.theme.FgMuted()).
		Background(bg).
		Italic(true)

	// When spinning, replace the capture area with the spinner + message.
	var leftContent string
	if sb.spinning {
		msgStyle := lipgloss.NewStyle().Foreground(sb.theme.FgMuted()).Background(bg)
		leftContent = sb.spinner.View() + Spacer(1, bg) + msgStyle.Render(sb.spinnerMsg)
	} else {
		leftContent = captureStyle.Render(sb.captureText)
	}

	// Centre section: focused node ID, type badges, and edge count.
	var centreContent string
	if sb.nodeID != "" {
		idStyle := lipgloss.NewStyle().Foreground(sb.theme.FgMuted()).Background(bg)
		typeStyle := lipgloss.NewStyle().Foreground(sb.theme.AccentSecondary()).Background(bg)

		parts := []string{idStyle.Render(sb.nodeID)}
		if len(sb.nodeTypes) > 0 {
			parts = append(parts, typeStyle.Render("["+strings.Join(sb.nodeTypes, " ")+"]"))
		}
		if sb.edgeCount > 0 {
			suffix := "s"
			if sb.edgeCount == 1 {
				suffix = ""
			}
			parts = append(parts, idStyle.Render(fmt.Sprintf("%d edge%s", sb.edgeCount, suffix)))
		}
		centreContent = strings.Join(parts, Spacer(2, bg))
	}

	// Right section: clock + tracker.
	clockStr := time.Now().Format("15:04")
	trackerStyle := lipgloss.NewStyle().
		Foreground(sb.theme.FgMuted()).
		Background(bg)
	rightContent := trackerStyle.Render(clockStr + "  " + sb.trackerText)

	// Distribute space: left, centre (centred), right.
	// Spacers are rendered with the status bar background so they don't bleed.
	spacerStyle := lipgloss.NewStyle().Background(bg)
	leftWidth := lipgloss.Width(leftContent)
	centreWidth := lipgloss.Width(centreContent)
	rightWidth := lipgloss.Width(rightContent)

	remaining := sb.width - leftWidth - centreWidth - rightWidth
	if remaining < 2 {
		remaining = 2
	}
	leftPad := remaining / 2
	rightPad := remaining - leftPad

	bar := leftContent +
		spacerStyle.Render(strings.Repeat(" ", leftPad)) +
		centreContent +
		spacerStyle.Render(strings.Repeat(" ", rightPad)) +
		rightContent
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
