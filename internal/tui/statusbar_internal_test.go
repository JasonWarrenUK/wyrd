package tui

import (
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jasonwarrenuk/wyrd/internal/query"
	"github.com/jasonwarrenuk/wyrd/internal/store"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// newCaptureTestModel creates a minimal Model for exercising the capture-bar
// sticky/transient/esc behaviour (CP.17). No node is seeded — these tests
// only drive status-bar state via Update, not the node list.
func newCaptureTestModel(t *testing.T) Model {
	t.Helper()
	dir := t.TempDir()
	clock := types.StubClock{Fixed: time.Date(2026, 3, 23, 9, 0, 0, 0, time.UTC)}
	s, err := store.New(dir, clock)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	engine := query.NewEngine(s.Index(), 10)
	m, err := New(Config{
		Store:       s,
		StorePath:   dir,
		Index:       s.Index(),
		QueryRunner: engine,
		Clock:       clock,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return m
}

// updateModel drives Update and returns the resulting Model, asserting the
// concrete type assertion succeeds (Update returns tea.Model).
func updateModel(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	next, _ := m.Update(msg)
	got, ok := next.(Model)
	if !ok {
		t.Fatalf("Update did not return a tui.Model, got %T", next)
	}
	return got
}

// TestCaptureConfirmClearMsgRaceGuard verifies that a stale clear tick (for
// an older generation) does not clear a newer message that has since
// replaced it.
func TestCaptureConfirmClearMsgRaceGuard(t *testing.T) {
	m := newCaptureTestModel(t)

	m.statusBar.SetCaptureText("Message A")
	genA := m.statusBar.CaptureGen()

	m.statusBar.SetCaptureText("Message B")

	// A tick scheduled for generation A arrives after B has already
	// replaced it — it must be ignored.
	m = updateModel(t, m, captureConfirmClearMsg{gen: genA})

	if m.statusBar.captureText != "Message B" {
		t.Errorf("stale clear tick cleared a newer message: got %q, want %q", m.statusBar.captureText, "Message B")
	}

	// A tick scheduled for the current generation must clear it.
	m = updateModel(t, m, captureConfirmClearMsg{gen: m.statusBar.CaptureGen()})
	if m.statusBar.captureText != CaptureBarPlaceholder() {
		t.Errorf("current-gen clear tick did not reset to placeholder: got %q", m.statusBar.captureText)
	}
}

// TestSyncFailureIsStickyAndEscDismisses verifies that a sync failure sets a
// persistent (sticky) message with no auto-clear, and that esc dismisses it.
func TestSyncFailureIsStickyAndEscDismisses(t *testing.T) {
	m := newCaptureTestModel(t)
	m = sendWindowSizeInternal(t, m, 120, 40)

	m = updateModel(t, m, syncResultMsg{err: errors.New("network unreachable")})

	if !m.statusBar.CaptureSticky() {
		t.Fatal("expected sync failure to mark the capture message sticky")
	}
	if m.statusBar.captureText != "Sync failed: network unreachable" {
		t.Errorf("unexpected capture text: %q", m.statusBar.captureText)
	}

	// esc should dismiss a sticky message and restore the placeholder.
	m = updateModel(t, m, tea.KeyPressMsg{Code: tea.KeyEscape, Text: ""})

	if m.statusBar.CaptureSticky() {
		t.Error("expected esc to clear the sticky flag")
	}
	if m.statusBar.captureText != CaptureBarPlaceholder() {
		t.Errorf("expected esc to restore the placeholder, got %q", m.statusBar.captureText)
	}
}

// TestSyncSuccessIsTransientAndEscIgnoresIt verifies that a successful sync
// schedules an auto-clear (non-sticky) and that esc does NOT dismiss it —
// only sticky/persistent messages are esc-dismissable.
func TestSyncSuccessIsTransientAndEscIgnoresIt(t *testing.T) {
	m := newCaptureTestModel(t)
	m = sendWindowSizeInternal(t, m, 120, 40)

	m = updateModel(t, m, syncResultMsg{output: "up to date"})

	if m.statusBar.CaptureSticky() {
		t.Fatal("expected sync success to be transient, not sticky")
	}
	if m.statusBar.captureText != "Sync complete" {
		t.Errorf("unexpected capture text: %q", m.statusBar.captureText)
	}

	// esc must not touch a non-sticky (transient) message.
	m = updateModel(t, m, tea.KeyPressMsg{Code: tea.KeyEscape, Text: ""})

	if m.statusBar.captureText != "Sync complete" {
		t.Errorf("esc should not dismiss a transient message, got %q", m.statusBar.captureText)
	}
}

// TestTransientConfirmationEscIgnored verifies that an ordinary transient
// confirmation (e.g. node creation) is left untouched by esc.
func TestTransientConfirmationEscIgnored(t *testing.T) {
	m := newCaptureTestModel(t)
	m = sendWindowSizeInternal(t, m, 120, 40)

	m = updateModel(t, m, captureSubmitMsg{nodeID: "abc123", label: "Fix the bug"})

	if m.statusBar.CaptureSticky() {
		t.Fatal("expected node-creation confirmation to be transient, not sticky")
	}
	if m.statusBar.captureText != "Created Fix the bug" {
		t.Errorf("unexpected capture text: %q", m.statusBar.captureText)
	}

	m = updateModel(t, m, tea.KeyPressMsg{Code: tea.KeyEscape, Text: ""})

	if m.statusBar.captureText != "Created Fix the bug" {
		t.Errorf("esc should not dismiss a transient confirmation, got %q", m.statusBar.captureText)
	}
}

// sendWindowSizeInternal is a package-local mirror of the tui_test package's
// sendWindowSize helper (unexported, so it can't be imported across the
// test-package boundary).
func sendWindowSizeInternal(t *testing.T, m Model, w, h int) Model {
	t.Helper()
	return updateModel(t, m, tea.WindowSizeMsg{Width: w, Height: h})
}
