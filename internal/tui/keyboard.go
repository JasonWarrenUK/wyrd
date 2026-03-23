package tui

import (
	tea "charm.land/bubbletea/v2"
)

// KeyAction identifies what should happen in response to a key sequence.
type KeyAction int

const (
	// ActionNone means the key was not recognised and should be forwarded
	// to the focused pane.
	ActionNone KeyAction = iota

	// Navigation actions.
	ActionMoveLeft
	ActionMoveDown
	ActionMoveUp
	ActionMoveRight
	ActionJumpTop    // shift+alt+up
	ActionJumpBottom // shift+alt+down

	// Application actions.
	ActionSearch         // /
	ActionSwitchPane     // Ctrl+W
	ActionQuit           // q or Ctrl+C
	ActionCommandPalette // :
	ActionFuzzyPalette   // Ctrl+K
	ActionCapture        // i
)

// keyRule maps a key sequence to a KeyAction.
type keyRule struct {
	// sequence is the key string as returned by tea.KeyMsg.String().
	sequence string
	action   KeyAction
}

// defaultKeyRules are the built-in bindings for the shell frame.
var defaultKeyRules = []keyRule{
	{"h", ActionMoveLeft},
	{"j", ActionMoveDown},
	{"k", ActionMoveUp},
	{"l", ActionMoveRight},
	{"alt+shift+up", ActionJumpTop},
	{"alt+shift+down", ActionJumpBottom},
	{"/", ActionSearch},
	{"ctrl+w", ActionSwitchPane},
	{"q", ActionQuit},
	{"ctrl+c", ActionQuit},
	{":", ActionCommandPalette},
	{"ctrl+k", ActionFuzzyPalette},
	{"i", ActionCapture},
}

// KeyMap holds the complete set of active key rules.
type KeyMap struct {
	rules []keyRule
}

// NewKeyMap builds a KeyMap from the default rules.
func NewKeyMap() *KeyMap {
	rules := make([]keyRule, len(defaultKeyRules))
	copy(rules, defaultKeyRules)
	return &KeyMap{rules: rules}
}

// Register adds a custom keybinding. Phase 4 agents call this to extend
// the global key map. If the sequence already exists the new rule replaces it.
func (km *KeyMap) Register(sequence string, action KeyAction) {
	for i, r := range km.rules {
		if r.sequence == sequence {
			km.rules[i].action = action
			return
		}
	}
	km.rules = append(km.rules, keyRule{sequence: sequence, action: action})
}

// RegisterCustom is the same as Register but accepts an arbitrary KeyAction
// value defined by the caller. Callers must use values >= ActionCustomBase.
func (km *KeyMap) RegisterCustom(binding KeyBinding, action KeyAction) {
	km.Register(binding.Key, action)
}

// Dispatch processes a key message and returns the resolved KeyAction.
func (km *KeyMap) Dispatch(msg tea.KeyPressMsg) KeyAction {
	key := msg.String()
	for _, r := range km.rules {
		if r.sequence == key {
			return r.action
		}
	}
	return ActionNone
}

// AllBindings returns the full list of KeyBindings suitable for display
// in the command palette.
func (km *KeyMap) AllBindings() []KeyBinding {
	descriptions := map[KeyAction]string{
		ActionMoveLeft:       "Move focus left",
		ActionMoveDown:       "Move focus down",
		ActionMoveUp:         "Move focus up",
		ActionMoveRight:      "Move focus right",
		ActionJumpTop:        "Jump to top",
		ActionJumpBottom:     "Jump to bottom",
		ActionSearch:         "Search",
		ActionSwitchPane:     "Switch pane",
		ActionQuit:           "Quit",
		ActionCommandPalette: "Open command palette",
		ActionFuzzyPalette:   "Fuzzy command search",
		ActionCapture:        "Capture node",
	}

	bindings := make([]KeyBinding, 0, len(km.rules))
	for _, r := range km.rules {
		desc, ok := descriptions[r.action]
		if !ok {
			desc = r.sequence
		}
		bindings = append(bindings, KeyBinding{Key: r.sequence, Description: desc})
	}
	return bindings
}
