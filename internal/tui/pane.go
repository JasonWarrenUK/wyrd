// Package tui provides the Bubble Tea application shell for Wyrd.
// It defines the split-pane layout, theme engine, keyboard model,
// command palette, and status bar. Phase 4 agents mount content
// views by implementing the PaneModel interface.
package tui

import (
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
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
type PaneModel interface {
	// Update handles a Bubble Tea message and returns the updated pane
	// model along with any commands to run.
	Update(msg tea.Msg) (PaneModel, tea.Cmd)

	// View renders the pane content as a string ready for Lipgloss layout.
	View() string

	// KeyBindings returns the list of keybindings this pane handles,
	// used to populate the command palette.
	KeyBindings() []KeyBinding
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

// NewEmptyPane returns a PaneModel that renders a muted placeholder.
func NewEmptyPane(theme *ActiveTheme) PaneModel {
	return emptyPane{theme: theme}
}

// viewportPane is a PaneModel that wraps bubbles/viewport for scrollable
// detail content. It replaces the static detailPane so long node bodies,
// edge lists, and budget sections can be scrolled with j/k, arrow keys,
// Page Up, and Page Down.
type viewportPane struct {
	vp viewport.Model
}

// newViewportPane creates a viewportPane sized to the given dimensions and
// pre-loaded with the provided content string.
func newViewportPane(width, height int, content string) viewportPane {
	vp := viewport.New(width, height)
	vp.SetContent(content)
	return viewportPane{vp: vp}
}

// Update handles scroll key messages forwarded from the root model when the
// right pane has focus, and resizes the viewport on WindowSizeMsg.
func (d viewportPane) Update(msg tea.Msg) (PaneModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Borders consume 2 columns and 2 rows from the layout dimensions.
		d.vp.Width = msg.Width/2 - 2
		d.vp.Height = msg.Height - 3 // status bar (1) + top border (1) + bottom border (1)
		if d.vp.Width < 1 {
			d.vp.Width = 1
		}
		if d.vp.Height < 1 {
			d.vp.Height = 1
		}
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
