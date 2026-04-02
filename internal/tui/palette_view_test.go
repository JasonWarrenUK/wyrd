package tui

import (
	"strings"
	"testing"
)

// paletteWithTheme builds a PaletteState wired to the builtin theme.
// The theme is required for any test that calls View() — nil theme panics.
func paletteWithTheme(t *testing.T) PaletteState {
	t.Helper()
	theme := &ActiveTheme{
		raw:  builtinTheme(),
		tier: builtinTheme().Tiers.Truecolor,
	}
	ps := NewPaletteState(theme, nil)
	return ps
}

// ── title tests ──────────────────────────────────────────────────────────────

func TestFuzzyViewShowsSearchTitle(t *testing.T) {
	ps := paletteWithTheme(t)
	ps.Open(PaletteModeFuzzy)
	out := ps.View(80, 24)

	if !strings.Contains(out, "SEARCH") {
		t.Error("expected fuzzy overlay to contain 'SEARCH'")
	}
	if strings.Contains(out, "SEARCH COMMANDS") {
		t.Error("expected fuzzy overlay NOT to contain 'SEARCH COMMANDS'")
	}
}

func TestCLIViewShowsCommandPaletteTitle(t *testing.T) {
	ps := paletteWithTheme(t)
	ps.Open(PaletteModeCLI)
	out := ps.View(80, 24)

	if !strings.Contains(out, "COMMAND PALETTE") {
		t.Error("expected CLI overlay to contain 'COMMAND PALETTE'")
	}
}

// ── kind prefix tests ────────────────────────────────────────────────────────

func TestFuzzyViewRendersKindPrefixes(t *testing.T) {
	ps := paletteWithTheme(t)
	ps.active = true
	ps.mode = PaletteModeFuzzy
	ps.results = []SearchResult{
		{Kind: SearchResultCommand, Title: "quit", Description: "Exit", Command: &Command{Name: "quit"}},
		{Kind: SearchResultNode, Title: "My note", Types: []string{"note"}, NodeID: "n1"},
		{Kind: SearchResultEdge, Title: "blocks: A → B", NodeID: "n2", EdgeID: "e1"},
	}

	out := ps.View(80, 24)

	if !strings.Contains(out, ">") {
		t.Error("expected command prefix '>' in output")
	}
	if !strings.Contains(out, "◆") {
		t.Error("expected node prefix '◆' in output")
	}
	if !strings.Contains(out, "→") {
		t.Error("expected edge prefix '→' in output")
	}
}

func TestCLIViewDoesNotRenderKindPrefixes(t *testing.T) {
	ps := paletteWithTheme(t)
	ps.Open(PaletteModeCLI)
	out := ps.View(80, 24)

	if strings.Contains(out, "◆") {
		t.Error("expected CLI overlay NOT to contain node prefix '◆'")
	}
}

// ── node badge tests ─────────────────────────────────────────────────────────

func TestFuzzyViewNodeShowsBadges(t *testing.T) {
	ps := paletteWithTheme(t)
	ps.active = true
	ps.mode = PaletteModeFuzzy
	ps.results = []SearchResult{
		{Kind: SearchResultNode, Title: "Sprint planning", Types: []string{"task"}, NodeID: "n1"},
	}

	out := ps.View(80, 24)

	if !strings.Contains(out, "task") {
		t.Errorf("expected type badge 'task' in node row output, got: %q", out)
	}
}

// ── empty results ────────────────────────────────────────────────────────────

func TestFuzzyViewEmptyResults(t *testing.T) {
	ps := paletteWithTheme(t)
	ps.active = true
	ps.mode = PaletteModeFuzzy
	ps.results = []SearchResult{}

	out := ps.View(80, 24)

	if !strings.Contains(out, "No results") {
		t.Errorf("expected 'No results' in empty fuzzy overlay, got: %q", out)
	}
}

// ── filterFuzzy no longer populates ps.filtered ──────────────────────────────

func TestFilterFuzzyDoesNotPopulateFiltered(t *testing.T) {
	ps := paletteWithTheme(t)
	// filterFuzzy with no index returns command-only results; filtered stays empty.
	ps.filterFuzzy("")
	if len(ps.filtered) != 0 {
		t.Errorf("expected ps.filtered to be empty after filterFuzzy, got %d entries", len(ps.filtered))
	}
	// ps.results should be populated (at least the builtin commands).
	if len(ps.results) == 0 {
		t.Error("expected ps.results to be non-empty after filterFuzzy with empty query")
	}
}
