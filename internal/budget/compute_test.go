package budget

import (
	"testing"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// budgetNode constructs a minimal budget node with the given properties.
func budgetNode(allocated, warnAt float64, period string, entries []types.SpendEntry) *types.Node {
	return &types.Node{
		ID:    "test-node-1",
		Body:  "Test budget",
		Types: []string{"budget"},
		Properties: map[string]interface{}{
			"allocated": allocated,
			"warn_at":   warnAt,
			"period":    period,
			"spend_log": entries,
		},
	}
}

// fixedNow is a stable reference time in the middle of a month.
var fixedNow = time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)

func TestCompute_OK(t *testing.T) {
	entries := []types.SpendEntry{
		{Date: "2026-03-10", Amount: 20},
		{Date: "2026-03-12", Amount: 10},
	}
	node := budgetNode(100, 0.8, "month", entries)
	summary := Compute(node, fixedNow)

	if summary.Status != BudgetOK {
		t.Errorf("expected OK, got %s", summary.Status)
	}
	if summary.Spent != 30 {
		t.Errorf("expected spent 30, got %g", summary.Spent)
	}
	if summary.Allocated != 100 {
		t.Errorf("expected allocated 100, got %g", summary.Allocated)
	}
	if summary.Remaining != 70 {
		t.Errorf("expected remaining 70, got %g", summary.Remaining)
	}
}

func TestCompute_Caution(t *testing.T) {
	// 80% of 100 = 80; spend 85 should trigger Caution.
	entries := []types.SpendEntry{
		{Date: "2026-03-01", Amount: 85},
	}
	node := budgetNode(100, 0.8, "month", entries)
	summary := Compute(node, fixedNow)

	if summary.Status != BudgetCaution {
		t.Errorf("expected Caution, got %s", summary.Status)
	}
	if summary.Spent != 85 {
		t.Errorf("expected spent 85, got %g", summary.Spent)
	}
	if summary.Remaining != 15 {
		t.Errorf("expected remaining 15, got %g", summary.Remaining)
	}
}

func TestCompute_Over(t *testing.T) {
	entries := []types.SpendEntry{
		{Date: "2026-03-05", Amount: 60},
		{Date: "2026-03-10", Amount: 50},
	}
	node := budgetNode(100, 0.8, "month", entries)
	summary := Compute(node, fixedNow)

	if summary.Status != BudgetOver {
		t.Errorf("expected Over, got %s", summary.Status)
	}
	if summary.Spent != 110 {
		t.Errorf("expected spent 110, got %g", summary.Spent)
	}
	if summary.Remaining != -10 {
		t.Errorf("expected remaining -10, got %g", summary.Remaining)
	}
}

func TestCompute_PeriodFiltering_Month(t *testing.T) {
	// One entry in the current month, one from last month — only the current one counts.
	entries := []types.SpendEntry{
		{Date: "2026-02-28", Amount: 999}, // previous month
		{Date: "2026-03-01", Amount: 25},  // current month
	}
	node := budgetNode(100, 0.8, "month", entries)
	summary := Compute(node, fixedNow)

	if summary.Spent != 25 {
		t.Errorf("expected spent 25 (only current month), got %g", summary.Spent)
	}
}

func TestCompute_PeriodFiltering_Week(t *testing.T) {
	// fixedNow = Sunday 2026-03-15. ISO week starts Monday 2026-03-09.
	entries := []types.SpendEntry{
		{Date: "2026-03-08", Amount: 999}, // previous week (Sunday before week start)
		{Date: "2026-03-09", Amount: 40},  // Monday — start of week
		{Date: "2026-03-14", Amount: 20},  // Saturday — still in week
	}
	node := budgetNode(100, 0.8, "week", entries)
	summary := Compute(node, fixedNow)

	if summary.Spent != 60 {
		t.Errorf("expected spent 60 (current week only), got %g", summary.Spent)
	}
}

func TestCompute_PeriodFiltering_Quarter(t *testing.T) {
	// fixedNow is 2026-03-15 — Q1 starts 2026-01-01.
	entries := []types.SpendEntry{
		{Date: "2025-12-31", Amount: 999}, // Q4 previous year
		{Date: "2026-01-01", Amount: 50},  // first day of Q1
		{Date: "2026-03-15", Amount: 30},  // mid-Q1
	}
	node := budgetNode(200, 0.8, "quarter", entries)
	summary := Compute(node, fixedNow)

	if summary.Spent != 80 {
		t.Errorf("expected spent 80 (current quarter), got %g", summary.Spent)
	}
}

func TestCompute_PeriodFiltering_Year(t *testing.T) {
	entries := []types.SpendEntry{
		{Date: "2025-12-31", Amount: 999}, // previous year
		{Date: "2026-01-01", Amount: 100}, // current year
	}
	node := budgetNode(500, 0.8, "year", entries)
	summary := Compute(node, fixedNow)

	if summary.Spent != 100 {
		t.Errorf("expected spent 100 (current year only), got %g", summary.Spent)
	}
}

func TestCompute_EmptyLog(t *testing.T) {
	node := budgetNode(100, 0.8, "month", nil)
	summary := Compute(node, fixedNow)

	if summary.Status != BudgetOK {
		t.Errorf("expected OK for empty log, got %s", summary.Status)
	}
	if summary.Spent != 0 {
		t.Errorf("expected spent 0, got %g", summary.Spent)
	}
}

func TestCompute_ExactWarnThreshold(t *testing.T) {
	// Spend exactly equals allocated * warn_at — should be Caution.
	entries := []types.SpendEntry{
		{Date: "2026-03-10", Amount: 80},
	}
	node := budgetNode(100, 0.8, "month", entries)
	summary := Compute(node, fixedNow)

	if summary.Status != BudgetCaution {
		t.Errorf("expected Caution at exact threshold, got %s", summary.Status)
	}
}

func TestCompute_ExactAllocation(t *testing.T) {
	// Spend exactly equals allocated — should be Over.
	entries := []types.SpendEntry{
		{Date: "2026-03-10", Amount: 100},
	}
	node := budgetNode(100, 0.8, "month", entries)
	summary := Compute(node, fixedNow)

	if summary.Status != BudgetOver {
		t.Errorf("expected Over at exact allocation, got %s", summary.Status)
	}
}

func TestCompute_NilProperties(t *testing.T) {
	node := &types.Node{ID: "x", Body: "budget", Types: []string{"budget"}}
	summary := Compute(node, fixedNow)

	if summary.Status != BudgetOK {
		t.Errorf("expected OK for nil properties, got %s", summary.Status)
	}
	if summary.Allocated != 0 {
		t.Errorf("expected zero allocation, got %g", summary.Allocated)
	}
}
