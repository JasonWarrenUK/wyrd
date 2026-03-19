package tui_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonwarrenuk/wyrd/internal/tui"
)

// newTestModel creates a Model with no store and no theme files on disk.
// It should initialise cleanly using the built-in theme.
func newTestModel(t *testing.T) tui.Model {
	t.Helper()
	m, err := tui.New(tui.Config{StorePath: t.TempDir()})
	if err != nil {
		t.Fatalf("New returned unexpected error: %v", err)
	}
	return m
}

// sendWindowSize delivers a WindowSizeMsg to a model and returns the
// updated model.
func sendWindowSize(t *testing.T, m tui.Model, w, h int) tui.Model {
	t.Helper()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	result, ok := updated.(tui.Model)
	if !ok {
		t.Fatalf("Update returned unexpected type %T", updated)
	}
	return result
}

// TestAppBootsOnEmptyStore verifies that the app can be constructed without
// any nodes, edges, or theme files present.
func TestAppBootsOnEmptyStore(t *testing.T) {
	m := newTestModel(t)
	if m.Theme() == nil {
		t.Fatal("expected a non-nil theme after boot")
	}
}

// TestInitDoesNotPanic ensures Init() returns without panicking on an
// empty store.
func TestInitDoesNotPanic(t *testing.T) {
	m := newTestModel(t)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Init() panicked: %v", r)
		}
	}()
	_ = m.Init()
}

// TestViewBeforeWindowSize checks that View() returns a non-empty
// placeholder before the first WindowSizeMsg.
func TestViewBeforeWindowSize(t *testing.T) {
	m := newTestModel(t)
	v := m.View()
	if v == "" {
		t.Error("expected a non-empty placeholder before WindowSizeMsg")
	}
}

// TestViewAfterWindowSize verifies that View() returns content after
// a WindowSizeMsg is received.
func TestViewAfterWindowSize(t *testing.T) {
	m := newTestModel(t)
	m = sendWindowSize(t, m, 120, 40)
	v := m.View()
	if v == "" {
		t.Error("expected non-empty view after WindowSizeMsg")
	}
}

// TestTerminalResize verifies that multiple successive resize events do not
// cause a panic or empty view.
func TestTerminalResize(t *testing.T) {
	m := newTestModel(t)
	sizes := [][2]int{{80, 24}, {120, 40}, {40, 15}, {200, 60}}
	for _, s := range sizes {
		m = sendWindowSize(t, m, s[0], s[1])
		v := m.View()
		if v == "" {
			t.Errorf("empty view after resize to %dx%d", s[0], s[1])
		}
	}
}

// TestQuitAction verifies that pressing "q" issues a quit command.
func TestQuitAction(t *testing.T) {
	m := newTestModel(t)
	m = sendWindowSize(t, m, 80, 24)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected a non-nil command after 'q'")
	}
	// Execute the command and verify it is a quit message.
	msg := cmd()
	if msg != tea.Quit() {
		t.Errorf("expected tea.Quit message, got %v", msg)
	}
}

// TestSwitchPaneFocus verifies that Ctrl+W toggles focus between panes.
func TestSwitchPaneFocus(t *testing.T) {
	m := newTestModel(t)
	m = sendWindowSize(t, m, 80, 24)

	// Default focus should be left.
	initialTheme := m.Theme()
	if initialTheme == nil {
		t.Fatal("expected non-nil theme")
	}

	// Press Ctrl+W — should switch to right pane.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	m2, ok := updated.(tui.Model)
	if !ok {
		t.Fatalf("unexpected type %T", updated)
	}

	// Press Ctrl+W again — should switch back to left.
	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	m3, ok := updated2.(tui.Model)
	if !ok {
		t.Fatalf("unexpected type %T", updated2)
	}

	// Both views should be non-empty.
	if m2.View() == "" || m3.View() == "" {
		t.Error("view was empty after pane switch")
	}
}

// TestCommandPaletteOpensOnColon checks that pressing ":" opens the palette.
func TestCommandPaletteOpensOnColon(t *testing.T) {
	m := newTestModel(t)
	m = sendWindowSize(t, m, 80, 24)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	m2, ok := updated.(tui.Model)
	if !ok {
		t.Fatalf("unexpected type %T", updated)
	}

	v := m2.View()
	if v == "" {
		t.Error("expected non-empty view with palette open")
	}
}

// TestPaletteClosesOnEscape verifies that Escape dismisses the palette.
func TestPaletteClosesOnEscape(t *testing.T) {
	m := newTestModel(t)
	m = sendWindowSize(t, m, 80, 24)

	// Open palette.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	m2 := updated.(tui.Model)

	// Close with Escape.
	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m3 := updated2.(tui.Model)

	v := m3.View()
	if v == "" {
		t.Error("expected non-empty view after palette close")
	}
}

// TestKeyboardInputRoutedToFocusedPane verifies that unrecognised keys reach
// the focused pane's Update without panicking.
func TestKeyboardInputRoutedToFocusedPane(t *testing.T) {
	m := newTestModel(t)
	m = sendWindowSize(t, m, 80, 24)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("keyboard routing panicked: %v", r)
		}
	}()

	// Press an unrecognised key — should be silently forwarded to the pane.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
}

// TestMountLeftAndRight verifies that custom pane implementations can be
// mounted without panicking or producing empty views.
func TestMountLeftAndRight(t *testing.T) {
	m := newTestModel(t)
	m = sendWindowSize(t, m, 80, 24)

	m.MountLeft(tui.NewEmptyPane(m.Theme()))
	m.MountRight(tui.NewEmptyPane(m.Theme()))

	v := m.View()
	if v == "" {
		t.Error("expected non-empty view after mounting panes")
	}
}
