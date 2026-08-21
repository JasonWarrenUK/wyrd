package tui

import (
	"image/color"

	"github.com/jasonwarrenuk/wyrd/internal/tui/views"
)

// BadgeColour, BadgeColourFor, TypeBadge, BlockedBadge, and TypeBadges were
// relocated to internal/tui/views/badge.go (TD.20a): they only ever touched
// string/color.Color values and lipgloss, no tui-package state, so ProseRenderer
// can call them directly instead of drifting with a second copy. Local
// aliases below keep this file's own call sites unchanged.

// BadgeColour holds the foreground and background hex values for a type badge pill.
type BadgeColour = views.BadgeColour

// BadgeColourFor returns the badge colour for a type name.
func BadgeColourFor(typeName string) BadgeColour {
	return views.BadgeColourFor(typeName)
}

// TypeBadge renders a single type name as a coloured pill.
func TypeBadge(typeName string) string {
	return views.TypeBadge(typeName)
}

// BlockedBadge renders a self-contained "BLOCKED" pill.
func BlockedBadge(glyph string) string {
	return views.BlockedBadge(glyph)
}

// TypeBadges renders multiple type names as space-separated pill badges.
func TypeBadges(types []string, containerBg color.Color) string {
	return views.TypeBadges(types, containerBg)
}
