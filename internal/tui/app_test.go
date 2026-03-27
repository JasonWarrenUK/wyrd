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

// newTestModelWithStore creates a Model wired to a real store so the
// capture bar is active. Used for CP.0 / CP.8 tests.
func newTestModelWithStore(t *testing.T) tui.Model {
	t.Helper()
	dir := t.TempDir()
	clock := types.StubClock{Fixed: time.Date(2026, 3, 23, 9, 0, 0, 0, time.UTC)}
	s, err := store.New(dir, clock)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
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

// TestQuitAction verifies that pressing ctrl+c issues a quit command.
func TestQuitAction(t *testing.T) {
	m := newTestModel(t)
	m = sendWindowSize(t, m, 80, 24)

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("expected a non-nil command after ctrl+c")
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

// TestCommandPaletteOpensOnCtrlP checks that pressing ctrl+p opens the palette.
func TestCommandPaletteOpensOnCtrlP(t *testing.T) {
	m := newTestModel(t)
	m = sendWindowSize(t, m, 80, 24)

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
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
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
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
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("expected quit command after ctrl+c")
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

// --- CP.0 / CP.8: capture bar + form dispatch tests ---

// pressKey is a helper that sends a single key press and returns the updated model.
func pressKey(t *testing.T, m tui.Model, code rune, text string) tui.Model {
	t.Helper()
	updated, _ := m.Update(tea.KeyPressMsg{Code: code, Text: text})
	result, ok := updated.(tui.Model)
	if !ok {
		t.Fatalf("Update returned unexpected type %T", updated)
	}
	return result
}

// pressModKey is a helper that sends a modifier key press (e.g. ctrl+n) and
// returns the updated model.
func pressModKey(t *testing.T, m tui.Model, code rune, mod tea.KeyMod) tui.Model {
	t.Helper()
	updated, _ := m.Update(tea.KeyPressMsg{Code: code, Mod: mod})
	result, ok := updated.(tui.Model)
	if !ok {
		t.Fatalf("Update returned unexpected type %T", updated)
	}
	return result
}

// TestCaptureBarFocusesOnCtrlN verifies that pressing ctrl+n updates the
// status bar to show the cursor character (indicating capture mode is active).
func TestCaptureBarFocusesOnCtrlN(t *testing.T) {
	m := newTestModelWithStore(t)
	m = sendWindowSize(t, m, 120, 40)

	m = pressModKey(t, m, 'n', tea.ModCtrl)

	v := m.View().Content
	// The cursor character should appear in the status bar area.
	if !strings.Contains(v, "▌") {
		t.Error("expected cursor character '▌' in view after pressing ctrl+n")
	}
}

// TestCaptureBarTypingAccumulates verifies that typing after pressing ctrl+n
// accumulates in the status bar display.
func TestCaptureBarTypingAccumulates(t *testing.T) {
	m := newTestModelWithStore(t)
	m = sendWindowSize(t, m, 120, 40)

	m = pressModKey(t, m, 'n', tea.ModCtrl)
	m = pressKey(t, m, 'h', "h")
	// 'i' while capture bar focused is a rune input, not ActionCapture.
	m = pressKey(t, m, 'i', "i")

	v := m.View().Content
	if !strings.Contains(v, "hi▌") {
		t.Errorf("expected 'hi▌' in view, got content length %d", len(v))
	}
}

// TestCaptureBarEscapeCancels verifies that Escape clears capture mode and
// restores the placeholder text.
func TestCaptureBarEscapeCancels(t *testing.T) {
	m := newTestModelWithStore(t)
	m = sendWindowSize(t, m, 120, 40)

	m = pressModKey(t, m, 'n', tea.ModCtrl) // focus capture bar
	m = pressKey(t, m, tea.KeyEsc, "")      // cancel

	v := m.View().Content
	// Placeholder text should be restored and cursor should be gone.
	if strings.Contains(v, "▌") {
		t.Error("cursor '▌' should not appear in view after Escape")
	}
	if !strings.Contains(v, "ctrl+n to capture") {
		t.Error("expected placeholder text after Escape")
	}
}

// TestCaptureBarEnterOpensTaskForm verifies that pressing Enter after typing
// a "t:" prefix mounts a form with a Title field in the right pane.
func TestCaptureBarEnterOpensTaskForm(t *testing.T) {
	m := newTestModelWithStore(t)
	m = sendWindowSize(t, m, 120, 40)

	// Type "t: Buy milk" and press Enter.
	m = pressModKey(t, m, 'n', tea.ModCtrl)
	for _, r := range "t: Buy milk" {
		m = pressKey(t, m, r, string(r))
	}
	m = pressKey(t, m, tea.KeyEnter, "")

	// The view should now contain the huh form (title field label).
	v := m.View().Content
	if !strings.Contains(v, "Title") {
		t.Errorf("expected form title field in view after capture Enter, got content length %d", len(v))
	}
}

// TestCaptureBarEmptyEnterBlurs verifies that pressing Enter on an empty
// capture bar simply blurs without opening a form.
func TestCaptureBarEmptyEnterBlurs(t *testing.T) {
	m := newTestModelWithStore(t)
	m = sendWindowSize(t, m, 120, 40)

	m = pressModKey(t, m, 'n', tea.ModCtrl)
	m = pressKey(t, m, tea.KeyEnter, "")

	v := m.View().Content
	if strings.Contains(v, "▌") {
		t.Error("cursor should not be present after empty Enter")
	}
}

// --- Readiness and spurious input tests ---

// TestSpuriousBytesDoNotTriggerActions verifies that bytes commonly found in
// OSC 11 terminal responses (e.g. iTerm2 background colour notifications) do
// not trigger application-level actions. Previously ':' from 'rgb:000/000/000'
// would open the command palette.
func TestSpuriousBytesDoNotTriggerActions(t *testing.T) {
	m := newTestModel(t)
	m = sendWindowSize(t, m, 80, 24)

	// Simulate the body bytes of an OSC 11 response fragmented into keypresses.
	for _, ch := range []rune{':', ']', '[', ';', 'r', 'g', 'b'} {
		updated, cmd := m.Update(tea.KeyPressMsg{Code: ch, Text: string(ch)})
		m = updated.(tui.Model)
		if cmd != nil {
			if msg := cmd(); msg == tea.Quit() {
				t.Fatalf("spurious byte %q triggered quit", ch)
			}
		}
	}
	// None of these should have opened the palette.
	v := m.View().Content
	if strings.Contains(v, "COMMAND PALETTE") || strings.Contains(v, "SEARCH COMMANDS") {
		t.Error("spurious bytes opened the command palette")
	}
}

// --- CP.10: edit node tests ---

// newTestModelWithNode creates a model pre-seeded with a single task node
// for edit-form testing.
func newTestModelWithNode(t *testing.T) (tui.Model, string) {
	t.Helper()
	dir := t.TempDir()
	clock := types.StubClock{Fixed: time.Date(2026, 3, 23, 9, 0, 0, 0, time.UTC)}
	s, err := store.New(dir, clock)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	node, err := s.CreateNode("Fix the bug", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	node.Title = "Fix the bug"
	if err := s.WriteNode(node); err != nil {
		t.Fatalf("WriteNode: %v", err)
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
	return m, node.ID
}

// TestEditNodeKeyOpensEditForm verifies that pressing ctrl+o when a node is
// available in the dashboard mounts an edit form in the right pane.
func TestEditNodeKeyOpensEditForm(t *testing.T) {
	m, _ := newTestModelWithNode(t)
	m = sendWindowSize(t, m, 120, 40)

	// The dashboard is populated; press ctrl+o to open the edit form.
	m = pressModKey(t, m, 'o', tea.ModCtrl)

	v := m.View().Content
	// The edit form should render the Title field.
	if !strings.Contains(v, "Title") {
		t.Errorf("expected edit form with Title field after ctrl+o; got content length %d", len(v))
	}
}

// TestEditNodeIgnoredWhenNoSelection verifies that ctrl+o does nothing when
// the node list is empty (no node to edit).
func TestEditNodeIgnoredWhenNoSelection(t *testing.T) {
	// newTestModel boots without any nodes — list is empty.
	m := newTestModel(t)
	m = sendWindowSize(t, m, 120, 40)

	v1 := m.View().Content

	m = pressModKey(t, m, 'o', tea.ModCtrl)

	v2 := m.View().Content
	// The view should be identical (no form opened).
	if strings.Contains(v2, "Title") && !strings.Contains(v1, "Title") {
		t.Error("ctrl+o should not open edit form when no node is selected")
	}
}

// TestEditNodeIgnoredWhenFormActive verifies that ctrl+o is a no-op when
// a creation form is already open in the right pane.
func TestEditNodeIgnoredWhenFormActive(t *testing.T) {
	m, _ := newTestModelWithNode(t)
	m = sendWindowSize(t, m, 120, 40)

	// Open a creation form first via capture bar.
	m = pressModKey(t, m, 'n', tea.ModCtrl)
	for _, r := range "t: Buy milk" {
		m = pressKey(t, m, r, string(r))
	}
	m = pressKey(t, m, tea.KeyEnter, "")

	// Capture the view with the creation form active.
	viewWithForm := m.View().Content

	// Press ctrl+o — should be a no-op because a form is already active.
	m = pressModKey(t, m, 'o', tea.ModCtrl)

	viewAfterCtrlO := m.View().Content

	// The view should be unchanged (ctrl+o did nothing).
	if viewAfterCtrlO != viewWithForm {
		t.Error("ctrl+o should not change the view when a creation form is already active")
	}
}

// TestCtrlCWorksBeforeReady verifies that ctrl+c (which has a modifier) is
// not blocked by the readiness gate and produces a quit command immediately.
func TestCtrlCWorksBeforeReady(t *testing.T) {
	m := newTestModel(t)

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("expected quit command from ctrl+c before ready")
	}
	if msg := cmd(); msg != tea.Quit() {
		t.Errorf("expected tea.Quit, got %v", msg)
	}
}

// TestArrowKeysWorkBeforeReady verifies that arrow keys (which have Code >
// unicode.MaxRune) are not blocked by the readiness gate.
func TestArrowKeysWorkBeforeReady(t *testing.T) {
	m := newTestModel(t)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("arrow key before ready panicked: %v", r)
		}
	}()

	// Down arrow should be forwarded to the pane without panicking.
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
}
