// Package budget provides computation and recording of budget spend for Wyrd nodes.
package budget

import (
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// BudgetStatus represents the spending state of a budget envelope.
type BudgetStatus string

const (
	// BudgetOK indicates spend is below the warning threshold.
	BudgetOK BudgetStatus = "ok"

	// BudgetCaution indicates spend has reached or exceeded the warning threshold
	// but has not yet exhausted the allocation.
	BudgetCaution BudgetStatus = "caution"

	// BudgetOver indicates spend has reached or exceeded the full allocation.
	BudgetOver BudgetStatus = "over"
)

// BudgetSummary holds the computed state of a budget envelope for the current period.
type BudgetSummary struct {
	// Status is the derived spending status.
	Status BudgetStatus

	// Spent is the total amount spent in the current period.
	Spent float64

	// Allocated is the envelope's total allocation for the period.
	Allocated float64

	// Remaining is the allocation minus the amount spent (may be negative when over).
	Remaining float64
}

// Compute derives the current BudgetSummary from a budget node's properties.
//
// It reads the following fields from node.Properties:
//   - "spend_log"  — []SpendEntry or []interface{} (raw JSON)
//   - "allocated"  — float64 total for the period
//   - "warn_at"    — float64 fraction (0–1) at which to show Caution
//   - "period"     — string: "week", "month", "quarter", or "year"
//
// Entries outside the current period (relative to now) are excluded from the sum.
func Compute(node *types.Node, now time.Time) BudgetSummary {
	allocated := floatProperty(node, "allocated")
	warnAt := floatProperty(node, "warn_at")
	period := stringProperty(node, "period")

	entries := spendLog(node)
	spent := sumCurrentPeriod(entries, period, now)

	var status BudgetStatus
	switch {
	case allocated <= 0:
		// No allocation configured — treat as OK regardless of spend.
		status = BudgetOK
	case spent >= allocated:
		status = BudgetOver
	case warnAt > 0 && spent >= allocated*warnAt:
		status = BudgetCaution
	default:
		status = BudgetOK
	}

	return BudgetSummary{
		Status:    status,
		Spent:     spent,
		Allocated: allocated,
		Remaining: allocated - spent,
	}
}

// periodStart returns the start of the budget period that contains now.
func periodStart(period string, now time.Time) time.Time {
	y, m, d := now.Date()
	loc := now.Location()

	switch period {
	case "week":
		// Monday-anchored ISO week.
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7 // Sunday → 7
		}
		offset := weekday - 1
		return time.Date(y, m, d-offset, 0, 0, 0, 0, loc)

	case "month":
		return time.Date(y, m, 1, 0, 0, 0, 0, loc)

	case "quarter":
		// Quarters: Jan–Mar, Apr–Jun, Jul–Sep, Oct–Dec.
		quarterMonth := ((int(m) - 1) / 3) * 3
		return time.Date(y, time.Month(quarterMonth+1), 1, 0, 0, 0, 0, loc)

	case "year":
		return time.Date(y, time.January, 1, 0, 0, 0, 0, loc)

	default:
		// Unknown period — include everything by using the zero time.
		return time.Time{}
	}
}

// sumCurrentPeriod filters spend entries to the current period and returns the total.
func sumCurrentPeriod(entries []types.SpendEntry, period string, now time.Time) float64 {
	start := periodStart(period, now)
	var total float64
	for _, e := range entries {
		t, err := time.Parse("2006-01-02", e.Date)
		if err != nil {
			continue
		}
		if !t.Before(start) {
			total += e.Amount
		}
	}
	return total
}

// spendLog extracts the spend_log slice from node properties.
// It handles both []types.SpendEntry and the raw []interface{} form that
// arrives when properties are deserialised from JSON without type information.
func spendLog(node *types.Node) []types.SpendEntry {
	if node.Properties == nil {
		return nil
	}
	raw, ok := node.Properties["spend_log"]
	if !ok {
		return nil
	}

	// Already the correct concrete type.
	if entries, ok := raw.([]types.SpendEntry); ok {
		return entries
	}

	// Raw JSON slice — each element is a map[string]interface{}.
	rawSlice, ok := raw.([]interface{})
	if !ok {
		return nil
	}

	entries := make([]types.SpendEntry, 0, len(rawSlice))
	for _, item := range rawSlice {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		entry := types.SpendEntry{}
		if v, ok := m["date"].(string); ok {
			entry.Date = v
		}
		if v, ok := m["amount"].(float64); ok {
			entry.Amount = v
		}
		if v, ok := m["note"].(string); ok {
			entry.Note = v
		}
		entries = append(entries, entry)
	}
	return entries
}

// floatProperty reads a float64 property, returning 0 on absence or type mismatch.
func floatProperty(node *types.Node, key string) float64 {
	if node.Properties == nil {
		return 0
	}
	v, ok := node.Properties[key]
	if !ok {
		return 0
	}
	f, ok := v.(float64)
	if !ok {
		return 0
	}
	return f
}

// stringProperty reads a string property, returning "" on absence or type mismatch.
func stringProperty(node *types.Node, key string) string {
	if node.Properties == nil {
		return ""
	}
	v, ok := node.Properties[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}
