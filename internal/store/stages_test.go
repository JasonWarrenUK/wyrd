package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// newStagesTestStore creates a parent/store layout so stages.jsonc at
// filepath.Join(s.path, "..", "stages.jsonc") resolves to parent/stages.jsonc
// — the same geometry as the ReadConfig / ReadKinds tests.
func newStagesTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	parent := t.TempDir()
	storePath := filepath.Join(parent, "store")
	clock := &fixedClock{t: time.Date(2026, 3, 17, 10, 30, 0, 0, time.UTC)}
	s, err := New(storePath, clock)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return s, parent
}

// writeStagesFixture writes content as stages.jsonc in the parent directory.
func writeStagesFixture(t *testing.T, parent, content string) {
	t.Helper()
	path := filepath.Join(parent, "stages.jsonc")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing stages.jsonc fixture: %v", err)
	}
}

// ---------------------------------------------------------------------------

func TestReadStagesMissingFile(t *testing.T) {
	s, _ := newStagesTestStore(t)

	reg, err := s.ReadStages()
	if err != nil {
		t.Fatalf("ReadStages() error = %v, want nil (missing file is not an error)", err)
	}
	if reg == nil {
		t.Fatal("ReadStages() registry = nil, want empty non-nil registry")
	}
	if all := reg.All(); len(all) != 0 {
		t.Errorf("All() len = %d, want 0", len(all))
	}
}

func TestReadStagesParse(t *testing.T) {
	s, parent := newStagesTestStore(t)

	writeStagesFixture(t, parent, `[
		{"name": "review-flow",  "stages": ["Draft", "Review", "Approved"], "cycle": "terminate"},
		{"name": "sprint-flow", "stages": ["Backlog", "In Progress", "Done"], "cycle": "terminate"}
	]`)

	reg, err := s.ReadStages()
	if err != nil {
		t.Fatalf("ReadStages() error = %v", err)
	}
	if all := reg.All(); len(all) != 2 {
		t.Fatalf("len(All()) = %d, want 2", len(all))
	}

	review, ok := reg.Lookup("review-flow")
	if !ok {
		t.Fatal("Lookup(review-flow) ok = false")
	}
	if len(review.Stages) != 3 {
		t.Errorf("review-flow stages len = %d, want 3", len(review.Stages))
	}
	if review.Stages[0] != "Draft" {
		t.Errorf("review-flow first stage = %q, want %q", review.Stages[0], "Draft")
	}
	if review.Cycle != types.CycleTerminate {
		t.Errorf("review-flow cycle = %q, want terminate", review.Cycle)
	}

	_, ok = reg.Lookup("sprint-flow")
	if !ok {
		t.Error("Lookup(sprint-flow) ok = false")
	}
}

func TestReadStagesParentPath(t *testing.T) {
	// Regression guard against the config.jsonc trap: a file placed at the
	// store ROOT must be ignored; only the parent-level file is loaded.
	s, parent := newStagesTestStore(t)

	// Write to the WRONG location (store root — should be ignored).
	wrongPath := filepath.Join(s.path, "stages.jsonc")
	wrongContent := `[{"name": "wrong-flow", "stages": ["A", "B"], "cycle": "terminate"}]`
	if err := os.WriteFile(wrongPath, []byte(wrongContent), 0o644); err != nil {
		t.Fatalf("writing wrong stages.jsonc: %v", err)
	}

	// Nothing at the correct location yet — should return empty registry.
	reg, err := s.ReadStages()
	if err != nil {
		t.Fatalf("ReadStages() with only root-level file: error = %v", err)
	}
	if _, ok := reg.Lookup("wrong-flow"); ok {
		t.Error("Lookup(wrong-flow) ok = true: store-root stages.jsonc was read, expected parent only")
	}

	// Now write the correct one and confirm it is used.
	writeStagesFixture(t, parent, `[{"name": "right-flow", "stages": ["X", "Y"], "cycle": "terminate"}]`)
	reg2, err := s.ReadStages()
	if err != nil {
		t.Fatalf("ReadStages() with parent file: error = %v", err)
	}
	if _, ok := reg2.Lookup("right-flow"); !ok {
		t.Error("Lookup(right-flow) ok = false: parent stages.jsonc not loaded")
	}
}

func TestReadStagesStripsComments(t *testing.T) {
	s, parent := newStagesTestStore(t)

	// JSONC with // comments and a trailing comma — readJSONC/stripComments
	// should handle both.
	writeStagesFixture(t, parent, `[
		// A custom review progression.
		{
			"name": "review-flow",
			"stages": ["Draft", "Review", "Approved"],
			"cycle": "terminate", // trailing comma on last field
		},
	]`)

	reg, err := s.ReadStages()
	if err != nil {
		t.Fatalf("ReadStages() with comments: error = %v", err)
	}
	if _, ok := reg.Lookup("review-flow"); !ok {
		t.Error("Lookup(review-flow) ok = false after comment stripping")
	}
}

// TestReadStagesSurvivesCommentMarkerInStringAndTrailingComma is the TD.1
// consolidation regression test for this consumer: a stage name containing
// "//" (comment-marker lookalike) alongside a trailing comma must both be
// handled correctly by the shared internal/jsonc.Strip pass.
func TestReadStagesSurvivesCommentMarkerInStringAndTrailingComma(t *testing.T) {
	s, parent := newStagesTestStore(t)

	writeStagesFixture(t, parent, `[
		{
			"name": "review-flow",
			"stages": ["see https://example.com/x", "Approved"],
			"cycle": "terminate",
		},
	]`)

	reg, err := s.ReadStages()
	if err != nil {
		t.Fatalf("ReadStages(): error = %v", err)
	}
	g, ok := reg.Lookup("review-flow")
	if !ok {
		t.Fatal("Lookup(review-flow) ok = false")
	}
	if len(g.Stages) != 2 || g.Stages[0] != "see https://example.com/x" {
		t.Errorf("Stages = %v, want first stage to preserve the URL verbatim", g.Stages)
	}
}

func TestReadStagesLenientSkip(t *testing.T) {
	s, parent := newStagesTestStore(t)

	// Two invalid entries and one valid one. All invalid variants should be
	// silently skipped, leaving only the valid group in the registry.
	writeStagesFixture(t, parent, `[
		{"name": "good-flow", "stages": ["A", "B"], "cycle": "terminate"},
		{"name": "", "stages": ["A"], "cycle": "terminate"},
		{"name": "loop-bad", "stages": ["X", "Y"], "cycle": "loop-to-stage", "loop_target": "Nonexistent"}
	]`)

	reg, err := s.ReadStages()
	if err != nil {
		t.Fatalf("ReadStages() error = %v, want nil (lenient skip)", err)
	}
	if all := reg.All(); len(all) != 1 {
		t.Errorf("len(All()) = %d, want 1 (invalid entries should be skipped)", len(all))
	}
	if _, ok := reg.Lookup("good-flow"); !ok {
		t.Error("Lookup(good-flow) ok = false: valid group was skipped along with invalid ones")
	}
	if _, ok := reg.Lookup(""); ok {
		t.Error("Lookup(\"\") ok = true: empty-name group should have been skipped")
	}
	if _, ok := reg.Lookup("loop-bad"); ok {
		t.Error("Lookup(loop-bad) ok = true: bad loop_target should have caused the group to be skipped")
	}
}

func TestReadStagesParseError(t *testing.T) {
	s, parent := newStagesTestStore(t)

	// Whole-file corruption (unclosed array) — must return a ParseError.
	writeStagesFixture(t, parent, `[{"name": "bad-flow", "stages": ["A"`)

	_, err := s.ReadStages()
	if err == nil {
		t.Fatal("ReadStages() error = nil, want ParseError for malformed JSON")
	}
	var pe *types.ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("error type = %T, want *types.ParseError", err)
	}
	if pe.Source != "stages.jsonc" {
		t.Errorf("ParseError.Source = %q, want %q", pe.Source, "stages.jsonc")
	}
}

func TestWriteStagesRoundTrip(t *testing.T) {
	s, parent := newStagesTestStore(t)

	groups := []types.StageGroup{
		{Name: "review-flow", Stages: []string{"Draft", "Review", "Approved"}, Cycle: types.CycleTerminate},
		{Name: "habit-flow", Stages: []string{"Pending", "Done"}, Cycle: types.CycleLoop},
	}

	if err := s.WriteStages(groups); err != nil {
		t.Fatalf("WriteStages() error = %v", err)
	}

	// Confirm the file is at the parent location, not the store root.
	parentPath := filepath.Join(parent, "stages.jsonc")
	if _, err := os.Stat(parentPath); err != nil {
		t.Fatalf("stages.jsonc not at parent dir: %v", err)
	}
	storePath := filepath.Join(s.path, "stages.jsonc")
	if _, err := os.Stat(storePath); err == nil {
		t.Error("stages.jsonc unexpectedly written to store root; expected parent only")
	}

	reg, err := s.ReadStages()
	if err != nil {
		t.Fatalf("ReadStages() after WriteStages: error = %v", err)
	}
	if all := reg.All(); len(all) != 2 {
		t.Fatalf("len(All()) = %d, want 2", len(all))
	}

	review, ok := reg.Lookup("review-flow")
	if !ok {
		t.Fatal("Lookup(review-flow) ok = false after write+read round-trip")
	}
	if review.Cycle != types.CycleTerminate {
		t.Errorf("review-flow cycle = %q, want terminate", review.Cycle)
	}
	if len(review.Stages) != 3 || review.Stages[0] != "Draft" {
		t.Errorf("review-flow stages = %v, want [Draft Review Approved]", review.Stages)
	}

	_, ok = reg.Lookup("habit-flow")
	if !ok {
		t.Error("Lookup(habit-flow) ok = false after write+read round-trip")
	}
}

// TestWriteStagesOmitsEmptyShadowOf verifies that a group with an empty
// ShadowOf produces no "shadow_of" key in the raw JSONC bytes — omitempty
// keeps a hand-written stages.jsonc clean (TD.14).
func TestWriteStagesOmitsEmptyShadowOf(t *testing.T) {
	s, parent := newStagesTestStore(t)

	groups := []types.StageGroup{
		{Name: "review-flow", Stages: []string{"Draft", "Approved"}, Cycle: types.CycleTerminate},
	}
	if err := s.WriteStages(groups); err != nil {
		t.Fatalf("WriteStages() error = %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(parent, "stages.jsonc"))
	if err != nil {
		t.Fatalf("reading stages.jsonc: %v", err)
	}
	if strings.Contains(string(raw), "shadow_of") {
		t.Errorf("raw stages.jsonc contains \"shadow_of\" for a group with an empty ShadowOf:\n%s", raw)
	}
}

// TestWriteStagesRoundTripsShadowOf verifies a non-empty ShadowOf survives a
// write/read round trip (TD.14).
func TestWriteStagesRoundTripsShadowOf(t *testing.T) {
	s, _ := newStagesTestStore(t)

	want := "sha256:deadbeefdeadbeef"
	groups := []types.StageGroup{
		{Name: "review-flow", Stages: []string{"Draft", "Approved"}, Cycle: types.CycleTerminate, ShadowOf: want},
	}
	if err := s.WriteStages(groups); err != nil {
		t.Fatalf("WriteStages() error = %v", err)
	}

	reg, err := s.ReadStages()
	if err != nil {
		t.Fatalf("ReadStages() error = %v", err)
	}
	review, ok := reg.Lookup("review-flow")
	if !ok {
		t.Fatal("Lookup(review-flow) ok = false after write+read round-trip")
	}
	if review.ShadowOf != want {
		t.Errorf("ShadowOf = %q, want %q", review.ShadowOf, want)
	}
}

func TestWriteStagesOverwrite(t *testing.T) {
	s, _ := newStagesTestStore(t)

	// Write an initial set.
	initial := []types.StageGroup{
		{Name: "flow-a", Stages: []string{"X", "Y"}, Cycle: types.CycleTerminate},
	}
	if err := s.WriteStages(initial); err != nil {
		t.Fatalf("WriteStages() initial: error = %v", err)
	}

	// Overwrite with a different set — flow-a should be gone, flow-b present.
	updated := []types.StageGroup{
		{Name: "flow-b", Stages: []string{"P", "Q", "R"}, Cycle: types.CycleLoop},
	}
	if err := s.WriteStages(updated); err != nil {
		t.Fatalf("WriteStages() overwrite: error = %v", err)
	}

	reg, err := s.ReadStages()
	if err != nil {
		t.Fatalf("ReadStages() after overwrite: error = %v", err)
	}
	if _, ok := reg.Lookup("flow-a"); ok {
		t.Error("Lookup(flow-a) ok = true after overwrite: old group should be gone")
	}
	if _, ok := reg.Lookup("flow-b"); !ok {
		t.Error("Lookup(flow-b) ok = false: new group should be present")
	}
}

func TestWriteStagesLoopToStage(t *testing.T) {
	s, _ := newStagesTestStore(t)

	group := types.StageGroup{
		Name:       "sprint-flow",
		Stages:     []string{"Backlog", "In Progress", "Review", "Done"},
		Cycle:      types.CycleLoopToStage,
		LoopTarget: "Backlog",
	}

	if err := s.WriteStages([]types.StageGroup{group}); err != nil {
		t.Fatalf("WriteStages() loop-to-stage: error = %v", err)
	}

	reg, err := s.ReadStages()
	if err != nil {
		t.Fatalf("ReadStages() after WriteStages: error = %v", err)
	}

	g, ok := reg.Lookup("sprint-flow")
	if !ok {
		t.Fatal("Lookup(sprint-flow) ok = false")
	}
	if g.Cycle != types.CycleLoopToStage {
		t.Errorf("cycle = %q, want loop-to-stage", g.Cycle)
	}
	if g.LoopTarget != "Backlog" {
		t.Errorf("loop_target = %q, want Backlog", g.LoopTarget)
	}
}
