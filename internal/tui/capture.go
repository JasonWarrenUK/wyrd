// Package tui implements the terminal user-interface layer for Wyrd.
package tui

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// DefaultCaptureKey is the keystroke that focuses the capture bar from
// anywhere in the TUI.
const DefaultCaptureKey = "i"

// CaptureResult holds the node (and optional edge) produced by a capture
// submission so the caller can persist them.
type CaptureResult struct {
	// Node is the newly created node.
	Node *types.Node

	// Edge is a "related" edge from Node to SelectedNodeID, or nil when no
	// node was selected in the right pane at capture time.
	Edge *types.Edge
}

// CaptureBar is the persistent single-line input that lives in the status-bar
// area and allows rapid node creation from anywhere in the TUI.
//
// Prefix syntax (case-insensitive):
//
//	j:  → journal node
//	n:  → note node
//	t:  → task node (default when no prefix given)
type CaptureBar struct {
	// focused reports whether the capture bar currently has keyboard focus.
	focused bool

	// input holds the current text in the input field.
	input string

	// selectedNodeID is the UUID of whatever node is selected in the right
	// pane when the capture bar is focused. May be empty.
	selectedNodeID string

	// clock is used to stamp new nodes.
	clock types.Clock

	// store is used to persist new nodes and edges.
	store types.StoreFS
}

// NewCaptureBar creates a CaptureBar wired to the provided store and clock.
func NewCaptureBar(store types.StoreFS, clock types.Clock) *CaptureBar {
	return &CaptureBar{
		store: store,
		clock: clock,
	}
}

// Focus gives the capture bar keyboard focus. selectedNodeID should be the
// UUID of any node currently selected in the right pane (empty string if
// none).
func (c *CaptureBar) Focus(selectedNodeID string) {
	c.focused = true
	c.selectedNodeID = selectedNodeID
}

// Blur removes keyboard focus from the capture bar without submitting.
func (c *CaptureBar) Blur() {
	c.focused = false
}

// IsFocused reports whether the capture bar currently has focus.
func (c *CaptureBar) IsFocused() bool {
	return c.focused
}

// Input returns the current text in the capture bar.
func (c *CaptureBar) Input() string {
	return c.input
}

// SetInput replaces the current input text. Used for testing and TUI wiring.
func (c *CaptureBar) SetInput(text string) {
	c.input = text
}

// AppendRune appends a single rune to the current input.
func (c *CaptureBar) AppendRune(r rune) {
	c.input += string(r)
}

// Backspace removes the last character from the input.
func (c *CaptureBar) Backspace() {
	runes := []rune(c.input)
	if len(runes) > 0 {
		c.input = string(runes[:len(runes)-1])
	}
}

// Submit parses the current input, creates a node (and optionally an edge),
// persists them via the store, and resets the capture bar. It returns a
// CaptureResult describing what was created. Returns nil if the input is
// empty after trimming.
func (c *CaptureBar) Submit() (*CaptureResult, error) {
	raw := strings.TrimSpace(c.input)
	if raw == "" {
		c.reset()
		return nil, nil
	}

	nodeType, body := parseCapturePrefixes(raw)
	now := c.clock.Now()

	node := &types.Node{
		ID:       uuid.New().String(),
		Body:     body,
		Types:    []string{nodeType},
		Created:  now,
		Modified: now,
		Properties: map[string]interface{}{
			"status": "inbox",
		},
	}

	if nodeType == "journal" {
		// Journal nodes carry the creation date as date.about.
		node.Date.About = &now
		delete(node.Properties, "status")
	} else if nodeType == "note" {
		// Notes do not carry a status.
		delete(node.Properties, "status")
	}

	if err := c.store.WriteNode(node); err != nil {
		return nil, err
	}

	result := &CaptureResult{Node: node}

	// If a node is selected in the right pane, create a "related" edge.
	if c.selectedNodeID != "" {
		edge := &types.Edge{
			ID:      uuid.New().String(),
			Type:    string(types.EdgeRelated),
			From:    node.ID,
			To:      c.selectedNodeID,
			Created: now,
		}
		if err := c.store.WriteEdge(edge); err != nil {
			// Non-fatal: return what we have.
			result.Edge = nil
		} else {
			result.Edge = edge
		}
	}

	c.reset()
	return result, nil
}

// reset clears the capture bar state after a successful submission.
func (c *CaptureBar) reset() {
	c.focused = false
	c.input = ""
	c.selectedNodeID = ""
}

// parseCapturePrefixes inspects the raw input for a recognised prefix and
// returns the node type and body text. The default type is "task".
func parseCapturePrefixes(raw string) (nodeType, body string) {
	// Prefixes are matched case-insensitively up to the first colon.
	lower := strings.ToLower(raw)

	switch {
	case strings.HasPrefix(lower, "j:"):
		return "journal", strings.TrimSpace(raw[2:])
	case strings.HasPrefix(lower, "n:"):
		return "note", strings.TrimSpace(raw[2:])
	case strings.HasPrefix(lower, "t:"):
		return "task", strings.TrimSpace(raw[2:])
	default:
		return "task", raw
	}
}

// CaptureBarPlaceholder returns the placeholder text shown when the capture
// bar is not focused and empty.
func CaptureBarPlaceholder() string {
	return "Press i to capture — j: journal · n: note · t: task (default)"
}

// unusedTime is a compile-time assertion that time is imported.
var _ = time.Time{}
