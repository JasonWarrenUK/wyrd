package query

import (
	"fmt"
	"math"
	"sort"
)

// aggregateResult holds intermediate state for a single aggregation.
type aggregateResult struct {
	name   string
	values []interface{}
}

// applyAggregate computes a final aggregate value from a slice of raw values.
// Supported functions: count, sum, avg, min, max, collect.
func applyAggregate(name string, values []interface{}) (interface{}, error) {
	switch name {
	case "count":
		return int64(len(values)), nil

	case "sum":
		return sumValues(values)

	case "avg":
		return avgValues(values)

	case "min":
		return minValue(values)

	case "max":
		return maxValue(values)

	case "collect":
		// collect() returns the raw slice as-is (nil values excluded).
		out := make([]interface{}, 0, len(values))
		for _, v := range values {
			if v != nil {
				out = append(out, v)
			}
		}
		return out, nil

	default:
		return nil, fmt.Errorf("unknown aggregate function %q", name)
	}
}

// isAggregateFunction reports whether name is a recognised aggregate function.
func isAggregateFunction(name string) bool {
	switch name {
	case "count", "sum", "avg", "min", "max", "collect":
		return true
	}
	return false
}

// sumValues adds all numeric values together. Non-numeric values are skipped.
func sumValues(values []interface{}) (interface{}, error) {
	var total float64
	for _, v := range values {
		n, err := toFloat(v)
		if err != nil {
			continue // skip non-numeric
		}
		total += n
	}
	// Return int64 when the result is a whole number for cleaner output.
	if total == math.Trunc(total) {
		return int64(total), nil
	}
	return total, nil
}

// avgValues computes the arithmetic mean of numeric values.
func avgValues(values []interface{}) (interface{}, error) {
	var total float64
	var count int
	for _, v := range values {
		n, err := toFloat(v)
		if err != nil {
			continue
		}
		total += n
		count++
	}
	if count == 0 {
		return nil, nil
	}
	return total / float64(count), nil
}

// minValue returns the smallest comparable value.
// Strings and numbers are supported; mixed types are compared by string representation.
func minValue(values []interface{}) (interface{}, error) {
	if len(values) == 0 {
		return nil, nil
	}
	comparables := toComparableSlice(values)
	sort.Slice(comparables, func(i, j int) bool {
		return compareLess(comparables[i], comparables[j])
	})
	return comparables[0], nil
}

// maxValue returns the largest comparable value.
func maxValue(values []interface{}) (interface{}, error) {
	if len(values) == 0 {
		return nil, nil
	}
	comparables := toComparableSlice(values)
	sort.Slice(comparables, func(i, j int) bool {
		return compareLess(comparables[i], comparables[j])
	})
	return comparables[len(comparables)-1], nil
}

// toFloat converts a value to float64. Returns an error if not numeric.
func toFloat(v interface{}) (float64, error) {
	switch n := v.(type) {
	case int:
		return float64(n), nil
	case int32:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case float32:
		return float64(n), nil
	case float64:
		return n, nil
	default:
		return 0, fmt.Errorf("not a number: %T", v)
	}
}

// toComparableSlice returns a copy of values with nil entries removed.
func toComparableSlice(values []interface{}) []interface{} {
	out := make([]interface{}, 0, len(values))
	for _, v := range values {
		if v != nil {
			out = append(out, v)
		}
	}
	return out
}

// compareLess returns true when a is ordered before b.
// Numbers are compared numerically; strings lexicographically; mixed → string comparison.
func compareLess(a, b interface{}) bool {
	af, aErr := toFloat(a)
	bf, bErr := toFloat(b)
	if aErr == nil && bErr == nil {
		return af < bf
	}
	return fmt.Sprintf("%v", a) < fmt.Sprintf("%v", b)
}
