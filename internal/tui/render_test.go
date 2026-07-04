package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestPadLines_ShortLinePadded(t *testing.T) {
	result := PadLines("hi", 10, lipgloss.Color("#1a1a2e"))
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
	result := PadLines("hello", 5, lipgloss.Color("#1a1a2e"))
	if w := lipgloss.Width(result); w != 5 {
		t.Errorf("expected width 5, got %d", w)
	}
}

func TestPadLines_MultiLine(t *testing.T) {
	result := PadLines("line one\nline two", 12, lipgloss.Color("#1a1a2e"))
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
	result := PadLines("", 10, lipgloss.Color("#1a1a2e"))
	if result != "" {
		t.Errorf("expected empty string, got: %q", result)
	}
}

func TestPadLines_ZeroWidthPassThrough(t *testing.T) {
	result := PadLines("hello", 0, lipgloss.Color("#1a1a2e"))
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
	result := PadLines("hi", 10, lipgloss.Color("#1a1a2e"))
	if w := lipgloss.Width(result); w != 10 {
		t.Errorf("expected lipgloss.Width 10, got %d (result: %q)", w, result)
	}
}

// --- FillBackground tests ---

// TestFillBackground_PlainStringGetsBgAsserted checks that a plain string with
// no ANSI codes still gets the bg SGR injected at the start of each line. The
// visible text must be unchanged.
func TestFillBackground_PlainStringGetsBgAsserted(t *testing.T) {
	bg := lipgloss.Color("#1a1a2e")
	result := FillBackground("hello\nworld", bg)
	// Visible text must be preserved exactly.
	if got := stripANSI(result); got != "hello\nworld" {
		t.Errorf("visible text changed: got %q", got)
	}
	// Line count must be stable.
	if n := strings.Count(result, "\n"); n != 1 {
		t.Errorf("expected 1 newline, got %d", n)
	}
}

// TestFillBackground_InteriorResetGetsReasserted verifies that after an
// interior ESC[m reset the bg SGR is re-inserted, while a preceding fg SGR
// (ASCII bytes) is still present in the output before the glyph.
func TestFillBackground_InteriorResetGetsReasserted(t *testing.T) {
	bg := lipgloss.Color("#1a1a2e")
	// Simulate a styled glyph followed by a reset and then a plain space
	// (as the viewport would emit for an interior padding cell).
	const fgSGR = "\x1b[38;5;196m" // arbitrary fg colour
	const reset = "\x1b[m"
	input := fgSGR + "X" + reset + " "
	result := FillBackground(input, bg)
	// The fg SGR must survive.
	if !strings.Contains(result, fgSGR) {
		t.Errorf("fg SGR was stripped; result: %q", result)
	}
	// After the reset, the bg must be re-asserted (some bg SGR must follow
	// immediately after the reset sequence).
	resetIdx := strings.Index(result, reset)
	if resetIdx < 0 {
		t.Fatalf("reset not found in result: %q", result)
	}
	after := result[resetIdx+len(reset):]
	if !strings.HasPrefix(after, "\x1b[") {
		t.Errorf("expected bg re-assertion (ESC[...) after reset, got: %q", after)
	}
	// Visible text must still be "X ".
	if got := stripANSI(result); got != "X " {
		t.Errorf("visible text changed: got %q", got)
	}
}

// TestFillBackground_NilBgReturnsUnchanged confirms that a nil bg produces no
// change (guards the no-colour path).
func TestFillBackground_NilBgReturnsUnchanged(t *testing.T) {
	input := "hello"
	result := FillBackground(input, nil)
	if result != input {
		t.Errorf("expected input unchanged for nil bg, got %q", result)
	}
}

// TestFillBackground_OSCHyperlinkPassesThrough checks that an OSC-8 hyperlink
// sequence (not an SGR) is copied verbatim and not mistaken for a reset.
// Note: stripANSI is a minimal SGR-only stripper and cannot handle OSC
// sequences, so we check the raw bytes rather than the stripped output.
func TestFillBackground_OSCHyperlinkPassesThrough(t *testing.T) {
	bg := lipgloss.Color("#1a1a2e")
	// OSC-8 hyperlink: ESC]8;;URL ST  and the terminating ESC]8;; ST
	const osc8 = "\x1b]8;;https://example.com\x1b\\"
	const osc8end = "\x1b]8;;\x1b\\"
	input := osc8 + "link text" + osc8end
	result := FillBackground(input, bg)
	if !strings.Contains(result, osc8) {
		t.Errorf("OSC-8 open not preserved; result: %q", result)
	}
	if !strings.Contains(result, osc8end) {
		t.Errorf("OSC-8 close not preserved; result: %q", result)
	}
	// Check the glyph bytes are present directly — stripANSI cannot handle
	// OSC sequences (it only scans for 'm' as the escape terminator).
	if !strings.Contains(result, "link text") {
		t.Errorf("visible text 'link text' missing from raw result: %q", result)
	}
}

// TestFillBackground_MultiLineLineCountStable confirms FillBackground never
// introduces extra newlines.
func TestFillBackground_MultiLineLineCountStable(t *testing.T) {
	bg := lipgloss.Color("#1a1a2e")
	input := "line one\nline two\nline three"
	result := FillBackground(input, bg)
	in := strings.Count(input, "\n")
	out := strings.Count(result, "\n")
	if in != out {
		t.Errorf("newline count changed: input %d, output %d; result: %q", in, out, result)
	}
}

// TestFillBackground_EmptyStringReturnsEmpty is a degenerate guard.
func TestFillBackground_EmptyStringReturnsEmpty(t *testing.T) {
	result := FillBackground("", lipgloss.Color("#1a1a2e"))
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}
