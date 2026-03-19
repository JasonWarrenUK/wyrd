package query

import (
	"testing"
)

func TestApplyAggregate_Count(t *testing.T) {
	v, err := applyAggregate("count", []interface{}{1, 2, 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != int64(3) {
		t.Errorf("expected 3, got %v", v)
	}
}

func TestApplyAggregate_CountEmpty(t *testing.T) {
	v, err := applyAggregate("count", []interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != int64(0) {
		t.Errorf("expected 0, got %v", v)
	}
}

func TestApplyAggregate_Sum(t *testing.T) {
	v, err := applyAggregate("sum", []interface{}{int64(1), int64(2), int64(3)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != int64(6) {
		t.Errorf("expected 6, got %v", v)
	}
}

func TestApplyAggregate_SumFloat(t *testing.T) {
	v, err := applyAggregate("sum", []interface{}{1.5, 2.5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// When the floating-point result is a whole number it is returned as int64
	// for cleaner output; accept both numeric representations.
	f, floatErr := toFloat(v)
	if floatErr != nil {
		t.Fatalf("expected numeric result, got %T: %v", v, v)
	}
	if f != 4.0 {
		t.Errorf("expected 4.0, got %v", f)
	}
}

func TestApplyAggregate_Avg(t *testing.T) {
	v, err := applyAggregate("avg", []interface{}{int64(2), int64(4), int64(6)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := v.(float64)
	if !ok {
		t.Fatalf("expected float64, got %T", v)
	}
	if f != 4.0 {
		t.Errorf("expected 4.0, got %v", f)
	}
}

func TestApplyAggregate_AvgEmpty(t *testing.T) {
	v, err := applyAggregate("avg", []interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != nil {
		t.Errorf("expected nil for avg of empty slice, got %v", v)
	}
}

func TestApplyAggregate_Min(t *testing.T) {
	v, err := applyAggregate("min", []interface{}{int64(3), int64(1), int64(2)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// min returns a comparable value; it comes back from sort.
	// All values are int64, so toFloat comparison applies.
	if compareLess(v, int64(1)) {
		t.Errorf("expected min to be 1, got %v (which is less than 1)", v)
	}
	if compareLess(int64(1), v) {
		t.Errorf("expected min to be 1, got %v (which is greater than 1)", v)
	}
}

func TestApplyAggregate_Max(t *testing.T) {
	v, err := applyAggregate("max", []interface{}{int64(3), int64(1), int64(2)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if compareLess(v, int64(3)) || compareLess(int64(3), v) {
		t.Errorf("expected max to be 3, got %v", v)
	}
}

func TestApplyAggregate_Collect(t *testing.T) {
	v, err := applyAggregate("collect", []interface{}{"a", nil, "b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	slice, ok := v.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", v)
	}
	if len(slice) != 2 {
		t.Errorf("expected 2 items (nil excluded), got %d", len(slice))
	}
}

func TestApplyAggregate_Unknown(t *testing.T) {
	_, err := applyAggregate("mode", []interface{}{1, 2})
	if err == nil {
		t.Fatal("expected error for unknown aggregate")
	}
}

func TestIsAggregateFunction(t *testing.T) {
	for _, name := range []string{"count", "sum", "avg", "min", "max", "collect"} {
		if !isAggregateFunction(name) {
			t.Errorf("expected %q to be recognised as aggregate", name)
		}
	}
	if isAggregateFunction("unknown") {
		t.Error("expected 'unknown' not to be an aggregate")
	}
}

func TestCompareLess_Numbers(t *testing.T) {
	if !compareLess(int64(1), int64(2)) {
		t.Error("expected 1 < 2")
	}
	if compareLess(int64(2), int64(1)) {
		t.Error("expected 2 not < 1")
	}
}

func TestCompareLess_Strings(t *testing.T) {
	if !compareLess("apple", "banana") {
		t.Error("expected 'apple' < 'banana'")
	}
}
