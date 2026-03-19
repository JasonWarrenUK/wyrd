package views

import (
	"strings"
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

func TestListRenderer_EmptyResult(t *testing.T) {
	r := NewListRenderer(nil)
	result := types.QueryResult{
		Columns: []string{"title", "status"},
		Rows:    []map[string]interface{}{},
	}
	output := r.Render(result, -1, 80)
	if !strings.Contains(output, "No results") {
		t.Errorf("expected empty-state message, got: %q", output)
	}
}

func TestListRenderer_RendersColumns(t *testing.T) {
	r := NewListRenderer([]string{"title", "status"})
	result := types.QueryResult{
		Columns: []string{"title", "status"},
		Rows: []map[string]interface{}{
			{"title": "Write tests", "status": "open"},
			{"title": "Deploy app", "status": "closed"},
		},
	}
	output := r.Render(result, -1, 80)

	if !strings.Contains(output, "title") {
		t.Error("expected header 'title' in output")
	}
	if !strings.Contains(output, "Write tests") {
		t.Error("expected row content 'Write tests' in output")
	}
}

func TestListRenderer_ColumnAutoSizing(t *testing.T) {
	// Columns should auto-size: a very long value should be present (possibly truncated).
	r := NewListRenderer([]string{"name"})
	longValue := strings.Repeat("A", 100)
	result := types.QueryResult{
		Columns: []string{"name"},
		Rows: []map[string]interface{}{
			{"name": longValue},
		},
	}
	// Use a narrow width to force truncation.
	output := r.Render(result, -1, 20)
	if strings.Contains(output, longValue) {
		t.Error("expected long value to be truncated")
	}
	if !strings.Contains(output, listEllipsis) {
		t.Errorf("expected ellipsis %q in truncated output", listEllipsis)
	}
}

func TestListRenderer_EllipsisTruncation(t *testing.T) {
	truncated := padOrTruncate("Hello, World!", 8)
	if len([]rune(truncated)) != 8 {
		t.Errorf("padOrTruncate should produce exactly 8 runes, got %d", len([]rune(truncated)))
	}
	if !strings.HasSuffix(truncated, listEllipsis) {
		t.Errorf("padOrTruncate should end with ellipsis, got %q", truncated)
	}
}

func TestListRenderer_Padding(t *testing.T) {
	padded := padOrTruncate("Hi", 10)
	if len([]rune(padded)) != 10 {
		t.Errorf("padOrTruncate should produce exactly 10 runes when padding, got %d", len([]rune(padded)))
	}
	if !strings.HasPrefix(padded, "Hi") {
		t.Errorf("padded string should start with original content, got %q", padded)
	}
}

func TestListRenderer_SelectedRowHighlighting(t *testing.T) {
	r := NewListRenderer([]string{"title"})
	result := types.QueryResult{
		Columns: []string{"title"},
		Rows: []map[string]interface{}{
			{"title": "First"},
			{"title": "Second"},
		},
	}
	// Both render calls should produce output containing the row content.
	// The selection code path is exercised here; ANSI injection may or may not
	// occur depending on whether stdout is a terminal, so we only verify that
	// content is present in both cases.
	withSelection := r.Render(result, 0, 80)
	withoutSelection := r.Render(result, -1, 80)

	if !strings.Contains(withSelection, "First") {
		t.Error("selected render should contain row content")
	}
	if !strings.Contains(withoutSelection, "First") {
		t.Error("unselected render should contain row content")
	}
}

func TestListRenderer_FallbackColumns(t *testing.T) {
	// When no explicit columns are configured, result.Columns should be used.
	r := NewListRenderer(nil)
	result := types.QueryResult{
		Columns: []string{"id", "body"},
		Rows: []map[string]interface{}{
			{"id": "abc", "body": "Hello"},
		},
	}
	output := r.Render(result, -1, 80)
	if !strings.Contains(output, "id") {
		t.Error("expected fallback column 'id' in header")
	}
	if !strings.Contains(output, "body") {
		t.Error("expected fallback column 'body' in header")
	}
}
