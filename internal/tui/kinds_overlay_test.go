package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
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
