package movement_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jasonwarrenuk/wyrd/internal/movement"
	"github.com/jasonwarrenuk/wyrd/internal/stage"
	"github.com/jasonwarrenuk/wyrd/internal/store"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// --- Round-trip and coercion ---

func TestApplyToThenFromNode(t *testing.T) {
	date := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	mv := movement.New(12.34, date, "coffee")

	node := mv.ApplyTo(nil)

	got, err := movement.FromNode(node)
	if err != nil {
		t.Fatalf("FromNode: %v", err)
	}
	if got.Amount != mv.Amount {
		t.Errorf("Amount = %v, want %v", got.Amount, mv.Amount)
	}
	if !got.Date.Equal(mv.Date) {
		t.Errorf("Date = %v, want %v", got.Date, mv.Date)
	}
	if got.Note != mv.Note {
		t.Errorf("Note = %q, want %q", got.Note, mv.Note)
	}
	if got.Stage != movement.StageExpected {
		t.Errorf("Stage = %q, want %q", got.Stage, movement.StageExpected)
	}
}

func TestFromNodeAfterJSONRoundTrip(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}

	date := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	mv := movement.New(12.34, date, "coffee")

	node := mv.ApplyTo(nil)
	node.ID = uuid.New().String()
	node.Types = []string{"movement"}
	node.Date.Created = date
	node.Date.Modified = date

	if err := s.WriteNode(node); err != nil {
		t.Fatalf("WriteNode: %v", err)
	}

	reread, err := s.ReadNode(node.ID)
	if err != nil {
		t.Fatalf("ReadNode: %v", err)
	}

	got, err := movement.FromNode(reread)
	if err != nil {
		t.Fatalf("FromNode: %v", err)
	}
	if got.Amount != 12.34 {
		t.Errorf("Amount after round-trip = %v, want 12.34 (Properties flatten must not eat amount)", got.Amount)
	}
	if !got.Date.Equal(date) {
		t.Errorf("Date after round-trip = %v, want %v (Date.About must survive nested date encoding)", got.Date, date)
	}
	if got.Note != "coffee" {
		t.Errorf("Note after round-trip = %q, want %q", got.Note, "coffee")
	}
}

func TestAmountCoercion(t *testing.T) {
	cases := []struct {
		name string
		val  interface{}
		want float64
	}{
		{"float64", float64(5), 5},
		{"int", int(5), 5},
		{"int64", int64(5), 5},
		{"float32", float32(5), 5},
		{"json.Number", json.Number("5"), 5},
		{"string", "5", 5},
		{"missing key", nil, 0},
		{"bool", true, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node := &types.Node{Properties: map[string]interface{}{}}
			if tc.name != "missing key" {
				node.Properties[movement.PropAmount] = tc.val
			}
			got := movement.Amount(node)
			if got != tc.want {
				t.Errorf("Amount() = %v, want %v", got, tc.want)
			}
		})
	}

	t.Run("nil Properties", func(t *testing.T) {
		node := &types.Node{}
		if got := movement.Amount(node); got != 0 {
			t.Errorf("Amount() = %v, want 0", got)
		}
	})

	t.Run("nil node", func(t *testing.T) {
		if got := movement.Amount(nil); got != 0 {
			t.Errorf("Amount(nil) = %v, want 0", got)
		}
	})
}

// --- ApplyTo semantics ---

func TestApplyToDoesNotMutateInput(t *testing.T) {
	date := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	original := &types.Node{
		Properties: map[string]interface{}{movement.PropAmount: float64(1)},
		Date:       types.DateFields{About: &date},
	}

	newDate := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	mv := movement.New(99, newDate, "changed")
	out := mv.ApplyTo(original)

	if out == original {
		t.Fatal("ApplyTo returned the same pointer as its input")
	}
	if movement.Amount(original) != 1 {
		t.Errorf("original amount mutated: got %v, want 1", movement.Amount(original))
	}
	if !original.Date.About.Equal(date) {
		t.Errorf("original Date.About mutated: got %v, want %v", original.Date.About, date)
	}
}

func TestApplyToPreservesUnrelatedProperties(t *testing.T) {
	original := &types.Node{
		Properties: map[string]interface{}{"category": "groceries"},
	}

	mv := movement.New(10, time.Now(), "shop")
	out := mv.ApplyTo(original)

	if out.Properties["category"] != "groceries" {
		t.Errorf("category property lost: got %v", out.Properties["category"])
	}
}

func TestApplyToNilProperties(t *testing.T) {
	original := &types.Node{}
	mv := movement.New(10, time.Now(), "shop")

	out := mv.ApplyTo(original)
	if out.Properties == nil {
		t.Fatal("ApplyTo left Properties nil")
	}
	if movement.Amount(out) != 10 {
		t.Errorf("Amount = %v, want 10", movement.Amount(out))
	}
}

func TestApplyToStampsKindAndStage(t *testing.T) {
	mv := movement.New(10, time.Now(), "shop")
	out := mv.ApplyTo(nil)

	if out.Kind != movement.KindName {
		t.Errorf("Kind = %q, want %q", out.Kind, movement.KindName)
	}
	if out.Stage != movement.StageExpected {
		t.Errorf("Stage = %q, want %q", out.Stage, movement.StageExpected)
	}

	existing := &types.Node{Stage: movement.StageCleared}
	out2 := mv.ApplyTo(existing)
	if out2.Stage != movement.StageCleared {
		t.Errorf("Stage = %q, want existing Cleared stage preserved", out2.Stage)
	}
}

func TestApplyToNilNode(t *testing.T) {
	mv := movement.New(10, time.Now(), "shop")
	out := mv.ApplyTo(nil)
	if out == nil {
		t.Fatal("ApplyTo(nil) returned nil")
	}
	if movement.Amount(out) != 10 {
		t.Errorf("Amount = %v, want 10", movement.Amount(out))
	}
}

// --- Validation ---

func TestValidateRejectsZeroAndNegativeAmount(t *testing.T) {
	for _, amount := range []float64{0, -5} {
		mv := movement.New(amount, time.Now(), "x")
		err := mv.Validate()
		var ve *types.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("Validate() for amount %v: want *types.ValidationError, got %v", amount, err)
		}
		if ve.Field != "amount" {
			t.Errorf("Field = %q, want %q", ve.Field, "amount")
		}
	}
}

func TestValidateRejectsZeroDate(t *testing.T) {
	mv := movement.New(10, time.Time{}, "x")
	err := mv.Validate()
	var ve *types.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("Validate(): want *types.ValidationError, got %v", err)
	}
	if ve.Field != "date" {
		t.Errorf("Field = %q, want %q", ve.Field, "date")
	}
}

func TestValidateRejectsUnknownStage(t *testing.T) {
	mv := movement.New(10, time.Now(), "x")
	mv.Stage = "Bogus"
	err := mv.Validate()
	var ve *types.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("Validate(): want *types.ValidationError, got %v", err)
	}
	if ve.Field != "stage" {
		t.Errorf("Field = %q, want %q", ve.Field, "stage")
	}
}

func TestValidateAcceptsEmptyNoteAndEmptyStage(t *testing.T) {
	mv := movement.New(10, time.Now(), "")
	mv.Stage = ""
	if err := mv.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

// --- FromNode edges ---

func TestFromNodeRejectsWrongKind(t *testing.T) {
	node := &types.Node{Kind: "Budget"}
	_, err := movement.FromNode(node)
	var ve *types.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("FromNode(): want *types.ValidationError, got %v", err)
	}
	if ve.Field != "kind" {
		t.Errorf("Field = %q, want %q", ve.Field, "kind")
	}
}

func TestFromNodeToleratesMissingAmount(t *testing.T) {
	node := &types.Node{Kind: movement.KindName}
	got, err := movement.FromNode(node)
	if err != nil {
		t.Fatalf("FromNode(): want nil error, got %v", err)
	}
	if got.Amount != 0 {
		t.Errorf("Amount = %v, want 0", got.Amount)
	}
}

func TestFromNodeToleratesNilAboutDate(t *testing.T) {
	node := &types.Node{Kind: movement.KindName}
	got, err := movement.FromNode(node)
	if err != nil {
		t.Fatalf("FromNode(): want nil error, got %v", err)
	}
	if !got.Date.IsZero() {
		t.Errorf("Date = %v, want zero value", got.Date)
	}
}

// --- Predicates ---

func TestIsMovementAndIsCleared(t *testing.T) {
	if movement.IsMovement(nil) {
		t.Error("IsMovement(nil) = true, want false")
	}
	if movement.IsCleared(nil) {
		t.Error("IsCleared(nil) = true, want false")
	}

	movementNode := &types.Node{Kind: movement.KindName, Stage: movement.StageCleared}
	if !movement.IsMovement(movementNode) {
		t.Error("IsMovement() = false, want true")
	}
	if !movement.IsCleared(movementNode) {
		t.Error("IsCleared() = false, want true")
	}

	other := &types.Node{Kind: "Budget", Stage: movement.StageCleared}
	if movement.IsMovement(other) {
		t.Error("IsMovement() = true for non-movement kind, want false")
	}

	expected := &types.Node{Kind: movement.KindName, Stage: movement.StageExpected}
	if movement.IsCleared(expected) {
		t.Error("IsCleared() = true for Expected stage, want false")
	}
}

// --- Constants ---

func TestKindNameMatchesRegistry(t *testing.T) {
	kinds, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds(): %v", err)
	}

	var found *types.Kind
	for i := range kinds {
		if kinds[i].Name == movement.KindName {
			found = &kinds[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("no default kind named %q", movement.KindName)
	}
	if found.StageGroup != movement.StageGroupName {
		t.Errorf("kind %q stage_group = %q, want %q", movement.KindName, found.StageGroup, movement.StageGroupName)
	}

	groups, err := stage.DefaultStageGroups()
	if err != nil {
		t.Fatalf("DefaultStageGroups(): %v", err)
	}

	var group *types.StageGroup
	for i := range groups {
		if groups[i].Name == movement.StageGroupName {
			group = &groups[i]
			break
		}
	}
	if group == nil {
		t.Fatalf("no default stage group named %q", movement.StageGroupName)
	}
	wantStages := []string{movement.StageExpected, movement.StageCleared}
	if len(group.Stages) != len(wantStages) {
		t.Fatalf("group.Stages = %v, want %v", group.Stages, wantStages)
	}
	for i, want := range wantStages {
		if group.Stages[i] != want {
			t.Errorf("group.Stages[%d] = %q, want %q", i, group.Stages[i], want)
		}
	}
}
