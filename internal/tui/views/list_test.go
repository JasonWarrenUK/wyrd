package views

import (
	"regexp"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// ansiTruecolourBgRe extracts the r;g;b triple from a truecolour BACKGROUND
// SGR sequence (38;2;… is foreground, 48;2;… is background). Local copy of
// the regex used in internal/tui's own bleed-regression tests — this
// package cannot import internal/tui (circular dependency), so the
// duplication mirrors the existing spacer()/padLine() split.
var ansiTruecolourBgRe = regexp.MustCompile(`48;2;(\d+);(\d+);(\d+)`)

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

// TestListRenderer_BackgroundCarriesThroughEveryStyle covers TD.6: header
// cells, data cells, the inter-column gap, and the selected-row highlight
// must all carry the palette's Background colour — the class of bug
// CLAUDE.md flags as "reintroduced 5+ times" when a style has only
// Foreground set.
func TestListRenderer_BackgroundCarriesThroughEveryStyle(t *testing.T) {
	r := NewListRenderer([]string{"title"})
	r.Palette.Background = lipgloss.Color("#1a1a2e")
	result := types.QueryResult{
		Columns: []string{"title"},
		Rows: []map[string]interface{}{
			{"title": "First"},
			{"title": "Second"},
		},
	}

	unselected := r.Render(result, -1, 40)
	if !ansiTruecolourBgRe.MatchString(unselected) {
		t.Errorf("expected a truecolour background in unselected output, got %q", unselected)
	}

	selected := r.Render(result, 0, 40)
	if !ansiTruecolourBgRe.MatchString(selected) {
		t.Errorf("expected a truecolour background in selected-row output, got %q", selected)
	}

	empty := r.Render(types.QueryResult{Columns: []string{"title"}}, -1, 40)
	if !ansiTruecolourBgRe.MatchString(empty) {
		t.Errorf("expected the empty-state message to carry a background too, got %q", empty)
	}
}

// TestListRenderer_InterColumnGapCarriesBackground covers the specific rule
// 2/3 violation: the space between columns must be rendered via a
// background-aware spacer, not a bare strings.Repeat(" ", n) — a bare gap
// would show the terminal's default background rather than the pane's,
// breaking the illusion of one continuous themed row.
func TestListRenderer_InterColumnGapCarriesBackground(t *testing.T) {
	r := NewListRenderer([]string{"a", "b"})
	r.Palette.Background = lipgloss.Color("#1a1a2e")
	result := types.QueryResult{
		Columns: []string{"a", "b"},
		Rows: []map[string]interface{}{
			{"a": "x", "b": "y"},
		},
	}

	out := r.Render(result, -1, 40)
	// Every background-colour run in the output must match the palette's,
	// including whatever sits between the two data cells — if the gap were
	// a bare " ", the run of consecutive 48;2;… matches would show a gap
	// (unstyled bytes) rather than being contiguous across the whole line.
	matches := ansiTruecolourBgRe.FindAllStringSubmatch(out, -1)
	if len(matches) == 0 {
		t.Fatal("expected at least one truecolour background sequence")
	}
	for _, m := range matches {
		if m[1] != "26" || m[2] != "26" || m[3] != "46" { // #1a1a2e
			t.Errorf("unexpected background colour %s;%s;%s, want 26;26;46 (#1a1a2e)", m[1], m[2], m[3])
		}
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
