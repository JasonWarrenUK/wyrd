package tui

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// ":stages edit <name>" dispatch tests. Mirrors
// kind_edit_command_internal_test.go exactly. Reuses newRemapTestModel,
// runCommand, and truncateForTest from remap_command_internal_test.go, and
// stagesCommand from the same file.
// ---------------------------------------------------------------------------

// TestStagesEditMountsFormForKnownGroup verifies ":stages edit task-flow"
// mounts a stageFormPane pre-populated from the registry entry.
func TestStagesEditMountsFormForKnownGroup(t *testing.T) {
	m := newRemapTestModel(t, nil)

	cmd := stagesCommand(t, m)
	m = runCommand(t, m, cmd, []string{"edit", "task-flow"})

	fp, ok := m.rightPane.(stageFormPane)
	if !ok {
		t.Fatalf("expected rightPane to be a stageFormPane, got %T", m.rightPane)
	}
	if fp.originalName != "task-flow" {
		t.Errorf("fp.originalName = %q, want %q", fp.originalName, "task-flow")
	}
	if fp.name != "task-flow" {
		t.Errorf("fp.name = %q, want %q (seeded from the existing entry)", fp.name, "task-flow")
	}
}

// TestStagesEditMultiWordNameJoins verifies args after "edit" are joined
// with spaces. No group named "My Flow" exists — this asserts the name
// reaches the mount handler joined, via the not-found message echoing it.
func TestStagesEditMultiWordNameJoins(t *testing.T) {
	m := newRemapTestModel(t, nil)

	cmd := stagesCommand(t, m)
	m = runCommand(t, m, cmd, []string{"edit", "My", "Flow"})

	view := m.View().Content
	if !strings.Contains(view, `"My Flow"`) {
		t.Errorf("expected the not-found message to echo the joined name %q; got: %q", "My Flow", truncateForTest(view, 300))
	}
}

// TestStagesEditNoNameShowsUsage verifies ":stages edit" with no name gives
// a usage message rather than silently closing the palette.
func TestStagesEditNoNameShowsUsage(t *testing.T) {
	m := newRemapTestModel(t, nil)

	cmd := stagesCommand(t, m)
	m = runCommand(t, m, cmd, []string{"edit"})

	if _, isForm := m.rightPane.(formActivePane); isForm {
		t.Error("expected no form mounted when no name is given")
	}
	view := m.View().Content
	if !strings.Contains(view, "Usage: :stages edit") {
		t.Errorf("expected usage message in view; got: %q", truncateForTest(view, 300))
	}
}

// TestStagesEditUnknownNameShowsNotFound verifies an unknown group name
// leaves the pane unchanged and reports via the status bar.
func TestStagesEditUnknownNameShowsNotFound(t *testing.T) {
	m := newRemapTestModel(t, nil)

	cmd := stagesCommand(t, m)
	m = runCommand(t, m, cmd, []string{"edit", "nonexistent-flow"})

	if _, isForm := m.rightPane.(formActivePane); isForm {
		t.Error("expected no form mounted for an unknown group name")
	}
	view := m.View().Content
	if !strings.Contains(view, `No stage group "nonexistent-flow"`) {
		t.Errorf("expected not-found message in view; got: %q", truncateForTest(view, 300))
	}
}

// TestStagesEditWrongCaseResolves verifies ":stages edit Task-Flow" resolves
// to the registry's "task-flow" entry via the case-insensitive fallback.
func TestStagesEditWrongCaseResolves(t *testing.T) {
	m := newRemapTestModel(t, nil)

	cmd := stagesCommand(t, m)
	m = runCommand(t, m, cmd, []string{"edit", "Task-Flow"})

	fp, ok := m.rightPane.(stageFormPane)
	if !ok {
		t.Fatalf("expected rightPane to be a stageFormPane, got %T", m.rightPane)
	}
	if fp.originalName != "task-flow" {
		t.Errorf("fp.originalName = %q, want %q (case-insensitive resolution)", fp.originalName, "task-flow")
	}
}

// TestStagesEditBlockedWhenFormActive verifies the formActivePane guard
// prevents ":stages edit" from clobbering an already-open form.
func TestStagesEditBlockedWhenFormActive(t *testing.T) {
	m := newRemapTestModel(t, nil)

	cmd := stagesCommand(t, m)
	m = runCommand(t, m, cmd, []string{"new"})
	if _, isForm := m.rightPane.(formActivePane); !isForm {
		t.Fatal("precondition failed: create form should be mounted")
	}
	createPane := m.rightPane

	m = runCommand(t, m, cmd, []string{"edit", "task-flow"})

	if m.rightPane != createPane {
		t.Error("expected the original create form to remain mounted, guard should have blocked the edit open")
	}
}
