package tui

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// ":kinds edit <name>" dispatch tests. Live in the internal package for the
// same reason remap_command_internal_test.go does — PaletteState.commands
// and Model.palette are unexported. Reuses newRemapTestModel, runCommand,
// and truncateForTest from that file.
// ---------------------------------------------------------------------------

// kindsCommand locates the registered "kinds" command by name.
func kindsCommand(t *testing.T, m Model) Command {
	t.Helper()
	for _, c := range m.palette.commands {
		if c.Name == "kinds" {
			return c
		}
	}
	t.Fatal("no \"kinds\" command registered")
	return Command{}
}

// TestKindsEditMountsFormForKnownKind verifies ":kinds edit Task" mounts a
// kindFormPane pre-populated from the registry entry.
func TestKindsEditMountsFormForKnownKind(t *testing.T) {
	m := newRemapTestModel(t, nil)

	cmd := kindsCommand(t, m)
	m = runCommand(t, m, cmd, []string{"edit", "Task"})

	fp, ok := m.rightPane.(kindFormPane)
	if !ok {
		t.Fatalf("expected rightPane to be a kindFormPane, got %T", m.rightPane)
	}
	if fp.originalName != "Task" {
		t.Errorf("fp.originalName = %q, want %q", fp.originalName, "Task")
	}
	if fp.name != "Task" {
		t.Errorf("fp.name = %q, want %q (seeded from the existing entry)", fp.name, "Task")
	}
}

// TestKindsEditMultiWordNameJoins verifies args after "edit" are joined with
// spaces, since the palette's strings.Fields tokeniser splits a multi-word
// name into separate args before Execute ever sees it. No kind actually
// named "My Kind" exists — this test only asserts the name reaches the
// mount handler joined, via the not-found message echoing it back.
func TestKindsEditMultiWordNameJoins(t *testing.T) {
	m := newRemapTestModel(t, nil)

	cmd := kindsCommand(t, m)
	m = runCommand(t, m, cmd, []string{"edit", "My", "Kind"})

	view := m.View().Content
	if !strings.Contains(view, `"My Kind"`) {
		t.Errorf("expected the not-found message to echo the joined name %q; got: %q", "My Kind", truncateForTest(view, 300))
	}
}

// TestKindsEditNoNameShowsUsage verifies ":kinds edit" with no name gives a
// usage message rather than silently closing the palette.
func TestKindsEditNoNameShowsUsage(t *testing.T) {
	m := newRemapTestModel(t, nil)

	cmd := kindsCommand(t, m)
	m = runCommand(t, m, cmd, []string{"edit"})

	if _, isForm := m.rightPane.(formActivePane); isForm {
		t.Error("expected no form mounted when no name is given")
	}
	view := m.View().Content
	if !strings.Contains(view, "Usage: :kinds edit") {
		t.Errorf("expected usage message in view; got: %q", truncateForTest(view, 300))
	}
}

// TestKindsEditUnknownNameShowsNotFound verifies an unknown kind name leaves
// the pane unchanged and reports via the status bar rather than mounting a
// form against a zero-value entry.
func TestKindsEditUnknownNameShowsNotFound(t *testing.T) {
	m := newRemapTestModel(t, nil)

	cmd := kindsCommand(t, m)
	m = runCommand(t, m, cmd, []string{"edit", "Sasquatch"})

	if _, isForm := m.rightPane.(formActivePane); isForm {
		t.Error("expected no form mounted for an unknown kind name")
	}
	view := m.View().Content
	if !strings.Contains(view, `No kind "Sasquatch"`) {
		t.Errorf("expected not-found message in view; got: %q", truncateForTest(view, 300))
	}
}

// TestKindsEditWrongCaseResolves verifies ":kinds edit task" (lowercase)
// resolves to the registry's "Task" entry via the case-insensitive fallback.
func TestKindsEditWrongCaseResolves(t *testing.T) {
	m := newRemapTestModel(t, nil)

	cmd := kindsCommand(t, m)
	m = runCommand(t, m, cmd, []string{"edit", "task"})

	fp, ok := m.rightPane.(kindFormPane)
	if !ok {
		t.Fatalf("expected rightPane to be a kindFormPane, got %T", m.rightPane)
	}
	if fp.originalName != "Task" {
		t.Errorf("fp.originalName = %q, want %q (case-insensitive resolution)", fp.originalName, "Task")
	}
}

// TestKindsEditBlockedWhenFormActive verifies the formActivePane guard
// prevents ":kinds edit" from clobbering an already-open form.
func TestKindsEditBlockedWhenFormActive(t *testing.T) {
	m := newRemapTestModel(t, nil)

	// Open the create form first.
	cmd := kindsCommand(t, m)
	m = runCommand(t, m, cmd, []string{"new"})
	if _, isForm := m.rightPane.(formActivePane); !isForm {
		t.Fatal("precondition failed: create form should be mounted")
	}
	createPane := m.rightPane

	m = runCommand(t, m, cmd, []string{"edit", "Task"})

	if m.rightPane != createPane {
		t.Error("expected the original create form to remain mounted, guard should have blocked the edit open")
	}
}
