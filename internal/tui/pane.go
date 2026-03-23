// Package tui provides the Bubble Tea application shell for Wyrd.
// It defines the split-pane layout, theme engine, keyboard model,
// command palette, and status bar. Phase 4 agents mount content
// views by implementing the PaneModel interface.
package tui

import tea "github.com/charmbracelet/bubbletea"

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

// detailPane is a PaneModel that renders a pre-rendered detail string.
// It is replaced wholesale whenever a new node is selected.
type detailPane struct {
	content string
}

func (d detailPane) Update(msg tea.Msg) (PaneModel, tea.Cmd) { return d, nil }
func (d detailPane) View() string                             { return d.content }
func (d detailPane) KeyBindings() []KeyBinding                { return nil }
