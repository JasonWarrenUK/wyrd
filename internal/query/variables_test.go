package query

import (
	"testing"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// fixedClock returns a StubClock fixed to the given time.
func fixedClock(t time.Time) types.Clock {
	return types.StubClock{Fixed: t}
}

// wednesday 2025-03-19 12:00:00 UTC — used as the reference time in tests
var refTime = time.Date(2025, 3, 19, 12, 0, 0, 0, time.UTC)

func TestResolveBuiltin_Now(t *testing.T) {
	bv := &BuiltinVariable{Name: "now"}
	result, err := resolveBuiltin(bv, fixedClock(refTime))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Equal(refTime) {
		t.Errorf("expected %v, got %v", refTime, result)
	}
}

func TestResolveBuiltin_Today(t *testing.T) {
	bv := &BuiltinVariable{Name: "today"}
	result, err := resolveBuiltin(bv, fixedClock(refTime))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2025, 3, 19, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestResolveBuiltin_WeekStart(t *testing.T) {
	// refTime is a Wednesday (2025-03-19).
	// Monday of that week is 2025-03-17.
	bv := &BuiltinVariable{Name: "week_start"}
	result, err := resolveBuiltin(bv, fixedClock(refTime))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2025, 3, 17, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("expected %v (Monday), got %v", expected, result)
	}
}

func TestResolveBuiltin_WeekStart_OnMonday(t *testing.T) {
	monday := time.Date(2025, 3, 17, 9, 0, 0, 0, time.UTC)
	bv := &BuiltinVariable{Name: "week_start"}
	result, err := resolveBuiltin(bv, fixedClock(monday))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2025, 3, 17, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestResolveBuiltin_WeekStart_OnSunday(t *testing.T) {
	sunday := time.Date(2025, 3, 23, 9, 0, 0, 0, time.UTC)
	bv := &BuiltinVariable{Name: "week_start"}
	result, err := resolveBuiltin(bv, fixedClock(sunday))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2025, 3, 17, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("expected Monday %v, got %v", expected, result)
	}
}

func TestResolveBuiltin_MonthStart(t *testing.T) {
	bv := &BuiltinVariable{Name: "month_start"}
	result, err := resolveBuiltin(bv, fixedClock(refTime))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestResolveBuiltin_Unknown(t *testing.T) {
	bv := &BuiltinVariable{Name: "yesterday"}
	_, err := resolveBuiltin(bv, fixedClock(refTime))
	if err == nil {
		t.Fatal("expected error for unknown built-in variable")
	}
}

// ---------------------------------------------------------------------------
// Date offset arithmetic
// ---------------------------------------------------------------------------

func TestResolveBuiltin_TodayPlusDays(t *testing.T) {
	bv := &BuiltinVariable{
		Name:   "today",
		Offset: &DateOffset{Sign: "+", Amount: 7, Unit: "d"},
	}
	result, err := resolveBuiltin(bv, fixedClock(refTime))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2025, 3, 26, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestResolveBuiltin_TodayMinusDays(t *testing.T) {
	bv := &BuiltinVariable{
		Name:   "today",
		Offset: &DateOffset{Sign: "-", Amount: 30, Unit: "d"},
	}
	result, err := resolveBuiltin(bv, fixedClock(refTime))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2025, 2, 17, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestResolveBuiltin_TodayPlusWeeks(t *testing.T) {
	bv := &BuiltinVariable{
		Name:   "today",
		Offset: &DateOffset{Sign: "+", Amount: 2, Unit: "w"},
	}
	result, err := resolveBuiltin(bv, fixedClock(refTime))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2025, 4, 2, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestResolveBuiltin_TodayPlusMonths(t *testing.T) {
	bv := &BuiltinVariable{
		Name:   "today",
		Offset: &DateOffset{Sign: "+", Amount: 1, Unit: "m"},
	}
	result, err := resolveBuiltin(bv, fixedClock(refTime))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2025, 4, 19, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestResolveBuiltin_TodayPlusYears(t *testing.T) {
	bv := &BuiltinVariable{
		Name:   "today",
		Offset: &DateOffset{Sign: "+", Amount: 1, Unit: "y"},
	}
	result, err := resolveBuiltin(bv, fixedClock(refTime))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestApplyDateOffset_InvalidUnit(t *testing.T) {
	_, err := applyDateOffset(refTime, &DateOffset{Sign: "+", Amount: 1, Unit: "x"})
	if err == nil {
		t.Fatal("expected error for unknown offset unit")
	}
}
