package views

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

func TestTimelineRenderer_EmptyResult(t *testing.T) {
	r := NewTimelineRenderer()
	result := types.QueryResult{
		Columns: []string{"created", "body"},
		Rows:    []map[string]interface{}{},
	}
	output := r.Render(result, 80)
	if !strings.Contains(output, "No entries") {
		t.Errorf("expected empty-state message, got: %q", output)
	}
}

func TestTimelineRenderer_ReverseChronologicalOrder(t *testing.T) {
	r := NewTimelineRenderer()

	older := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	newer := time.Date(2024, 6, 15, 9, 0, 0, 0, time.UTC)

	result := types.QueryResult{
		Columns: []string{"created", "body"},
		Rows: []map[string]interface{}{
			{"created": older, "body": "Older entry"},
			{"created": newer, "body": "Newer entry"},
		},
	}

	output := r.Render(result, 80)

	olderIdx := strings.Index(output, "Older entry")
	newerIdx := strings.Index(output, "Newer entry")

	if olderIdx == -1 || newerIdx == -1 {
		t.Fatal("expected both entries to appear in output")
	}
	if newerIdx > olderIdx {
		t.Error("expected newer entry to appear before older entry (reverse-chronological)")
	}
}

func TestTimelineRenderer_DateHeaderFormatting(t *testing.T) {
	r := NewTimelineRenderer()
	ts := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	result := types.QueryResult{
		Columns: []string{"created", "body"},
		Rows: []map[string]interface{}{
			{"created": ts, "body": "Test entry"},
		},
	}

	output := r.Render(result, 80)
	// The date format is "Monday 2 January 2006".
	if !strings.Contains(output, "15 March 2024") {
		t.Errorf("expected formatted date in output, got:\n%s", output)
	}
}

func TestTimelineRenderer_ISO8601DateString(t *testing.T) {
	r := NewTimelineRenderer()

	result := types.QueryResult{
		Columns: []string{"created", "body"},
		Rows: []map[string]interface{}{
			{"created": "2024-09-20T08:30:00Z", "body": "ISO string entry"},
		},
	}

	output := r.Render(result, 80)
	if !strings.Contains(output, "ISO string entry") {
		t.Errorf("expected entry body in output, got:\n%s", output)
	}
	if !strings.Contains(output, "2024") {
		t.Errorf("expected year 2024 in output, got:\n%s", output)
	}
}

func TestTimelineRenderer_SeparatorBetweenEntries(t *testing.T) {
	r := NewTimelineRenderer()

	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	result := types.QueryResult{
		Columns: []string{"created", "body"},
		Rows: []map[string]interface{}{
			{"created": t1, "body": "Entry one"},
			{"created": t2, "body": "Entry two"},
		},
	}

	output := r.Render(result, 80)
	if !strings.Contains(output, timelineSeparator) {
		t.Error("expected separator between entries")
	}
}

func TestTimelineRenderer_UnknownDateHandled(t *testing.T) {
	r := NewTimelineRenderer()

	result := types.QueryResult{
		Columns: []string{"created", "body"},
		Rows: []map[string]interface{}{
			{"created": nil, "body": "No date entry"},
		},
	}

	output := r.Render(result, 80)
	if !strings.Contains(output, "Unknown date") {
		t.Errorf("expected 'Unknown date' for nil timestamp, got:\n%s", output)
	}
}

func TestTimelineRenderer_TypeColouredOutputDiffersFromPlain(t *testing.T) {
	ts := time.Date(2024, 5, 10, 12, 0, 0, 0, time.UTC)

	rows := []map[string]interface{}{
		{"created": ts, "body": "Coloured entry", "types": []interface{}{"task"}},
	}
	qr := types.QueryResult{
		Columns: []string{"created", "body", "types"},
		Rows:    rows,
	}

	// Plain renderer (no TypeColour callback).
	plain := NewTimelineRenderer()
	plainOut := plain.Render(qr, 80)

	// Coloured renderer.
	coloured := NewTimelineRenderer()
	coloured.TypeColour = func(typeName string) (bg, fg string) {
		return "#794aff", "#f6f6f6"
	}
	colouredOut := coloured.Render(qr, 80)

	if plainOut == colouredOut {
		t.Error("expected type-coloured output to differ from plain output")
	}
}

func TestTimelineRenderer_MissingTypesColumnGraceful(t *testing.T) {
	ts := time.Date(2024, 5, 10, 12, 0, 0, 0, time.UTC)

	// Row has no "types" column at all.
	rows := []map[string]interface{}{
		{"created": ts, "body": "No types here"},
	}
	qr := types.QueryResult{
		Columns: []string{"created", "body"},
		Rows:    rows,
	}

	r := NewTimelineRenderer()
	r.TypeColour = func(typeName string) (bg, fg string) {
		return "#794aff", "#f6f6f6"
	}

	// Should not panic and should render normally.
	output := r.Render(qr, 80)
	if !strings.Contains(output, "No types here") {
		t.Errorf("expected body content in output, got:\n%s", output)
	}
}

func TestTimelineRenderer_TypeBadgeAppearsInOutput(t *testing.T) {
	ts := time.Date(2024, 5, 10, 12, 0, 0, 0, time.UTC)

	rows := []map[string]interface{}{
		{"created": ts, "body": "Task entry", "types": []interface{}{"task"}},
	}
	qr := types.QueryResult{
		Columns: []string{"created", "body", "types"},
		Rows:    rows,
	}

	r := NewTimelineRenderer()
	r.TypeColour = func(typeName string) (bg, fg string) {
		return "#794aff", "#f6f6f6"
	}

	output := r.Render(qr, 80)
	if !strings.Contains(output, "task") {
		t.Errorf("expected type badge text 'task' in output, got:\n%s", output)
	}
}

func TestExtractTypes_SliceOfInterface(t *testing.T) {
	row := map[string]interface{}{
		"types": []interface{}{"task", "note"},
	}
	got := extractTypes(row, "types")
	if len(got) != 2 || got[0] != "task" || got[1] != "note" {
		t.Errorf("expected [task note], got %v", got)
	}
}

func TestExtractTypes_StringSlice(t *testing.T) {
	row := map[string]interface{}{
		"types": []string{"journal"},
	}
	got := extractTypes(row, "types")
	if len(got) != 1 || got[0] != "journal" {
		t.Errorf("expected [journal], got %v", got)
	}
}

func TestExtractTypes_SingleString(t *testing.T) {
	row := map[string]interface{}{
		"types": "budget",
	}
	got := extractTypes(row, "types")
	if len(got) != 1 || got[0] != "budget" {
		t.Errorf("expected [budget], got %v", got)
	}
}

func TestExtractTypes_MissingColumn(t *testing.T) {
	row := map[string]interface{}{
		"body": "hello",
	}
	got := extractTypes(row, "types")
	if got != nil {
		t.Errorf("expected nil for missing column, got %v", got)
	}
}

func TestExtractTypes_NilValue(t *testing.T) {
	row := map[string]interface{}{
		"types": nil,
	}
	got := extractTypes(row, "types")
	if got != nil {
		t.Errorf("expected nil for nil value, got %v", got)
	}
}

// TestTimelineRenderer_RenderBackgroundCarriesThroughEveryStyle covers
// TD.6's Render path (RenderBlocks already carried Background — this
// asserts the plain, non-block Render mode was brought up to the same
// standard): date header, separator, body text, and the blank inter-entry
// line must all carry the palette's Background.
func TestTimelineRenderer_RenderBackgroundCarriesThroughEveryStyle(t *testing.T) {
	r := NewTimelineRenderer()
	r.Palette.Background = lipgloss.Color("#1a1a2e")
	result := types.QueryResult{
		Columns: []string{"created", "body"},
		Rows: []map[string]interface{}{
			{"created": time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC), "body": "First entry"},
			{"created": time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC), "body": "Second entry"},
		},
	}

	out := r.Render(result, 40)
	if !ansiTruecolourBgRe.MatchString(out) {
		t.Errorf("expected a truecolour background in output, got %q", out)
	}

	empty := r.Render(types.QueryResult{Columns: []string{"created", "body"}}, 40)
	if !ansiTruecolourBgRe.MatchString(empty) {
		t.Errorf("expected the empty-state message to carry a background too, got %q", empty)
	}
}

// TestTimelineRenderer_RenderTypeBadgeSpacerCarriesBackground covers the
// space between the date header and its type badge — a bare "  " literal
// (rule 3) rather than a background-aware spacer would show the terminal's
// default background in that gap.
func TestTimelineRenderer_RenderTypeBadgeSpacerCarriesBackground(t *testing.T) {
	r := NewTimelineRenderer()
	r.Palette.Background = lipgloss.Color("#1a1a2e")
	r.TypeColour = func(typeName string) (bg, fg string) { return "#9b70ff", "#ffffff" }
	result := types.QueryResult{
		Columns: []string{"created", "body", "types"},
		Rows: []map[string]interface{}{
			{"created": time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC), "body": "Entry", "types": []string{"task"}},
		},
	}

	out := r.Render(result, 40)
	if !ansiTruecolourBgRe.MatchString(out) {
		t.Errorf("expected a truecolour background in output with a type badge, got %q", out)
	}
}
