package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestPadLines_ShortLinePadded(t *testing.T) {
	result := PadLines("hi", 10, "#1a1a2e")
	// Visible content must start with "hi"
	if !strings.HasPrefix(stripANSI(result), "hi") {
		t.Errorf("expected result to start with 'hi', got: %q", stripANSI(result))
	}
	// Rendered width should be exactly 10 runes (lipgloss.Width counts visible cols)
	if w := lipgloss.Width(result); w != 10 {
		t.Errorf("expected width 10, got %d", w)
	}
}

func TestPadLines_ExactWidthUnchangedVisually(t *testing.T) {
	result := PadLines("hello", 5, "#1a1a2e")
	if w := lipgloss.Width(result); w != 5 {
		t.Errorf("expected width 5, got %d", w)
	}
}

func TestPadLines_MultiLine(t *testing.T) {
	result := PadLines("line one\nline two", 12, "#1a1a2e")
	lines := strings.Split(result, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w != 12 {
			t.Errorf("line %d: expected width 12, got %d", i, w)
		}
	}
}

func TestPadLines_EmptyContentPassThrough(t *testing.T) {
	result := PadLines("", 10, "#1a1a2e")
	if result != "" {
		t.Errorf("expected empty string, got: %q", result)
	}
}

func TestPadLines_ZeroWidthPassThrough(t *testing.T) {
	result := PadLines("hello", 0, "#1a1a2e")
	if result != "hello" {
		t.Errorf("expected unmodified 'hello', got: %q", result)
	}
}

func TestSpacer_WidthIsExactlyN(t *testing.T) {
	result := Spacer(3, lipgloss.Color("#1a1a2e"))
	if w := lipgloss.Width(result); w != 3 {
		t.Errorf("expected Spacer width 3, got %d (result: %q)", w, result)
	}
}

func TestSpacer_ZeroReturnsEmpty(t *testing.T) {
	result := Spacer(0, lipgloss.Color("#1a1a2e"))
	if result != "" {
		t.Errorf("expected empty string for n=0, got: %q", result)
	}
}

func TestSpacer_NegativeReturnsEmpty(t *testing.T) {
	result := Spacer(-1, lipgloss.Color("#1a1a2e"))
	if result != "" {
		t.Errorf("expected empty string for n=-1, got: %q", result)
	}
}

func TestPadLines_OutputWidthIsAlwaysExact(t *testing.T) {
	// Regardless of whether ANSI colour codes are emitted (they may be
	// suppressed in non-TTY test environments), lipgloss.Width must report
	// exactly the requested width.
	result := PadLines("hi", 10, "#1a1a2e")
	if w := lipgloss.Width(result); w != 10 {
		t.Errorf("expected lipgloss.Width 10, got %d (result: %q)", w, result)
	}
}

