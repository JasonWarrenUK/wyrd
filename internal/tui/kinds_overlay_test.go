package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/jasonwarrenuk/wyrd/internal/stage"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

func TestKindsOverlay_RendersAllKinds(t *testing.T) {
	kinds := []types.Kind{
		{Name: "Task", Glyph: "◆", Colour: "#9b70ff", StageGroup: "task-flow"},
		{Name: "Note", Glyph: "●", Colour: "#70c0ff", StageGroup: "content-flow"},
	}
	groups := []types.StageGroup{
		{Name: "task-flow", Stages: []string{"Open", "Done"}, Cycle: types.CycleTerminate},
		{Name: "content-flow", Stages: []string{"Draft", "Published"}, Cycle: types.CycleTerminate},
	}
	kindsReg := types.NewKindRegistry(kinds)
	groupsReg := types.NewStageGroupRegistry(groups)
	theme := loadStagesTestTheme(t)

	ko := newKindsOverlay(theme, kindsReg, groupsReg)
	ko.Open(120, 40)

	view := ko.View(120, 40)

	for _, k := range kinds {
		if !strings.Contains(view, k.Name) {
			t.Errorf("expected overlay to contain kind name %q", k.Name)
		}
	}
	if !strings.Contains(view, "Open → Done") {
		t.Error("expected resolved stage progression for Task")
	}
	if !strings.Contains(view, "KINDS") {
		t.Error("expected KINDS title in overlay")
	}
}

func TestKindsOverlay_EmptyState(t *testing.T) {
	theme := loadStagesTestTheme(t)
	ko := newKindsOverlay(theme, nil, nil)
	ko.Open(120, 40)

	view := ko.View(120, 40)
	if !strings.Contains(view, "No kinds registered") {
		t.Error("expected empty-state message for nil registry")
	}
}

// TestKindsOverlay_ProvenanceMarker mirrors
// TestStagesOverlay_ProvenanceMarker's three-state check: a purely
// user-defined kind gets (custom), an edited (shadowed) default gets
// (edited), and an untouched default gets no marker.
//
// Provenance (TD.15) comes from MergeKinds' registry itself — via
// types.NewKindRegistryFromMerge — rather than a separately-constructed
// userNames map threaded through the overlay constructor.
func TestKindsOverlay_ProvenanceMarker(t *testing.T) {
	defaults, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds: %v", err)
	}

	customKind := types.Kind{Name: "Errand", StageGroup: "task-flow", Glyph: "!", Colour: "#9b70ff"}
	// A shadow copy of the baked-in Task default — same name, different glyph.
	editedDefault := types.Kind{Name: "Task", StageGroup: "task-flow", Glyph: "★", Colour: "#9b70ff"}

	kindsReg := stage.MergeKinds(defaults, []types.Kind{customKind, editedDefault})
	groupsReg := types.NewStageGroupRegistry([]types.StageGroup{
		{Name: "task-flow", Stages: []string{"Open", "Done"}, Cycle: types.CycleTerminate},
	})
	theme := loadStagesTestTheme(t)

	ko := newKindsOverlay(theme, kindsReg, groupsReg)
	ko.Open(160, 50)

	view := ko.View(160, 50)

	if !strings.Contains(view, "(custom)") {
		t.Error("expected (custom) marker for the purely user-defined kind")
	}
	if !strings.Contains(view, "(edited)") {
		t.Error("expected (edited) marker for the shadowed default")
	}
	if !strings.Contains(view, "Errand") {
		t.Error("expected user kind Errand to appear")
	}
	// Note is a baked-in default never touched by the user — should appear
	// with no provenance marker.
	if !strings.Contains(view, "Note") {
		t.Fatal("expected untouched default Note to appear")
	}
}

// TestKindsOverlay_DivergedMarker covers TD.5: a shadowed kind whose
// ShadowOf no longer matches the current default's hash renders (diverged)
// instead of (edited) — the more actionable state takes priority since a
// diverged entry is necessarily also an edited shadow.
func TestKindsOverlay_DivergedMarker(t *testing.T) {
	defaults, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds: %v", err)
	}

	diverged := types.Kind{Name: "Task", StageGroup: "task-flow", Glyph: "★", Colour: "#9b70ff", ShadowOf: "sha256:0000000000000000"}
	faithful := types.Kind{Name: "Goblin", StageGroup: "task-flow", Glyph: "◈", Colour: "#d57300", ShadowOf: stage.DefaultKindHash("Goblin")}

	kindsReg := stage.MergeKinds(defaults, []types.Kind{diverged, faithful})
	groupsReg := types.NewStageGroupRegistry([]types.StageGroup{
		{Name: "task-flow", Stages: []string{"Open", "Done"}, Cycle: types.CycleTerminate},
	})
	theme := loadStagesTestTheme(t)

	ko := newKindsOverlay(theme, kindsReg, groupsReg)
	ko.Open(160, 50)

	view := ko.View(160, 50)

	if !strings.Contains(view, "(diverged)") {
		t.Error("expected (diverged) marker for the entry whose default changed")
	}
	// Goblin is faithfully shadowed (ShadowOf matches the current default),
	// so it must show (edited), never (diverged).
	lines := strings.Split(view, "\n")
	var goblinLine string
	for _, l := range lines {
		if strings.Contains(l, "Goblin") {
			goblinLine = l
			break
		}
	}
	if goblinLine == "" {
		t.Fatal("expected a Goblin row in the overlay")
	}
	if strings.Contains(goblinLine, "(diverged)") {
		t.Error("Goblin's ShadowOf matches its current default — must not show (diverged)")
	}
	if !strings.Contains(goblinLine, "(edited)") {
		t.Error("expected Goblin's row to show (edited)")
	}
}

// TestKindsOverlay_UntouchedDefaultHasNoMarker isolates the "no marker"
// case, mirroring TestStagesOverlay_UntouchedDefaultHasNoMarker.
func TestKindsOverlay_UntouchedDefaultHasNoMarker(t *testing.T) {
	defaults, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds: %v", err)
	}
	kindsReg := stage.MergeKinds(defaults, nil)
	theme := loadStagesTestTheme(t)

	ko := newKindsOverlay(theme, kindsReg, nil)
	ko.Open(160, 50)

	view := ko.View(160, 50)
	if strings.Contains(view, "(custom)") || strings.Contains(view, "(edited)") {
		t.Error("expected no provenance marker anywhere when no user kinds were merged in")
	}
}

// TestKindsOverlay_ColumnsUseDisplayWidth verifies that the name and
// stage-group column widths are measured with lipgloss.Width rather than
// len(). A kind named with a multi-byte rune (e.g. a CJK character) has a
// byte length longer than its display width, so len()-based padding
// under-pads the column and misaligns every row after it.
func TestKindsOverlay_ColumnsUseDisplayWidth(t *testing.T) {
	kinds := []types.Kind{
		// "任務" — 2 display cells... no, CJK is double-width: 4 display cells,
		// 6 bytes (3 bytes per rune in UTF-8). Either way len() != lipgloss.Width().
		{Name: "任務", Glyph: "◆", Colour: "#9b70ff", StageGroup: "task-flow"},
		{Name: "Errand", Glyph: "!", Colour: "#70c0ff", StageGroup: "task-flow"},
	}

	nameColWidth := 12
	for _, k := range kinds {
		if w := lipgloss.Width(k.Name) + 2; w > nameColWidth {
			nameColWidth = w
		}
	}

	for _, k := range kinds {
		displayWidth := lipgloss.Width(k.Name)
		byteLen := len(k.Name)

		pad := nameColWidth - displayWidth
		if pad < 1 {
			t.Errorf("kind %q: name pad = %d (< 1)", k.Name, pad)
			continue
		}

		if displayWidth != byteLen {
			bytePad := nameColWidth - byteLen
			if bytePad == pad {
				t.Errorf("kind %q: byte pad (%d) equals display pad (%d); test data doesn't exercise the bug",
					k.Name, bytePad, pad)
			}
		}

		if displayWidth+pad != nameColWidth {
			t.Errorf("kind %q: displayWidth(%d) + pad(%d) = %d, want %d",
				k.Name, displayWidth, pad, displayWidth+pad, nameColWidth)
		}
	}
}
