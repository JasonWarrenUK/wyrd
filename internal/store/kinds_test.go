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

// newKindsTestStore creates a parent/store layout so kinds.jsonc at
// filepath.Join(s.path, "..", "kinds.jsonc") resolves to parent/kinds.jsonc
// — the same geometry as the ReadConfig / WriteConfig tests.
func newKindsTestStore(t *testing.T) (*Store, string) {
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

// writeKindsFixture writes content as kinds.jsonc in the parent directory.
func writeKindsFixture(t *testing.T, parent, content string) {
	t.Helper()
	path := filepath.Join(parent, "kinds.jsonc")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing kinds.jsonc fixture: %v", err)
	}
}

// ---------------------------------------------------------------------------

func TestReadKindsMissingFile(t *testing.T) {
	s, _ := newKindsTestStore(t)

	reg, err := s.ReadKinds()
	if err != nil {
		t.Fatalf("ReadKinds() error = %v, want nil (missing file is not an error)", err)
	}
	if reg == nil {
		t.Fatal("ReadKinds() registry = nil, want empty non-nil registry")
	}
	if all := reg.All(); len(all) != 0 {
		t.Errorf("All() len = %d, want 0", len(all))
	}
}

func TestReadKindsParse(t *testing.T) {
	s, parent := newKindsTestStore(t)

	writeKindsFixture(t, parent, `[
		{"name": "Task",  "stage_group": "task-flow",  "glyph": "◆", "colour": "#9b70ff"},
		{"name": "Event", "stage_group": "event-flow", "glyph": "◇", "colour": "#d57300"}
	]`)

	reg, err := s.ReadKinds()
	if err != nil {
		t.Fatalf("ReadKinds() error = %v", err)
	}
	if all := reg.All(); len(all) != 2 {
		t.Fatalf("len(All()) = %d, want 2", len(all))
	}

	task, ok := reg.Lookup("Task")
	if !ok {
		t.Fatal("Lookup(Task) ok = false")
	}
	if task.StageGroup != "task-flow" {
		t.Errorf("Task.StageGroup = %q, want %q", task.StageGroup, "task-flow")
	}
	if task.Glyph != "◆" {
		t.Errorf("Task.Glyph = %q, want %q", task.Glyph, "◆")
	}
	if task.Colour != "#9b70ff" {
		t.Errorf("Task.Colour = %q, want %q", task.Colour, "#9b70ff")
	}

	_, ok = reg.Lookup("Event")
	if !ok {
		t.Error("Lookup(Event) ok = false")
	}
}

func TestReadKindsParentPath(t *testing.T) {
	// Regression guard against the config.jsonc trap: a file placed at the
	// store ROOT must be ignored; only the parent-level file is loaded.
	s, parent := newKindsTestStore(t)

	// Write to the WRONG location (store root — should be ignored).
	wrongPath := filepath.Join(s.path, "kinds.jsonc")
	wrongContent := `[{"name": "WrongKind", "stage_group": "task-flow", "glyph": "✗", "colour": "#ff0000"}]`
	if err := os.WriteFile(wrongPath, []byte(wrongContent), 0o644); err != nil {
		t.Fatalf("writing wrong kinds.jsonc: %v", err)
	}

	// Nothing at the correct location yet — should return empty registry.
	reg, err := s.ReadKinds()
	if err != nil {
		t.Fatalf("ReadKinds() with only root-level file: error = %v", err)
	}
	if _, ok := reg.Lookup("WrongKind"); ok {
		t.Error("Lookup(WrongKind) ok = true: store-root kinds.jsonc was read, expected parent only")
	}

	// Now write the correct one and confirm it is used.
	writeKindsFixture(t, parent, `[{"name": "RightKind", "stage_group": "task-flow", "glyph": "✓", "colour": "#00ff00"}]`)
	reg2, err := s.ReadKinds()
	if err != nil {
		t.Fatalf("ReadKinds() with parent file: error = %v", err)
	}
	if _, ok := reg2.Lookup("RightKind"); !ok {
		t.Error("Lookup(RightKind) ok = false: parent kinds.jsonc not loaded")
	}
}

func TestReadKindsStripsComments(t *testing.T) {
	s, parent := newKindsTestStore(t)

	// JSONC with // comments and a trailing comma — readJSONC/stripComments
	// should handle both.
	writeKindsFixture(t, parent, `[
		// This is a task kind.
		{
			"name": "Task",
			"stage_group": "task-flow",
			"glyph": "◆",
			"colour": "#9b70ff", // trailing comma on last field
		},
	]`)

	reg, err := s.ReadKinds()
	if err != nil {
		t.Fatalf("ReadKinds() with comments: error = %v", err)
	}
	if _, ok := reg.Lookup("Task"); !ok {
		t.Error("Lookup(Task) ok = false after comment stripping")
	}
}

func TestReadKindsLenientSkip(t *testing.T) {
	s, parent := newKindsTestStore(t)

	// One valid kind and one missing stage_group — the invalid entry should
	// be silently skipped, leaving only the valid kind in the registry.
	writeKindsFixture(t, parent, `[
		{"name": "Task",  "stage_group": "task-flow", "glyph": "◆", "colour": "#9b70ff"},
		{"name": "Bad",   "stage_group": "",           "glyph": "✗", "colour": "#ff0000"}
	]`)

	reg, err := s.ReadKinds()
	if err != nil {
		t.Fatalf("ReadKinds() error = %v, want nil (lenient skip)", err)
	}
	if all := reg.All(); len(all) != 1 {
		t.Errorf("len(All()) = %d, want 1 (bad entry should be skipped)", len(all))
	}
	if _, ok := reg.Lookup("Task"); !ok {
		t.Error("Lookup(Task) ok = false: valid kind was skipped along with invalid one")
	}
	if _, ok := reg.Lookup("Bad"); ok {
		t.Error("Lookup(Bad) ok = true: invalid kind (empty stage_group) should have been skipped")
	}
}

func TestReadKindsParseError(t *testing.T) {
	s, parent := newKindsTestStore(t)

	// Whole-file corruption (unclosed array) — must return a ParseError.
	writeKindsFixture(t, parent, `[{"name": "Task", "stage_group": "task-flow"`)

	_, err := s.ReadKinds()
	if err == nil {
		t.Fatal("ReadKinds() error = nil, want ParseError for malformed JSON")
	}
	var pe *types.ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("error type = %T, want *types.ParseError", err)
	}
	if pe.Source != "kinds.jsonc" {
		t.Errorf("ParseError.Source = %q, want %q", pe.Source, "kinds.jsonc")
	}
}

func TestWriteKindsRoundTrip(t *testing.T) {
	s, parent := newKindsTestStore(t)

	kinds := []types.Kind{
		{Name: "Errand", StageGroup: "task-flow", Glyph: "!", Colour: "#9b70ff"},
		{Name: "Chore", StageGroup: "habit-flow", Glyph: "~", Colour: "#d57300"},
	}

	if err := s.WriteKinds(kinds); err != nil {
		t.Fatalf("WriteKinds() error = %v", err)
	}

	// Confirm the file is at the parent location, not the store root.
	parentPath := filepath.Join(parent, "kinds.jsonc")
	if _, err := os.Stat(parentPath); err != nil {
		t.Fatalf("kinds.jsonc not at parent dir: %v", err)
	}
	storePath := filepath.Join(s.path, "kinds.jsonc")
	if _, err := os.Stat(storePath); err == nil {
		t.Error("kinds.jsonc unexpectedly written to store root; expected parent only")
	}

	reg, err := s.ReadKinds()
	if err != nil {
		t.Fatalf("ReadKinds() after WriteKinds: error = %v", err)
	}
	if all := reg.All(); len(all) != 2 {
		t.Fatalf("len(All()) = %d, want 2", len(all))
	}

	errand, ok := reg.Lookup("Errand")
	if !ok {
		t.Fatal("Lookup(Errand) ok = false after write+read round-trip")
	}
	if errand.StageGroup != "task-flow" {
		t.Errorf("Errand.StageGroup = %q, want %q", errand.StageGroup, "task-flow")
	}
	if errand.Glyph != "!" {
		t.Errorf("Errand.Glyph = %q, want %q", errand.Glyph, "!")
	}

	_, ok = reg.Lookup("Chore")
	if !ok {
		t.Error("Lookup(Chore) ok = false after write+read round-trip")
	}
}

// TestWriteKindsOmitsEmptyShadowOf verifies that a kind with an empty
// ShadowOf produces no "shadow_of" key in the raw JSONC bytes — omitempty
// keeps a hand-written kinds.jsonc clean, which is the reason it's required
// rather than optional on the field (TD.14).
func TestWriteKindsOmitsEmptyShadowOf(t *testing.T) {
	s, parent := newKindsTestStore(t)

	kinds := []types.Kind{
		{Name: "Errand", StageGroup: "task-flow", Glyph: "!", Colour: "#9b70ff"},
	}
	if err := s.WriteKinds(kinds); err != nil {
		t.Fatalf("WriteKinds() error = %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(parent, "kinds.jsonc"))
	if err != nil {
		t.Fatalf("reading kinds.jsonc: %v", err)
	}
	if strings.Contains(string(raw), "shadow_of") {
		t.Errorf("raw kinds.jsonc contains \"shadow_of\" for a kind with an empty ShadowOf:\n%s", raw)
	}
}

// TestWriteKindsRoundTripsShadowOf verifies a non-empty ShadowOf survives a
// write/read round trip (TD.14).
func TestWriteKindsRoundTripsShadowOf(t *testing.T) {
	s, _ := newKindsTestStore(t)

	want := "sha256:deadbeefdeadbeef"
	kinds := []types.Kind{
		{Name: "Errand", StageGroup: "task-flow", Glyph: "!", Colour: "#9b70ff", ShadowOf: want},
	}
	if err := s.WriteKinds(kinds); err != nil {
		t.Fatalf("WriteKinds() error = %v", err)
	}

	reg, err := s.ReadKinds()
	if err != nil {
		t.Fatalf("ReadKinds() error = %v", err)
	}
	errand, ok := reg.Lookup("Errand")
	if !ok {
		t.Fatal("Lookup(Errand) ok = false after write+read round-trip")
	}
	if errand.ShadowOf != want {
		t.Errorf("ShadowOf = %q, want %q", errand.ShadowOf, want)
	}
}

// TestWriteKindsOmitsEmptyShadowSource mirrors
// TestWriteKindsOmitsEmptyShadowOf for TD.18b's ShadowSource — the pointer
// type only pays off if omitempty genuinely drops it from a nil field on
// real disk bytes, not just in an in-memory hash comparison.
func TestWriteKindsOmitsEmptyShadowSource(t *testing.T) {
	s, parent := newKindsTestStore(t)

	kinds := []types.Kind{
		{Name: "Errand", StageGroup: "task-flow", Glyph: "!", Colour: "#9b70ff"},
	}
	if err := s.WriteKinds(kinds); err != nil {
		t.Fatalf("WriteKinds() error = %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(parent, "kinds.jsonc"))
	if err != nil {
		t.Fatalf("reading kinds.jsonc: %v", err)
	}
	if strings.Contains(string(raw), "shadow_source") {
		t.Errorf("raw kinds.jsonc contains \"shadow_source\" for a kind with a nil ShadowSource:\n%s", raw)
	}
}

// TestWriteKindsRoundTripsShadowSource verifies a non-nil ShadowSource
// survives a write/read round trip (TD.18b), mirroring
// TestWriteKindsRoundTripsShadowOf.
func TestWriteKindsRoundTripsShadowSource(t *testing.T) {
	s, _ := newKindsTestStore(t)

	source := &types.Kind{Name: "Task", StageGroup: "task-flow", Glyph: "◆", Colour: "#9b70ff"}
	kinds := []types.Kind{
		{
			Name: "Errand", StageGroup: "task-flow", Glyph: "!", Colour: "#9b70ff",
			ShadowOf:     "sha256:deadbeefdeadbeef",
			ShadowSource: source,
		},
	}
	if err := s.WriteKinds(kinds); err != nil {
		t.Fatalf("WriteKinds() error = %v", err)
	}

	reg, err := s.ReadKinds()
	if err != nil {
		t.Fatalf("ReadKinds() error = %v", err)
	}
	errand, ok := reg.Lookup("Errand")
	if !ok {
		t.Fatal("Lookup(Errand) ok = false after write+read round-trip")
	}
	if errand.ShadowSource == nil {
		t.Fatal("ShadowSource = nil after write+read round-trip")
	}
	if *errand.ShadowSource != *source {
		t.Errorf("ShadowSource = %+v, want %+v", *errand.ShadowSource, *source)
	}
}

func TestWriteKindsOverwrite(t *testing.T) {
	s, _ := newKindsTestStore(t)

	// Write an initial set.
	initial := []types.Kind{
		{Name: "KindA", StageGroup: "task-flow", Glyph: "A", Colour: "#111111"},
	}
	if err := s.WriteKinds(initial); err != nil {
		t.Fatalf("WriteKinds() initial: error = %v", err)
	}

	// Overwrite with a different set — KindA should be gone, KindB present.
	updated := []types.Kind{
		{Name: "KindB", StageGroup: "event-flow", Glyph: "B", Colour: "#222222"},
	}
	if err := s.WriteKinds(updated); err != nil {
		t.Fatalf("WriteKinds() overwrite: error = %v", err)
	}

	reg, err := s.ReadKinds()
	if err != nil {
		t.Fatalf("ReadKinds() after overwrite: error = %v", err)
	}
	if _, ok := reg.Lookup("KindA"); ok {
		t.Error("Lookup(KindA) ok = true after overwrite: old kind should be gone")
	}
	if _, ok := reg.Lookup("KindB"); !ok {
		t.Error("Lookup(KindB) ok = false: new kind should be present")
	}
}

func TestWriteKindsIntoExistingParent(t *testing.T) {
	// Writing kinds into a parent dir that already holds a kinds.jsonc (e.g.
	// hand-authored) must overwrite it cleanly rather than merging or erroring.
	s, parent := newKindsTestStore(t)

	writeKindsFixture(t, parent, `[{"name": "HandWritten", "stage_group": "task-flow", "glyph": "#", "colour": "#333333"}]`)

	updated := []types.Kind{
		{Name: "FromForm", StageGroup: "task-flow", Glyph: "$", Colour: "#444444"},
	}
	if err := s.WriteKinds(updated); err != nil {
		t.Fatalf("WriteKinds() over existing file: error = %v", err)
	}

	reg, err := s.ReadKinds()
	if err != nil {
		t.Fatalf("ReadKinds() after WriteKinds: error = %v", err)
	}
	if _, ok := reg.Lookup("HandWritten"); ok {
		t.Error("Lookup(HandWritten) ok = true: hand-authored kind should have been overwritten")
	}
	if _, ok := reg.Lookup("FromForm"); !ok {
		t.Error("Lookup(FromForm) ok = false: new kind should be present")
	}
}
