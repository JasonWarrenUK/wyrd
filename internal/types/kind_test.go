package types_test

import (
	"errors"
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// ---- Kind.Validate -------------------------------------------------------

func TestKindValidate(t *testing.T) {
	cases := []struct {
		name      string
		kind      types.Kind
		wantField string // empty means no error expected
	}{
		{
			name: "valid kind",
			kind: types.Kind{Name: "Task", StageGroup: "task-flow", Glyph: "◆", Colour: "#9b70ff"},
		},
		{
			name: "valid — missing glyph is allowed",
			kind: types.Kind{Name: "Task", StageGroup: "task-flow"},
		},
		{
			name: "valid — missing colour is allowed",
			kind: types.Kind{Name: "Task", StageGroup: "task-flow", Glyph: "◆"},
		},
		{
			name:      "empty name",
			kind:      types.Kind{StageGroup: "task-flow", Glyph: "◆", Colour: "#9b70ff"},
			wantField: "name",
		},
		{
			name:      "empty stage_group",
			kind:      types.Kind{Name: "Task", Glyph: "◆", Colour: "#9b70ff"},
			wantField: "stage_group",
		},
		{
			name:      "all empty",
			kind:      types.Kind{},
			wantField: "name",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.kind.Validate()
			if tc.wantField == "" {
				if err != nil {
					t.Errorf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() = nil, want ValidationError{Field:%q}", tc.wantField)
			}
			var ve *types.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("Validate() error type = %T, want *types.ValidationError", err)
			}
			if ve.Field != tc.wantField {
				t.Errorf("Validate() Field = %q, want %q", ve.Field, tc.wantField)
			}
		})
	}
}

// ---- KindRegistry -------------------------------------------------------

func TestKindRegistryLookup(t *testing.T) {
	kinds := []types.Kind{
		{Name: "Task", StageGroup: "task-flow", Glyph: "◆", Colour: "#9b70ff"},
		{Name: "Event", StageGroup: "event-flow", Glyph: "◇", Colour: "#d57300"},
	}
	reg := types.NewKindRegistry(kinds)

	k, ok := reg.Lookup("Task")
	if !ok {
		t.Fatal("Lookup(Task) ok = false, want true")
	}
	if k.StageGroup != "task-flow" {
		t.Errorf("Lookup(Task).StageGroup = %q, want %q", k.StageGroup, "task-flow")
	}

	_, ok = reg.Lookup("Nope")
	if ok {
		t.Error("Lookup(Nope) ok = true, want false")
	}
}

func TestKindRegistryOverrideByName(t *testing.T) {
	// Two kinds with the same name — last one wins. This is the SL.5 merge seam:
	// NewKindRegistry(append(defaults, userKinds...)) lets user kinds shadow defaults.
	kinds := []types.Kind{
		{Name: "Task", StageGroup: "task-flow", Glyph: "a", Colour: "#aaaaaa"},
		{Name: "Task", StageGroup: "task-flow", Glyph: "b", Colour: "#bbbbbb"},
	}
	reg := types.NewKindRegistry(kinds)

	k, ok := reg.Lookup("Task")
	if !ok {
		t.Fatal("Lookup(Task) ok = false")
	}
	if k.Glyph != "b" {
		t.Errorf("Lookup(Task).Glyph = %q, want %q (last entry should win)", k.Glyph, "b")
	}
	if len(reg.All()) != 1 {
		t.Errorf("len(All()) = %d, want 1 (duplicates should not inflate the list)", len(reg.All()))
	}
}

func TestKindRegistryOrderStable(t *testing.T) {
	names := []string{"Goblin", "Task", "Habit", "Event"}
	kinds := make([]types.Kind, len(names))
	for i, n := range names {
		kinds[i] = types.Kind{Name: n, StageGroup: "task-flow"}
	}
	reg := types.NewKindRegistry(kinds)

	got := reg.Names()
	if len(got) != len(names) {
		t.Fatalf("len(Names()) = %d, want %d", len(got), len(names))
	}
	for i, want := range names {
		if got[i] != want {
			t.Errorf("Names()[%d] = %q, want %q", i, got[i], want)
		}
	}
}

func TestKindRegistryEmpty(t *testing.T) {
	reg := types.NewKindRegistry(nil)

	if all := reg.All(); len(all) != 0 {
		t.Errorf("All() len = %d, want 0", len(all))
	}
	if names := reg.Names(); len(names) != 0 {
		t.Errorf("Names() len = %d, want 0", len(names))
	}
	_, ok := reg.Lookup("anything")
	if ok {
		t.Error("Lookup on empty registry returned ok = true")
	}
}

func TestKindRegistryAllDefensiveCopy(t *testing.T) {
	reg := types.NewKindRegistry([]types.Kind{
		{Name: "Task", StageGroup: "task-flow", Glyph: "◆", Colour: "#9b70ff"},
	})

	first := reg.All()
	first[0].Name = "Mutated"

	second := reg.All()
	if second[0].Name == "Mutated" {
		t.Error("mutation of All() slice affected registry internals")
	}
}
