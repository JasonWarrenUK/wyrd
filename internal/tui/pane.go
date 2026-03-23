// Package tui provides the Bubble Tea application shell for Wyrd.
// It defines the split-pane layout, theme engine, keyboard model,
// command palette, and status bar. Phase 4 agents mount content
// views by implementing the PaneModel interface.
package tui

import (
	"image/color"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// KeyBinding describes a single keyboard shortcut and its action.
type KeyBinding struct {
	// Key is the human-readable key name (e.g. "ctrl+w", "gg").
	Key string

	// Description is a short label shown in the command palette.
	Description string
}

// PaneModel is the interface that Phase 4 agents implement to provide
// content within a pane. All state must be held inside the implementation;
// the frame calls Update and View on every tick.
//
// Implementors: View() must return content where every line is padded to the
// pane width using PadLines(). This ensures the background colour extends to
// the pane edge even when inner container components (viewport, list) emit
// backgroundless padding spaces after ANSI-reset boundaries.
type PaneModel interface {
	// Update handles a Bubble Tea message and returns the updated pane
	// model along with any commands to run.
	Update(msg tea.Msg) (PaneModel, tea.Cmd)

	// View renders the pane content as a string ready for Lipgloss layout.
	// Every line must be padded to the pane width via PadLines so that
	// background colour extends to the pane edge without bleed.
	View() string

	// KeyBindings returns the list of keybindings this pane handles,
	// used to populate the command palette.
	KeyBindings() []KeyBinding

	// HandleFocusLost is called by the root model immediately before focus
	// moves away from this pane. Implementations may return a command (e.g.
	// to flush pending state) or nil for a no-op.
	HandleFocusLost() tea.Cmd
}

// emptyPane is the default PaneModel shown when no content has been mounted.
type emptyPane struct {
	theme *ActiveTheme
}

// Update is a no-op for the empty pane.
func (e emptyPane) Update(_ tea.Msg) (PaneModel, tea.Cmd) {
	return e, nil
}

// View renders a muted placeholder message.
func (e emptyPane) View() string {
	if e.theme == nil {
		return "No content"
	}
	style := e.theme.StyleMuted()
	return style.Render("No content")
}

// KeyBindings returns an empty slice — the empty pane handles nothing.
func (e emptyPane) KeyBindings() []KeyBinding {
	return nil
}

// HandleFocusLost is a no-op for the empty pane.
func (e emptyPane) HandleFocusLost() tea.Cmd { return nil }

// NewEmptyPane returns a PaneModel that renders a muted placeholder.
func NewEmptyPane(theme *ActiveTheme) PaneModel {
	return emptyPane{theme: theme}
}

// viewportPane is a PaneModel that wraps bubbles/viewport for scrollable
// detail content. It replaces the static detailPane so long node bodies,
// edge lists, and budget sections can be scrolled with j/k, arrow keys,
// Page Up, and Page Down.
type viewportPane struct {
	vp         viewport.Model
	bg         color.Color
	rawContent string // unpadded source content; re-padded on resize
}

// newViewportPane creates a viewportPane sized to the given dimensions and
// pre-loaded with the provided content string. bg is the theme background
// colour. Content is padded via PadLines before being set so every line
// reaches the viewport width; this prevents terminal-colour bleed at ANSI
// reset boundaries regardless of what the container component does.
func newViewportPane(width, height int, content string, bg color.Color) viewportPane {
	vp := viewport.New(viewport.WithWidth(width), viewport.WithHeight(height))
	vp.Style = lipgloss.NewStyle().Background(bg)
	vp.SetContent(PadLines(content, width, bg))
	return viewportPane{vp: vp, bg: bg, rawContent: content}
}

// Update handles scroll key messages forwarded from the root model when the
// right pane has focus, and resizes the viewport on WindowSizeMsg.
func (d viewportPane) Update(msg tea.Msg) (PaneModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Borders consume 2 columns and 2 rows from the layout dimensions.
		newWidth := msg.Width/2 - 2
		newHeight := msg.Height - 3 // status bar (1) + top border (1) + bottom border (1)
		if newWidth < 1 {
			newWidth = 1
		}
		if newHeight < 1 {
			newHeight = 1
		}
		d.vp.SetWidth(newWidth)
		d.vp.SetHeight(newHeight)
		d.vp.Style = lipgloss.NewStyle().Background(d.bg)
		// Re-pad to the new width so lines fill the resized viewport.
		d.vp.SetContent(PadLines(d.rawContent, d.vp.Width(), d.bg))
	default:
		d.vp, cmd = d.vp.Update(msg)
	}
	return d, cmd
}

// View renders the viewport content.
func (d viewportPane) View() string {
	return d.vp.View()
}

// KeyBindings advertises scroll shortcuts shown in the command palette.
func (d viewportPane) KeyBindings() []KeyBinding {
	return []KeyBinding{
		{Key: "j/↓", Description: "Scroll down"},
		{Key: "k/↑", Description: "Scroll up"},
		{Key: "ctrl+d", Description: "Page down"},
		{Key: "ctrl+u", Description: "Page up"},
		{Key: "g", Description: "Scroll to top"},
		{Key: "G", Description: "Scroll to bottom"},
	}
}

// HandleFocusLost is a no-op for the viewport pane.
func (d viewportPane) HandleFocusLost() tea.Cmd { return nil }
