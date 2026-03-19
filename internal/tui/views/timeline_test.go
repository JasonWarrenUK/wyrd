package views

import (
	"strings"
	"testing"
	"time"

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
