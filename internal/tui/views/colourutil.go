package views

import (
	"fmt"
	"image/color"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// IsColourDark returns true when c is a dark colour (perceived luminance
// < 0.5). Falls back to true (dark) for nil or transparent colours.
// Relocated from internal/tui/detail.go (TD.20) — see NodeTitle's doc
// comment for why.
func IsColourDark(c color.Color) bool {
	if c == nil {
		return true
	}
	// color.Color.RGBA() returns 16-bit channels (0-65535).
	r16, g16, b16, _ := c.RGBA()
	// Relative luminance approximation (ITU-R BT.709 coefficients).
	lum := (0.2126*float64(r16) + 0.7152*float64(g16) + 0.0722*float64(b16)) / 65535.0
	return lum < 0.5
}

// ColourHex converts a color.Color to a "#rrggbb" hex string for use in
// glamour's StylePrimitive.Color / BackgroundColor fields. Returns "" for
// nil or fully-transparent colours so callers can use HexPtr to skip nil
// fields. Relocated from internal/tui/detail.go (TD.20).
func ColourHex(c color.Color) string {
	if c == nil {
		return ""
	}
	// RGBA returns pre-multiplied 16-bit channels (0-65535); shift down to 8-bit.
	r16, g16, b16, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r16>>8, g16>>8, b16>>8)
}

// HexPtr converts a color.Color to the *string form required by glamour's
// StylePrimitive.Color field. Returns nil when the colour is nil or
// transparent so glamour falls back to its own defaults rather than
// receiving an empty string (which it would interpret as "no colour").
// Relocated from internal/tui/detail.go (TD.20).
func HexPtr(c color.Color) *string {
	s := ColourHex(c)
	if s == "" {
		return nil
	}
	return &s
}

// EdgeGlyph returns the directional glyph for an edge, from the perspective
// of the focal node: outgoing (nodeID == edge.From) means the node is the
// actor, incoming (nodeID == edge.To) means the node is the recipient.
// Relocated from internal/tui/detail.go (TD.20) so ProseRenderer's edge
// section can match DetailRenderer's exactly rather than drifting with its
// own subset (ProseRenderer previously computed direction differently and
// never used it — see git history for the dead `_ = direction` this
// replaces).
func EdgeGlyph(edgeType string, outgoing bool) string {
	switch edgeType {
	case string(types.EdgeBlocks):
		if outgoing {
			return "→" // this node blocks something
		}
		return "←" // something is blocking this node
	case string(types.EdgeParent):
		return "→"
	case string(types.EdgeWaitingOn):
		return "⊘"
	case string(types.EdgeRelated):
		return "◇"
	case "depends_on":
		return "→"
	default:
		if outgoing {
			return "→"
		}
		return "←"
	}
}

// AgeColours names the three colours AgeColourForDays selects between —
// callers supply their own palette's equivalents so this stays independent
// of any one renderer's palette type.
type AgeColours struct {
	Muted    color.Color
	Warn     color.Color
	Critical color.Color
}

// AgeColourForDays returns the appropriate colour based on an edge's age in
// days: 0-7 muted, 8-14 warn, 15+ critical. Relocated from
// internal/tui/detail.go (TD.20), generalised to take an AgeColours value
// instead of detail.go's tui-package Colours struct, since views cannot
// import package tui.
func AgeColourForDays(days int, c AgeColours) color.Color {
	switch {
	case days <= 7:
		return c.Muted
	case days <= 14:
		return c.Warn
	default:
		return c.Critical
	}
}
