package tui

import (
	"strings"

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
	ActionJumpTop    // gg
	ActionJumpBottom // G

	// Application actions.
	ActionSearch         // /
	ActionSwitchPane     // Ctrl+W
	ActionQuit           // q or Ctrl+C
	ActionCommandPalette // :
	ActionFuzzyPalette   // Ctrl+K
)

// keyRule maps a key sequence to a KeyAction.
type keyRule struct {
	// sequence is the key string as returned by tea.KeyMsg.String().
	sequence string
	action   KeyAction
}

// defaultKeyRules are the built-in vim-style bindings for the shell frame.
var defaultKeyRules = []keyRule{
	{"h", ActionMoveLeft},
	{"j", ActionMoveDown},
	{"k", ActionMoveUp},
	{"l", ActionMoveRight},
	{"G", ActionJumpBottom},
	{"/", ActionSearch},
	{"ctrl+w", ActionSwitchPane},
	{"q", ActionQuit},
	{"ctrl+c", ActionQuit},
	{":", ActionCommandPalette},
	{"ctrl+k", ActionFuzzyPalette},
}

// KeyMap holds the complete set of active key rules and the partial sequence
// buffer needed for multi-key sequences (e.g. "gg").
type KeyMap struct {
	rules  []keyRule
	buffer string // pending prefix, e.g. "g" while waiting for "gg"
}

// NewKeyMap builds a KeyMap from the default rules.
func NewKeyMap() *KeyMap {
	rules := make([]keyRule, len(defaultKeyRules))
	copy(rules, defaultKeyRules)
	// "gg" must be resolved at dispatch time using the buffer.
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
// It manages the multi-key buffer for sequences such as "gg".
func (km *KeyMap) Dispatch(msg tea.KeyPressMsg) KeyAction {
	key := msg.String()

	// Check for multi-key sequence completion first.
	if km.buffer != "" {
		candidate := km.buffer + key
		for _, r := range km.rules {
			if r.sequence == candidate {
				km.buffer = ""
				return r.action
			}
		}
		// The sequence did not complete — flush the buffer and fall through.
		km.buffer = ""
	}

	// Check whether this key starts a multi-key prefix.
	if isPrefix(key, km.rules) {
		km.buffer = key
		return ActionNone
	}

	// Direct single-key lookup.
	for _, r := range km.rules {
		if r.sequence == key {
			return r.action
		}
	}

	return ActionNone
}

// isPrefix returns true when key is the start of any multi-character sequence
// in rules but is not itself a complete rule.
func isPrefix(key string, rules []keyRule) bool {
	// "g" is a prefix for "gg" — detect this generically.
	for _, r := range rules {
		if strings.HasPrefix(r.sequence, key) && r.sequence != key && len(r.sequence) > 1 {
			return true
		}
	}
	return false
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
	}

	// Add "gg" manually — it is resolved via the buffer mechanism.
	bindings := []KeyBinding{
		{Key: "gg", Description: "Jump to top"},
	}

	for _, r := range km.rules {
		desc, ok := descriptions[r.action]
		if !ok {
			desc = r.sequence
		}
		bindings = append(bindings, KeyBinding{Key: r.sequence, Description: desc})
	}
	return bindings
}
