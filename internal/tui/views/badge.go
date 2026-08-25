package views

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// BadgeColour holds the foreground and background hex values for a type badge pill.
type BadgeColour struct {
	BG string
	FG string
}

// badgePalette maps canonical node types to their badge colours.
// All values are from Reasonable Colors; FG is always gray-1 (#f6f6f6)
// for reliable contrast against the mid-tone badge backgrounds.
var badgePalette = map[string]BadgeColour{
	"task":    {BG: "#794aff", FG: "#f6f6f6"}, // violet-4
	"note":    {BG: "#009e8c", FG: "#f6f6f6"}, // teal-3
	"journal": {BG: "#b98300", FG: "#f6f6f6"}, // amber-3
	"budget":  {BG: "#65a30d", FG: "#f6f6f6"}, // green
	"project": {BG: "#cd3c00", FG: "#f6f6f6"}, // orange-4
	"person":  {BG: "#82002c", FG: "#f6f6f6"}, // raspberry-5
}

// fallbackBadgePalette provides colours for unknown/user-defined types.
// Index is chosen by a deterministic hash of the type name so the same
// type always gets the same colour across renders.
var fallbackBadgePalette = []BadgeColour{
	{BG: "#794aff", FG: "#f6f6f6"}, // violet-4
	{BG: "#009e8c", FG: "#f6f6f6"}, // teal-3
	{BG: "#b98300", FG: "#f6f6f6"}, // amber-3
	{BG: "#65a30d", FG: "#f6f6f6"}, // green
	{BG: "#cd3c00", FG: "#f6f6f6"}, // orange-4
	{BG: "#82002c", FG: "#f6f6f6"}, // raspberry-5
	{BG: "#0066cc", FG: "#f6f6f6"}, // azure-4
	{BG: "#9b0062", FG: "#f6f6f6"}, // magenta-5
}

// BadgeColourFor returns the badge colour for a type name. Known types use the
// fixed palette; unknown types are assigned deterministically via a byte-sum hash.
func BadgeColourFor(typeName string) BadgeColour {
	if c, ok := badgePalette[typeName]; ok {
		return c
	}
	var sum int
	for _, b := range []byte(typeName) {
		sum += int(b)
	}
	return fallbackBadgePalette[sum%len(fallbackBadgePalette)]
}

// TypeBadge renders a single type name as a coloured pill: ` typeName `
// with the badge's own background and foreground. The badge is self-contained
// and readable on any container background.
func TypeBadge(typeName string) string {
	c := BadgeColourFor(typeName)
	return lipgloss.NewStyle().
		Background(lipgloss.Color(c.BG)).
		Foreground(lipgloss.Color(c.FG)).
		Padding(0, 1).
		Render(typeName)
}

// blockedBadgeColour is the fixed pill colour for BlockedBadge — raspberry-5
// from Reasonable Colors, matching the badgePalette entries' FG convention.
var blockedBadgeColour = BadgeColour{BG: "#82002c", FG: "#f6f6f6"}

// BlockedBadge renders a self-contained "BLOCKED" pill, prefixed with glyph
// (typically theme.Glyphs().Blocked, default "✖") so it respects theme
// overrides. Like TypeBadge, it carries its own background and foreground so
// it is bleed-safe on any container background.
func BlockedBadge(glyph string) string {
	label := "BLOCKED"
	if glyph != "" {
		label = glyph + " " + label
	}
	return lipgloss.NewStyle().
		Background(lipgloss.Color(blockedBadgeColour.BG)).
		Foreground(lipgloss.Color(blockedBadgeColour.FG)).
		Padding(0, 1).
		Bold(true).
		Render(label)
}

// TypeBadges renders multiple type names as space-separated pill badges.
// containerBg is the background colour of the surrounding element — used for
// the spacer between badges so there is no bleed at badge boundaries.
// Returns an empty string when types is empty.
func TypeBadges(types []string, containerBg color.Color) string {
	if len(types) == 0 {
		return ""
	}
	if len(types) == 1 {
		return TypeBadge(types[0])
	}
	parts := make([]string, len(types))
	for i, t := range types {
		parts[i] = TypeBadge(t)
	}
	sep := badgeSpacer(1, containerBg)
	result := parts[0]
	for _, p := range parts[1:] {
		result += sep + p
	}
	return result
}

// badgeSpacer returns n spaces styled with bg, mirroring package tui's
// Spacer helper (internal/tui/render.go). Duplicated here rather than
// imported since package views cannot import package tui (one-directional
// import boundary; see CLAUDE.md), and this is the only place in views that
// needs a background-safe spacer.
func badgeSpacer(n int, bg color.Color) string {
	if n <= 0 {
		return ""
	}
	spaces := make([]byte, n)
	for i := range spaces {
		spaces[i] = ' '
	}
	return lipgloss.NewStyle().Background(bg).Render(string(spaces))
}
