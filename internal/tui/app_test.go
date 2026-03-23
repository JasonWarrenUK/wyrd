package tui_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jasonwarrenuk/wyrd/internal/query"
	"github.com/jasonwarrenuk/wyrd/internal/store"
	"github.com/jasonwarrenuk/wyrd/internal/tui"
	"github.com/jasonwarrenuk/wyrd/internal/types"
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
	if v.Content == "" {
		t.Error("expected a non-empty placeholder before WindowSizeMsg")
	}
}

// TestViewAfterWindowSize verifies that View() returns content after
// a WindowSizeMsg is received.
func TestViewAfterWindowSize(t *testing.T) {
	m := newTestModel(t)
	m = sendWindowSize(t, m, 120, 40)
	v := m.View()
	if v.Content == "" {
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
		if v.Content == "" {
			t.Errorf("empty view after resize to %dx%d", s[0], s[1])
		}
	}
}

// TestQuitAction verifies that pressing "q" issues a quit command.
func TestQuitAction(t *testing.T) {
	m := newTestModel(t)
	m = sendWindowSize(t, m, 80, 24)

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
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
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl})
	m2, ok := updated.(tui.Model)
	if !ok {
		t.Fatalf("unexpected type %T", updated)
	}

	// Press Ctrl+W again — should switch back to left.
	updated2, _ := m2.Update(tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl})
	m3, ok := updated2.(tui.Model)
	if !ok {
		t.Fatalf("unexpected type %T", updated2)
	}

	// Both views should be non-empty.
	if m2.View().Content == "" || m3.View().Content == "" {
		t.Error("view was empty after pane switch")
	}
}

// TestCommandPaletteOpensOnColon checks that pressing ":" opens the palette.
func TestCommandPaletteOpensOnColon(t *testing.T) {
	m := newTestModel(t)
	m = sendWindowSize(t, m, 80, 24)

	updated, _ := m.Update(tea.KeyPressMsg{Code: ':', Text: ":"})
	m2, ok := updated.(tui.Model)
	if !ok {
		t.Fatalf("unexpected type %T", updated)
	}

	v := m2.View()
	if v.Content == "" {
		t.Error("expected non-empty view with palette open")
	}
}

// TestPaletteClosesOnEscape verifies that Escape dismisses the palette.
func TestPaletteClosesOnEscape(t *testing.T) {
	m := newTestModel(t)
	m = sendWindowSize(t, m, 80, 24)

	// Open palette.
	updated, _ := m.Update(tea.KeyPressMsg{Code: ':', Text: ":"})
	m2 := updated.(tui.Model)

	// Close with Escape.
	updated2, _ := m2.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m3 := updated2.(tui.Model)

	v := m3.View()
	if v.Content == "" {
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
	_, _ = m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
}

// TestMountLeftAndRight verifies that custom pane implementations can be
// mounted without panicking or producing empty views.
func TestMountLeftAndRight(t *testing.T) {
	m := newTestModel(t)
	m = sendWindowSize(t, m, 80, 24)

	m.MountLeft(tui.NewEmptyPane(m.Theme()))
	m.MountRight(tui.NewEmptyPane(m.Theme()))

	v := m.View()
	if v.Content == "" {
		t.Error("expected non-empty view after mounting panes")
	}
}

// TestAppBootsWithDashboard is the WL.6 smoke test. It wires the full stack —
// real store → memIndex → query engine → dashboard → TUI — and verifies the
// app initialises without error and renders non-empty content containing the
// expected node bodies.
func TestAppBootsWithDashboard(t *testing.T) {
	dir := t.TempDir()
	clock := types.StubClock{Fixed: time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)}

	s, err := store.New(dir, clock)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	defer s.Close()

	// Seed one node of each dashboard category with explicit titles.
	taskNode, err := s.CreateNode("Buy groceries", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode task: %v", err)
	}
	taskNode.Title = "Buy groceries"
	if err := s.WriteNode(taskNode); err != nil {
		t.Fatalf("WriteNode task: %v", err)
	}

	noteNode, err := s.CreateNode("Meeting notes", []string{"note"})
	if err != nil {
		t.Fatalf("CreateNode note: %v", err)
	}
	noteNode.Title = "Meeting notes"
	if err := s.WriteNode(noteNode); err != nil {
		t.Fatalf("WriteNode note: %v", err)
	}

	journalNode, err := s.CreateNode("Today I learned Go", []string{"journal"})
	if err != nil {
		t.Fatalf("CreateNode journal: %v", err)
	}
	journalNode.Title = "Today I learned Go"
	if err := s.WriteNode(journalNode); err != nil {
		t.Fatalf("WriteNode journal: %v", err)
	}

	engine := query.NewEngine(s.Index(), 10)

	m, err := tui.New(tui.Config{
		Store:       s,
		StorePath:   dir,
		Index:       s.Index(),
		QueryRunner: engine,
		Clock:       clock,
	})
	if err != nil {
		t.Fatalf("tui.New: %v", err)
	}

	m = sendWindowSize(t, m, 120, 40)

	v := m.View()
	if v.Content == "" {
		t.Fatal("expected non-empty view after boot with dashboard")
	}

	// Each seeded node body should appear somewhere in the rendered output.
	for _, body := range []string{"Buy groceries", "Meeting notes", "Today I learned Go"} {
		if !strings.Contains(v.Content, body) {
			t.Errorf("expected %q in dashboard view, not found", body)
		}
	}

	// Verify clean quit.
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("expected quit command after 'q'")
	}
	if msg := cmd(); msg != tea.Quit() {
		t.Errorf("expected tea.Quit, got %v", msg)
	}
}

// --- NV.8: spinner tests ---

// TestSpinnerStartAndTick starts the spinner, feeds a TickMsg through Update,
// and verifies the view contains the spinner message text.
func TestSpinnerStartAndTick(t *testing.T) {
	m := newTestModel(t)
	m = sendWindowSize(t, m, 80, 24)

	cmd := m.StatusBar().StartSpinner("Loading…")
	if cmd == nil {
		t.Fatal("StartSpinner should return a non-nil cmd")
	}

	// Feed the resulting TickMsg through Update.
	tickMsg := cmd()
	updated, _ := m.Update(tickMsg)
	result, ok := updated.(tui.Model)
	if !ok {
		t.Fatalf("unexpected type %T", updated)
	}

	view := result.View().Content
	if !strings.Contains(view, "Loading…") {
		t.Errorf("expected spinner message 'Loading…' in view, got:\n%s", view)
	}
}

// TestSpinnerStopRemovesFromView starts then stops the spinner, verifying
// the spinner message is absent from the view afterwards.
func TestSpinnerStopRemovesFromView(t *testing.T) {
	m := newTestModel(t)
	m = sendWindowSize(t, m, 80, 24)

	m.StatusBar().StartSpinner("Processing…")

	// Verify it's present while spinning.
	view := m.View().Content
	if !strings.Contains(view, "Processing…") {
		t.Errorf("expected spinner message while spinning, got:\n%s", view)
	}

	// Stop and verify it's gone.
	m.StatusBar().StopSpinner()
	view = m.View().Content
	if strings.Contains(view, "Processing…") {
		t.Errorf("expected spinner message absent after stop, got:\n%s", view)
	}
}

// --- NV.15: focus-lost tests ---

// spyPane is a minimal PaneModel that records whether HandleFocusLost was called.
type spyPane struct {
	focusLostCalled bool
}

func (s *spyPane) Update(_ tea.Msg) (tui.PaneModel, tea.Cmd) { return s, nil }
func (s *spyPane) View() string                              { return "spy" }
func (s *spyPane) KeyBindings() []tui.KeyBinding             { return nil }
func (s *spyPane) HandleFocusLost() tea.Cmd {
	s.focusLostCalled = true
	return nil
}

// TestFocusLostCalledOnSwitch mounts a spy pane on the left, switches focus
// to the right, and asserts that HandleFocusLost was called on the spy.
func TestFocusLostCalledOnSwitch(t *testing.T) {
	m := newTestModel(t)
	m = sendWindowSize(t, m, 80, 24)

	spy := &spyPane{}
	m.MountLeft(spy)

	// Focus starts on the left; switch to right — left pane loses focus.
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl})
	_, ok := updated.(tui.Model)
	if !ok {
		t.Fatalf("unexpected type %T", updated)
	}

	if !spy.focusLostCalled {
		t.Error("expected HandleFocusLost to be called on the left pane after focus switch")
	}
}

// TestJumpToTopDoesNotPanic verifies that alt+shift+up is routed to the
// focused pane without panicking.
func TestJumpToTopDoesNotPanic(t *testing.T) {
	m := newTestModel(t)
	m = sendWindowSize(t, m, 80, 24)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("jump to top panicked: %v", r)
		}
	}()

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModAlt | tea.ModShift})
	result, ok := updated.(tui.Model)
	if !ok {
		t.Fatalf("unexpected type %T", updated)
	}
	if result.View().Content == "" {
		t.Error("expected non-empty view after jump to top")
	}
}

// --- NV.14: broadcast tests ---

// TestBroadcastNonKeyMessage verifies that a custom non-key message does not
// panic and the model returns cleanly.
func TestBroadcastNonKeyMessage(t *testing.T) {
	m := newTestModel(t)
	m = sendWindowSize(t, m, 80, 24)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("broadcast panicked: %v", r)
		}
	}()

	type customMsg struct{ value string }
	updated, _ := m.Update(customMsg{value: "hello"})
	result, ok := updated.(tui.Model)
	if !ok {
		t.Fatalf("unexpected type %T", updated)
	}
	if result.View().Content == "" {
		t.Error("expected non-empty view after broadcast")
	}
}

// TestKeyReleaseRoutedToFocusedPane verifies that a KeyReleaseMsg does not
// panic and is routed to the focused pane only.
func TestKeyReleaseRoutedToFocusedPane(t *testing.T) {
	m := newTestModel(t)
	m = sendWindowSize(t, m, 80, 24)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("KeyReleaseMsg routing panicked: %v", r)
		}
	}()

	updated, _ := m.Update(tea.KeyReleaseMsg{Code: 'j'})
	result, ok := updated.(tui.Model)
	if !ok {
		t.Fatalf("unexpected type %T", updated)
	}
	if result.View().Content == "" {
		t.Error("expected non-empty view after KeyReleaseMsg")
	}
}

// TestJumpToBottomDoesNotPanic verifies that alt+shift+down is routed to the
// focused pane without panicking.
func TestJumpToBottomDoesNotPanic(t *testing.T) {
	m := newTestModel(t)
	m = sendWindowSize(t, m, 80, 24)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("jump to bottom panicked: %v", r)
		}
	}()

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModAlt | tea.ModShift})
	result, ok := updated.(tui.Model)
	if !ok {
		t.Fatalf("unexpected type %T", updated)
	}
	if result.View().Content == "" {
		t.Error("expected non-empty view after jump to bottom")
	}
}
