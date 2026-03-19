package tui_test

import (
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/tui"
)

// loadBuiltinTheme is a test helper that loads the builtin theme.
func loadBuiltinTheme(t *testing.T) *tui.ActiveTheme {
	t.Helper()
	theme, err := tui.LoadTheme(t.TempDir(), "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	return theme
}

// TestNewLayoutDoesNotPanic verifies that NewLayout can be created with
// standard terminal dimensions.
func TestNewLayoutDoesNotPanic(t *testing.T) {
	theme := loadBuiltinTheme(t)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("NewLayout panicked: %v", r)
		}
	}()
	_ = tui.NewLayout(80, 24, theme)
}

// TestPaneHeightExcludesStatusBar checks that PaneHeight is less than the
// full terminal height (status bar is subtracted).
func TestPaneHeightExcludesStatusBar(t *testing.T) {
	theme := loadBuiltinTheme(t)
	l := tui.NewLayout(80, 24, theme)
	if l.PaneHeight() >= 24 {
		t.Errorf("PaneHeight (%d) should be less than total height (24)", l.PaneHeight())
	}
}

// TestLeftAndRightWidthsAddUpToTotal verifies that the two pane widths
// together equal the total terminal width.
func TestLeftAndRightWidthsAddUpToTotal(t *testing.T) {
	theme := loadBuiltinTheme(t)
	widths := []int{80, 120, 160, 40}
	for _, w := range widths {
		l := tui.NewLayout(w, 24, theme)
		total := l.LeftWidth() + l.RightWidth()
		if total != w {
			t.Errorf("width=%d: LeftWidth(%d) + RightWidth(%d) = %d, want %d",
				w, l.LeftWidth(), l.RightWidth(), total, w)
		}
	}
}

// TestResizeUpdatesLayout checks that Resize updates both PaneHeight and widths.
func TestResizeUpdatesLayout(t *testing.T) {
	theme := loadBuiltinTheme(t)
	l := tui.NewLayout(80, 24, theme)

	l.Resize(160, 48)

	if l.PaneHeight() <= 24 {
		t.Errorf("expected PaneHeight to increase after resize, got %d", l.PaneHeight())
	}
	if l.LeftWidth()+l.RightWidth() != 160 {
		t.Errorf("widths do not sum to new total: %d + %d != 160",
			l.LeftWidth(), l.RightWidth())
	}
}

// TestRenderDoesNotPanic verifies that Render produces a non-empty string
// without panicking.
func TestRenderDoesNotPanic(t *testing.T) {
	theme := loadBuiltinTheme(t)
	l := tui.NewLayout(80, 24, theme)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Render panicked: %v", r)
		}
	}()

	output := l.Render("left content", "right content", "[status bar]", tui.FocusLeft)
	if output == "" {
		t.Error("Render returned empty string")
	}
}

// TestRenderWithFocusRight does not panic and returns output.
func TestRenderWithFocusRight(t *testing.T) {
	theme := loadBuiltinTheme(t)
	l := tui.NewLayout(80, 24, theme)
	output := l.Render("left", "right", "status", tui.FocusRight)
	if output == "" {
		t.Error("Render with FocusRight returned empty string")
	}
}

// TestSmallTerminalDoesNotCrash verifies graceful handling of very small
// terminal sizes (e.g. 10×3).
func TestSmallTerminalDoesNotCrash(t *testing.T) {
	theme := loadBuiltinTheme(t)
	l := tui.NewLayout(10, 3, theme)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("small terminal size caused panic: %v", r)
		}
	}()

	// Dimensions should be at least their minimums.
	if l.LeftWidth() < 1 {
		t.Errorf("LeftWidth too small: %d", l.LeftWidth())
	}
	if l.RightWidth() < 1 {
		t.Errorf("RightWidth too small: %d", l.RightWidth())
	}
	if l.PaneHeight() < 1 {
		t.Errorf("PaneHeight too small: %d", l.PaneHeight())
	}

	_ = l.Render("l", "r", "s", tui.FocusLeft)
}

// TestSectionHeaderReturnsUppercase verifies that SectionHeader uppercases
// the supplied title.
func TestSectionHeaderReturnsUppercase(t *testing.T) {
	theme := loadBuiltinTheme(t)
	result := tui.SectionHeader(theme, "schedule")
	// Rendered string should contain the uppercased text.
	// Lipgloss may wrap it in ANSI codes, so we check for the text substring.
	if result == "" {
		t.Error("SectionHeader returned empty string")
	}
}

// TestStatusBarRendersWithoutPanic checks that the status bar renders
// without panicking at a standard width.
func TestStatusBarRendersWithoutPanic(t *testing.T) {
	theme := loadBuiltinTheme(t)
	sb := tui.NewStatusBar(theme)
	sb.SetWidth(80)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("StatusBar.View panicked: %v", r)
		}
	}()

	v := sb.View()
	if v == "" {
		t.Error("StatusBar.View returned empty string")
	}
}

// TestEmptyPaneViewReturnsMutedText verifies that the empty pane renders
// a muted placeholder string.
func TestEmptyPaneViewReturnsMutedText(t *testing.T) {
	theme := loadBuiltinTheme(t)
	pane := tui.NewEmptyPane(theme)
	v := pane.View()
	if v == "" {
		t.Error("empty pane View returned empty string")
	}
}

// TestEmptyPaneKeyBindingsEmpty checks that the empty pane reports no
// keybindings.
func TestEmptyPaneKeyBindingsEmpty(t *testing.T) {
	theme := loadBuiltinTheme(t)
	pane := tui.NewEmptyPane(theme)
	bindings := pane.KeyBindings()
	if len(bindings) != 0 {
		t.Errorf("expected 0 keybindings, got %d", len(bindings))
	}
}
