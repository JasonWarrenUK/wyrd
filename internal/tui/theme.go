package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// colourCapability represents the terminal's colour support tier.
type colourCapability int

const (
	capability16       colourCapability = 16
	capability256      colourCapability = 256
	capabilityTruecolor colourCapability = 16777216
)

// detectColourCapability inspects the COLORTERM and TERM environment variables
// to determine the terminal's colour support tier.
func detectColourCapability() colourCapability {
	colorterm := strings.ToLower(os.Getenv("COLORTERM"))
	if colorterm == "truecolor" || colorterm == "24bit" {
		return capabilityTruecolor
	}
	term := strings.ToLower(os.Getenv("TERM"))
	if strings.Contains(term, "256color") {
		return capability256
	}
	return capability16
}

// ActiveTheme wraps a types.Theme and the resolved colour tier, exposing
// convenience methods that return lipgloss.Color values for use in styles.
// All colours are resolved at load time so callers never read raw strings.
type ActiveTheme struct {
	raw  *types.Theme
	tier *types.ColourTier
}

// LoadTheme reads the named theme from storePath/themes/{name}.jsonc and
// resolves the correct colour tier for the current terminal. If the named
// theme cannot be loaded it falls back to the first available theme.
// If no themes exist at all it returns a minimal built-in fallback.
func LoadTheme(storePath, name string) (*ActiveTheme, error) {
	themesDir := filepath.Join(storePath, "themes")

	theme, err := loadThemeFile(filepath.Join(themesDir, name+".jsonc"))
	if err != nil {
		// Attempt to fall back to the first available theme file.
		theme, err = loadFirstTheme(themesDir)
		if err != nil {
			// No theme files present — use a minimal built-in palette.
			theme = builtinTheme()
		}
	}

	cap := detectColourCapability()
	tier := resolveTier(theme, cap)

	return &ActiveTheme{raw: theme, tier: tier}, nil
}

// loadThemeFile reads and parses a single theme JSONC file.
// Comments (// …) are stripped before JSON parsing.
func loadThemeFile(path string) (*types.Theme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read theme %s: %w", path, err)
	}
	stripped := stripComments(data)
	var t types.Theme
	if err := json.Unmarshal(stripped, &t); err != nil {
		return nil, fmt.Errorf("parse theme %s: %w", path, err)
	}
	return &t, nil
}

// loadFirstTheme scans a directory and returns the first parseable theme.
func loadFirstTheme(dir string) (*types.Theme, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read themes dir %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".jsonc") && !strings.HasSuffix(name, ".json") {
			continue
		}
		t, err := loadThemeFile(filepath.Join(dir, name))
		if err == nil {
			return t, nil
		}
	}
	return nil, fmt.Errorf("no valid theme files found in %s", dir)
}

// stripComments removes single-line // comments from JSONC content.
var commentRE = regexp.MustCompile(`(?m)//[^\n]*`)

func stripComments(data []byte) []byte {
	return commentRE.ReplaceAll(data, nil)
}

// resolveTier picks the best available colour tier for the given capability.
// It degrades gracefully if a higher tier is unavailable.
func resolveTier(t *types.Theme, cap colourCapability) *types.ColourTier {
	if cap == capabilityTruecolor && t.Tiers.Truecolor != nil {
		return t.Tiers.Truecolor
	}
	if cap >= capability256 && t.Tiers.C256 != nil {
		return t.Tiers.C256
	}
	if t.Tiers.C16 != nil {
		return t.Tiers.C16
	}
	// Absolute fallback: return whatever tier is set.
	if t.Tiers.Truecolor != nil {
		return t.Tiers.Truecolor
	}
	if t.Tiers.C256 != nil {
		return t.Tiers.C256
	}
	// Return a minimal empty tier rather than nil so callers are safe.
	return &types.ColourTier{}
}

// builtinTheme returns a minimal hardcoded theme used when no theme files
// are present on disk. Colours are conservative ANSI values that work on
// most terminals.
func builtinTheme() *types.Theme {
	tier := &types.ColourTier{
		BG:        types.ColourBG{Primary: "#1a1a2e", Secondary: "#16213e"},
		FG:        types.ColourFG{Primary: "#e0e0e0", Muted: "#888888"},
		Accent:    types.ColourAccent{Primary: "#7c3aed", Secondary: "#a855f7"},
		Border:    "#444466",
		Selection: "#3d3a6e",
		StatusBar: "#0f3460",
		Energy:    types.ColourEnergy{Deep: "#22c55e", Medium: "#eab308", Low: "#ef4444"},
		Overflow:  types.ColourOverflow{Warn: "#f97316", Critical: "#ef4444"},
		Budget:    types.ColourBudget{OK: "#22c55e", Caution: "#eab308", Over: "#ef4444"},
	}
	return &types.Theme{
		Name: "builtin",
		Tiers: types.ThemeTiers{
			Truecolor: tier,
		},
		Glyphs: types.ThemeGlyphs{
			EnergyDeep:    "●",
			EnergyMedium:  "◑",
			EnergyLow:     "○",
			Overflow:      "▲",
			Blocked:       "✖",
			Waiting:       "⧖",
			BudgetOK:      "✓",
			BudgetCaution: "!",
			BudgetOver:    "✗",
		},
	}
}

// --- Colour accessors --------------------------------------------------------
// Each method returns a lipgloss.Color derived from the active tier.
// Methods never return a colour derived from a hardcoded literal.

// BgPrimary returns the primary background colour.
func (a *ActiveTheme) BgPrimary() lipgloss.Color {
	return lipgloss.Color(a.tier.BG.Primary)
}

// BgSecondary returns the secondary background colour.
func (a *ActiveTheme) BgSecondary() lipgloss.Color {
	return lipgloss.Color(a.tier.BG.Secondary)
}

// FgPrimary returns the primary foreground colour.
func (a *ActiveTheme) FgPrimary() lipgloss.Color {
	return lipgloss.Color(a.tier.FG.Primary)
}

// FgMuted returns the muted foreground colour used for hints and placeholders.
func (a *ActiveTheme) FgMuted() lipgloss.Color {
	return lipgloss.Color(a.tier.FG.Muted)
}

// AccentPrimary returns the primary accent colour.
func (a *ActiveTheme) AccentPrimary() lipgloss.Color {
	return lipgloss.Color(a.tier.Accent.Primary)
}

// AccentSecondary returns the secondary accent colour.
func (a *ActiveTheme) AccentSecondary() lipgloss.Color {
	return lipgloss.Color(a.tier.Accent.Secondary)
}

// Border returns the border colour.
func (a *ActiveTheme) Border() lipgloss.Color {
	return lipgloss.Color(a.tier.Border)
}

// Selection returns the selection highlight colour.
func (a *ActiveTheme) Selection() lipgloss.Color {
	return lipgloss.Color(a.tier.Selection)
}

// StatusBar returns the status bar background colour.
func (a *ActiveTheme) StatusBar() lipgloss.Color {
	return lipgloss.Color(a.tier.StatusBar)
}

// EnergyDeep returns the deep-energy indicator colour.
func (a *ActiveTheme) EnergyDeep() lipgloss.Color {
	return lipgloss.Color(a.tier.Energy.Deep)
}

// EnergyMedium returns the medium-energy indicator colour.
func (a *ActiveTheme) EnergyMedium() lipgloss.Color {
	return lipgloss.Color(a.tier.Energy.Medium)
}

// EnergyLow returns the low-energy indicator colour.
func (a *ActiveTheme) EnergyLow() lipgloss.Color {
	return lipgloss.Color(a.tier.Energy.Low)
}

// OverflowWarn returns the schedule-overrun warning colour.
func (a *ActiveTheme) OverflowWarn() lipgloss.Color {
	return lipgloss.Color(a.tier.Overflow.Warn)
}

// OverflowCritical returns the schedule-overrun critical colour.
func (a *ActiveTheme) OverflowCritical() lipgloss.Color {
	return lipgloss.Color(a.tier.Overflow.Critical)
}

// BudgetOK returns the within-budget colour.
func (a *ActiveTheme) BudgetOK() lipgloss.Color {
	return lipgloss.Color(a.tier.Budget.OK)
}

// BudgetCaution returns the near-budget colour.
func (a *ActiveTheme) BudgetCaution() lipgloss.Color {
	return lipgloss.Color(a.tier.Budget.Caution)
}

// BudgetOver returns the over-budget colour.
func (a *ActiveTheme) BudgetOver() lipgloss.Color {
	return lipgloss.Color(a.tier.Budget.Over)
}

// Name returns the display name of the active theme.
func (a *ActiveTheme) Name() string {
	return a.raw.Name
}

// Glyphs returns the glyph set for the active theme.
func (a *ActiveTheme) Glyphs() types.ThemeGlyphs {
	return a.raw.Glyphs
}

// --- Pre-built style helpers -------------------------------------------------
// These compose Lipgloss styles from theme colours so callers stay DRY.

// StylePrimary returns a base style using the primary bg/fg pair.
func (a *ActiveTheme) StylePrimary() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(a.BgPrimary()).
		Foreground(a.FgPrimary())
}

// StyleMuted returns a style using the muted foreground over the primary background.
func (a *ActiveTheme) StyleMuted() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(a.BgPrimary()).
		Foreground(a.FgMuted())
}

// StyleAccent returns a style using the primary accent colour.
func (a *ActiveTheme) StyleAccent() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(a.AccentPrimary())
}

// StyleBorder returns a style with a rounded border using the theme border colour.
func (a *ActiveTheme) StyleBorder() lipgloss.Style {
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(a.Border())
}

// StyleSectionHeader returns the uppercase letter-spaced header style used
// throughout the TUI for section titles.
func (a *ActiveTheme) StyleSectionHeader() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(a.FgMuted()).
		Bold(true)
}

// StyleStatusBar returns the full-width status bar base style.
func (a *ActiveTheme) StyleStatusBar() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(a.StatusBar()).
		Foreground(a.FgPrimary())
}
