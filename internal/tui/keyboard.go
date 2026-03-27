package tui

import "charm.land/bubbles/v2/key"

// AppKeyMap holds the application-level key bindings for the TUI shell.
// Navigation (up/down/page/jump) is intentionally absent — those are handled
// by the child components (bubbles/list, bubbles/viewport) via their own
// KeyMap configuration. Only truly app-level concerns live here.
//
// All bindings use modifier keys so that unmodified printable characters
// (which terminals may emit as fragments of protocol responses) never trigger
// application actions.
type AppKeyMap struct {
	SwitchPane     key.Binding
	Quit           key.Binding
	CommandPalette key.Binding
	FuzzyPalette   key.Binding
	Capture        key.Binding
	EditNode       key.Binding
	ArchiveNode    key.Binding
}

// DefaultAppKeyMap returns the built-in key bindings.
func DefaultAppKeyMap() AppKeyMap {
	return AppKeyMap{
		SwitchPane: key.NewBinding(
			key.WithKeys("ctrl+w"),
			key.WithHelp("ctrl+w", "switch pane"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
		CommandPalette: key.NewBinding(
			key.WithKeys("ctrl+p"),
			key.WithHelp("ctrl+p", "command palette"),
		),
		FuzzyPalette: key.NewBinding(
			key.WithKeys("ctrl+k"),
			key.WithHelp("ctrl+k", "fuzzy search"),
		),
		Capture: key.NewBinding(
			key.WithKeys("ctrl+n"),
			key.WithHelp("ctrl+n", "capture node"),
		),
		EditNode: key.NewBinding(
			key.WithKeys("ctrl+o"),
			key.WithHelp("ctrl+o", "edit node"),
		),
		ArchiveNode: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("ctrl+d", "archive node"),
		),
	}
}

// AllBindings returns the key bindings as a slice of KeyBinding for display
// in the command palette and help views.
func (km AppKeyMap) AllBindings() []KeyBinding {
	bindings := []key.Binding{
		km.SwitchPane,
		km.Quit,
		km.CommandPalette,
		km.FuzzyPalette,
		km.Capture,
		km.EditNode,
		km.ArchiveNode,
	}
	result := make([]KeyBinding, 0, len(bindings))
	for _, b := range bindings {
		if b.Enabled() {
			h := b.Help()
			result = append(result, KeyBinding{Key: h.Key, Description: h.Desc})
		}
	}
	return result
}
