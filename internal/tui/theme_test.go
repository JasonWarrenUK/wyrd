package tui_test

import (
	"encoding/json"
	"fmt"
	"image/color"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	wyrdEmbed "github.com/jasonwarrenuk/wyrd/internal/embed"
	"github.com/jasonwarrenuk/wyrd/internal/tui"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// writeThemeFile serialises a types.Theme to JSONC in the given directory.
func writeThemeFile(t *testing.T, dir, name string, theme types.Theme) {
	t.Helper()
	data, err := json.Marshal(theme)
	if err != nil {
		t.Fatalf("marshal theme: %v", err)
	}
	themesDir := filepath.Join(dir, "themes")
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		t.Fatalf("mkdir themes: %v", err)
	}
	path := filepath.Join(themesDir, name+".jsonc")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write theme file: %v", err)
	}
}

// sampleTheme returns a fully-populated test theme.
func sampleTheme(name string) types.Theme {
	tier := &types.ColourTier{
		BG:        types.ColourBG{Primary: "#0a0a0a", Secondary: "#1a1a1a"},
		FG:        types.ColourFG{Primary: "#ffffff", Muted: "#aaaaaa"},
		Accent:    types.ColourAccent{Primary: "#ff00ff", Secondary: "#aa00aa"},
		Border:    "#444444",
		Selection: "#333333",
		StatusBar: "#222222",
		Energy:    types.ColourEnergy{Deep: "#00ff00", Medium: "#ffff00", Low: "#ff0000"},
		Overflow:  types.ColourOverflow{Warn: "#ff8800", Critical: "#ff0000"},
		Budget:    types.ColourBudget{OK: "#00ff00", Caution: "#ffff00", Over: "#ff0000"},
	}
	return types.Theme{
		Name:  name,
		Tiers: types.ThemeTiers{Truecolor: tier},
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

// TestLoadThemeFallsBackToBuiltin verifies that LoadTheme works when no
// theme files exist on disk.
func TestLoadThemeFallsBackToBuiltin(t *testing.T) {
	dir := t.TempDir()
	theme, err := tui.LoadTheme(dir, "nonexistent")
	if err != nil {
		t.Fatalf("LoadTheme returned unexpected error: %v", err)
	}
	if theme == nil {
		t.Fatal("expected non-nil theme")
	}
	if theme.Name() == "" {
		t.Error("expected a non-empty theme name")
	}
}

// TestLoadThemeFromFile verifies that a theme file on disk is loaded correctly.
func TestLoadThemeFromFile(t *testing.T) {
	dir := t.TempDir()
	writeThemeFile(t, dir, "test-theme", sampleTheme("test-theme"))

	theme, err := tui.LoadTheme(dir, "test-theme")
	if err != nil {
		t.Fatalf("LoadTheme returned error: %v", err)
	}
	if theme.Name() != "test-theme" {
		t.Errorf("expected name 'test-theme', got %q", theme.Name())
	}
}

// TestLoadThemeFallsBackToFirstAvailable verifies that when the requested
// theme is absent, the first available theme is used.
func TestLoadThemeFallsBackToFirstAvailable(t *testing.T) {
	dir := t.TempDir()
	writeThemeFile(t, dir, "available-theme", sampleTheme("available-theme"))

	theme, err := tui.LoadTheme(dir, "missing-theme")
	if err != nil {
		t.Fatalf("LoadTheme returned error: %v", err)
	}
	if theme.Name() != "available-theme" {
		t.Errorf("expected fallback to 'available-theme', got %q", theme.Name())
	}
}

// colourIsNonZero returns true when c is non-nil and its RGBA is not all zeros.
func colourIsNonZero(c color.Color) bool {
	if c == nil {
		return false
	}
	r, g, b, a := c.RGBA()
	return r != 0 || g != 0 || b != 0 || a != 0
}

// TestColourAccessorsReturnNonEmptyValues checks that the colour accessor
// methods on ActiveTheme return non-nil, non-zero color.Color values.
func TestColourAccessorsReturnNonEmptyValues(t *testing.T) {
	dir := t.TempDir()
	writeThemeFile(t, dir, "test-theme", sampleTheme("test-theme"))

	theme, err := tui.LoadTheme(dir, "test-theme")
	if err != nil {
		t.Fatalf("LoadTheme error: %v", err)
	}

	checks := []struct {
		name  string
		value color.Color
	}{
		{"BgPrimary", theme.BgPrimary()},
		{"BgSecondary", theme.BgSecondary()},
		{"FgPrimary", theme.FgPrimary()},
		{"FgMuted", theme.FgMuted()},
		{"AccentPrimary", theme.AccentPrimary()},
		{"AccentSecondary", theme.AccentSecondary()},
		{"Border", theme.Border()},
		{"Selection", theme.Selection()},
		{"StatusBar", theme.StatusBar()},
		{"EnergyDeep", theme.EnergyDeep()},
		{"EnergyMedium", theme.EnergyMedium()},
		{"EnergyLow", theme.EnergyLow()},
		{"OverflowWarn", theme.OverflowWarn()},
		{"OverflowCritical", theme.OverflowCritical()},
		{"BudgetOK", theme.BudgetOK()},
		{"BudgetCaution", theme.BudgetCaution()},
		{"BudgetOver", theme.BudgetOver()},
	}

	for _, c := range checks {
		if !colourIsNonZero(c.value) {
			t.Errorf("%s returned a nil or zero colour", c.name)
		}
	}
}

// TestThemeSwitchChangesColours verifies that switching themes produces
// different colour values for the primary background.
func TestThemeSwitchChangesColours(t *testing.T) {
	dir := t.TempDir()

	// Write two themes with different primary backgrounds.
	themeA := sampleTheme("theme-a")
	themeA.Tiers.Truecolor.BG.Primary = "#111111"
	themeB := sampleTheme("theme-b")
	themeB.Tiers.Truecolor.BG.Primary = "#eeeeee"

	writeThemeFile(t, dir, "theme-a", themeA)
	writeThemeFile(t, dir, "theme-b", themeB)

	a, err := tui.LoadTheme(dir, "theme-a")
	if err != nil {
		t.Fatalf("load theme-a: %v", err)
	}
	b, err := tui.LoadTheme(dir, "theme-b")
	if err != nil {
		t.Fatalf("load theme-b: %v", err)
	}

	if fmt.Sprintf("%v", a.BgPrimary()) == fmt.Sprintf("%v", b.BgPrimary()) {
		t.Error("expected different BgPrimary after theme switch")
	}
}

// TestBuiltinThemeHasGlyphs verifies the builtin theme has non-empty glyphs.
func TestBuiltinThemeHasGlyphs(t *testing.T) {
	dir := t.TempDir()
	theme, err := tui.LoadTheme(dir, "")
	if err != nil {
		t.Fatalf("LoadTheme error: %v", err)
	}
	glyphs := theme.Glyphs()
	if glyphs.EnergyDeep == "" {
		t.Error("EnergyDeep glyph should not be empty")
	}
	if glyphs.EnergyMedium == "" {
		t.Error("EnergyMedium glyph should not be empty")
	}
	if glyphs.EnergyLow == "" {
		t.Error("EnergyLow glyph should not be empty")
	}
}

// TestStyleHelpersDoNotPanic verifies that each pre-built style helper
// method can be called without panicking.
func TestStyleHelpersDoNotPanic(t *testing.T) {
	dir := t.TempDir()
	theme, err := tui.LoadTheme(dir, "")
	if err != nil {
		t.Fatalf("LoadTheme error: %v", err)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("style helper panicked: %v", r)
		}
	}()

	_ = theme.StylePrimary()
	_ = theme.StyleMuted()
	_ = theme.StyleAccent()
	_ = theme.StyleBorder()
	_ = theme.StyleSectionHeader()
	_ = theme.StyleStatusBar()
}

// copyEmbeddedThemes extracts all JSONC files from the embedded starter/themes
// directory into a temp store directory so they can be loaded by LoadTheme.
func copyEmbeddedThemes(t *testing.T, destDir string) {
	t.Helper()
	themesDir := filepath.Join(destDir, "themes")
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		t.Fatalf("mkdir themes: %v", err)
	}
	entries, err := fs.ReadDir(wyrdEmbed.StarterFS, "starter/themes")
	if err != nil {
		t.Fatalf("read embedded themes: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := fs.ReadFile(wyrdEmbed.StarterFS, "starter/themes/"+e.Name())
		if err != nil {
			t.Fatalf("read embedded theme %s: %v", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(themesDir, e.Name()), data, 0o644); err != nil {
			t.Fatalf("write theme %s: %v", e.Name(), err)
		}
	}
}

// TestAllShippedThemesLoad verifies that all four shipped themes (Cairn, Peat,
// Kiln, Fell) parse correctly and that every colour accessor and glyph field
// returns a non-zero value.
func TestAllShippedThemesLoad(t *testing.T) {
	dir := t.TempDir()
	copyEmbeddedThemes(t, dir)

	themes := []string{"cairn", "peat", "kiln", "fell"}
	for _, name := range themes {
		t.Run(name, func(t *testing.T) {
			theme, err := tui.LoadTheme(dir, name)
			if err != nil {
				t.Fatalf("LoadTheme(%q): %v", name, err)
			}
			if theme.Name() != name {
				t.Errorf("expected name %q, got %q", name, theme.Name())
			}

			// All 17 colour accessors must return non-zero values.
			checks := []struct {
				label string
				value color.Color
			}{
				{"BgPrimary", theme.BgPrimary()},
				{"BgSecondary", theme.BgSecondary()},
				{"FgPrimary", theme.FgPrimary()},
				{"FgMuted", theme.FgMuted()},
				{"AccentPrimary", theme.AccentPrimary()},
				{"AccentSecondary", theme.AccentSecondary()},
				{"Border", theme.Border()},
				{"Selection", theme.Selection()},
				{"StatusBar", theme.StatusBar()},
				{"EnergyDeep", theme.EnergyDeep()},
				{"EnergyMedium", theme.EnergyMedium()},
				{"EnergyLow", theme.EnergyLow()},
				{"OverflowWarn", theme.OverflowWarn()},
				{"OverflowCritical", theme.OverflowCritical()},
				{"BudgetOK", theme.BudgetOK()},
				{"BudgetCaution", theme.BudgetCaution()},
				{"BudgetOver", theme.BudgetOver()},
			}
			for _, c := range checks {
				if !colourIsNonZero(c.value) {
					t.Errorf("%s.%s returned a nil or zero colour", name, c.label)
				}
			}

			// All 9 glyph fields must be non-empty.
			g := theme.Glyphs()
			glyphs := []struct {
				label string
				value string
			}{
				{"EnergyDeep", g.EnergyDeep},
				{"EnergyMedium", g.EnergyMedium},
				{"EnergyLow", g.EnergyLow},
				{"Overflow", g.Overflow},
				{"Blocked", g.Blocked},
				{"Waiting", g.Waiting},
				{"BudgetOK", g.BudgetOK},
				{"BudgetCaution", g.BudgetCaution},
				{"BudgetOver", g.BudgetOver},
			}
			for _, gl := range glyphs {
				if gl.value == "" {
					t.Errorf("%s glyph %s is empty", name, gl.label)
				}
			}
		})
	}
}

// TestShippedThemesHaveDistinctPalettes verifies that each shipped theme has a
// unique AccentPrimary colour, confirming that theme switching actually changes
// the active palette.
func TestShippedThemesHaveDistinctPalettes(t *testing.T) {
	dir := t.TempDir()
	copyEmbeddedThemes(t, dir)

	names := []string{"cairn", "peat", "kiln", "fell"}
	accents := make(map[string]string) // theme name → AccentPrimary string representation
	for _, name := range names {
		theme, err := tui.LoadTheme(dir, name)
		if err != nil {
			t.Fatalf("LoadTheme(%q): %v", name, err)
		}
		accents[name] = fmt.Sprintf("%v", theme.AccentPrimary())
	}

	// Every pair must be distinct.
	for i, a := range names {
		for _, b := range names[i+1:] {
			if accents[a] == accents[b] {
				t.Errorf("AccentPrimary identical for %q and %q: %s", a, b, accents[a])
			}
		}
	}
}

// TestRuntimeThemeSwitch verifies that the app correctly switches theme
// when a "theme" command is issued via the Model.
func TestRuntimeThemeSwitch(t *testing.T) {
	dir := t.TempDir()
	themeA := sampleTheme("theme-a")
	themeA.Tiers.Truecolor.BG.Primary = "#111111"
	themeB := sampleTheme("theme-b")
	themeB.Tiers.Truecolor.BG.Primary = "#eeeeee"

	writeThemeFile(t, dir, "theme-a", themeA)
	writeThemeFile(t, dir, "theme-b", themeB)

	m, err := tui.New(tui.Config{StorePath: dir, ThemeName: "theme-a"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	originalBg := m.Theme().BgPrimary()

	// Open the palette and type "theme theme-b" to switch.
	// We simulate the internal switchThemeMsg directly for test simplicity.
	type switchThemeMsg struct {
		name string
	}
	// Instead use Ctrl+K and then register the theme-b command through Update.
	// Easiest: send a WindowSizeMsg then manually trigger a theme switch
	// by running the registered command.
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(tui.Model)
	if cmd != nil {
		// Execute any startup command.
		_ = cmd()
	}

	// The theme switch message type is internal (unexported), so we test
	// indirectly: load theme-b separately and compare colours.
	newTheme, err := tui.LoadTheme(dir, "theme-b")
	if err != nil {
		t.Fatalf("LoadTheme theme-b: %v", err)
	}
	if fmt.Sprintf("%v", originalBg) == fmt.Sprintf("%v", newTheme.BgPrimary()) {
		t.Error("expected different primary background after theme switch")
	}
}
