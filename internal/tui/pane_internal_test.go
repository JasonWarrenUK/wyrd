package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/mattn/go-runewidth"
)

// newTestEmptyPane builds an emptyPane with the built-in fallback theme,
// sized via a WindowSizeMsg so PadLines has a real width to work with.
func newTestEmptyPane(t *testing.T, termWidth, termHeight int) PaneModel {
	t.Helper()
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	p := NewEmptyPane(theme)
	p, _ = p.Update(tea.WindowSizeMsg{Width: termWidth, Height: termHeight})
	return p
}

func TestEmptyPane_ViewPadsToPaneWidth(t *testing.T) {
	p := newTestEmptyPane(t, 80, 24)
	out := p.View()

	// viewportPane's width calculation: 80/2 - 2 = 38 printable cells.
	want := 80/2 - 2
	got := runewidth.StringWidth(stripANSI(out))
	if got != want {
		t.Errorf("expected padded width %d, got %d (output %q)", want, got, out)
	}
}

func TestEmptyPane_PaddingCarriesThemeBackground(t *testing.T) {
	p := newTestEmptyPane(t, 80, 24)
	out := p.View()

	// The padding spaces after the "No content" text must carry a truecolour
	// background — a trailing run of bare spaces after an ANSI reset is
	// exactly the bleed PadLines exists to prevent.
	if !ansiTruecolourBgRe.MatchString(out) {
		t.Fatalf("expected a truecolour background in output, got %q", out)
	}
	if strings.HasSuffix(stripANSI(out), " ") {
		// Padding exists; verify it is not emitted as bare characters at the
		// very end of the string (i.e. after the final SGR reset).
		if strings.HasSuffix(out, " ") {
			t.Errorf("padding spaces trail the final ANSI sequence unstyled: %q", out)
		}
	}
}

func TestEmptyPane_ZeroWidthBeforeFirstResize(t *testing.T) {
	// Before any WindowSizeMsg the pane has no width; View must still return
	// the placeholder rather than panicking or padding to a negative width.
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	p := NewEmptyPane(theme)
	out := stripANSI(p.View())
	if !strings.Contains(out, "No content") {
		t.Errorf("expected placeholder text, got %q", out)
	}
}
