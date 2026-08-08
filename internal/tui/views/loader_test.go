package views

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadView_SurvivesCommentMarkerInStringAndTrailingComma is the TD.1
// consolidation regression test for this consumer: a query string
// containing "//" (comment-marker lookalike) and a trailing comma in the
// JSONC must both be handled correctly by the shared internal/jsonc.Strip
// pass, rather than the previous local if-chain stripper.
func TestLoadView_SurvivesCommentMarkerInStringAndTrailingComma(t *testing.T) {
	dir := t.TempDir()
	viewsDir := filepath.Join(dir, "views")
	if err := os.MkdirAll(viewsDir, 0o755); err != nil {
		t.Fatalf("mkdir views: %v", err)
	}

	raw := `{
	// leading comment
	"name": "today",
	"query": "MATCH (n) WHERE n.body CONTAINS 'https://example.com/x' RETURN n",
	"pinned": true,
}`
	path := filepath.Join(viewsDir, "today.jsonc")
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write view file: %v", err)
	}

	view, err := LoadView(dir, "today")
	if err != nil {
		t.Fatalf("LoadView returned error: %v", err)
	}
	want := "MATCH (n) WHERE n.body CONTAINS 'https://example.com/x' RETURN n"
	if view.Query != want {
		t.Errorf("Query = %q, want %q", view.Query, want)
	}
	if !view.Pinned {
		t.Error("Pinned = false, want true (trailing comma should not break parsing)")
	}
}
