package views

import (
	"strings"
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// makebudgetNode is a helper that builds a minimal budget node from properties.
func makeBudgetNode(name string, allocated, warnAt float64, spendEntries []types.SpendEntry) *types.Node {
	return &types.Node{
		ID:    name,
		Body:  name,
		Types: []string{"budget"},
		Properties: map[string]interface{}{
			"allocated": allocated,
			"warn_at":   warnAt,
			"spend_log": spendEntries,
		},
	}
}

func TestBudgetRenderer_Empty(t *testing.T) {
	r := NewBudgetRenderer()
	output := r.Render(nil, 80)
	if !strings.Contains(output, "No budget envelopes") {
		t.Errorf("expected empty-state message, got: %q", output)
	}
}

func TestBudgetRenderer_OKStatus(t *testing.T) {
	r := NewBudgetRenderer()
	node := makeBudgetNode("Groceries", 200.0, 0.8, []types.SpendEntry{
		{Date: "2024-03-01", Amount: 30.0, Note: "Supermarket"},
		{Date: "2024-03-07", Amount: 25.0, Note: "Market"},
	})

	output := r.Render([]*types.Node{node}, 80)

	if !strings.Contains(output, r.Glyphs.OK) {
		t.Errorf("expected OK glyph %q for low-spend envelope, got:\n%s", r.Glyphs.OK, output)
	}
	if !strings.Contains(output, "Groceries") {
		t.Error("expected envelope name in output")
	}
	if !strings.Contains(output, "£55.00") {
		t.Errorf("expected spend total £55.00 in output, got:\n%s", output)
	}
}

func TestBudgetRenderer_CautionStatus(t *testing.T) {
	r := NewBudgetRenderer()
	// 85% spent of £100 with warn_at 0.8 → Caution.
	node := makeBudgetNode("Transport", 100.0, 0.8, []types.SpendEntry{
		{Date: "2024-03-01", Amount: 85.0, Note: "Train pass"},
	})

	output := r.Render([]*types.Node{node}, 80)

	if !strings.Contains(output, r.Glyphs.Caution) {
		t.Errorf("expected Caution glyph %q, got:\n%s", r.Glyphs.Caution, output)
	}
}

func TestBudgetRenderer_OverStatus(t *testing.T) {
	r := NewBudgetRenderer()
	// 120% spent of £50 → Over.
	node := makeBudgetNode("Eating out", 50.0, 0.8, []types.SpendEntry{
		{Date: "2024-03-01", Amount: 60.0, Note: "Dinner"},
	})

	output := r.Render([]*types.Node{node}, 80)

	if !strings.Contains(output, r.Glyphs.Over) {
		t.Errorf("expected Over glyph %q, got:\n%s", r.Glyphs.Over, output)
	}
}

func TestBudgetRenderer_AllThreeStatuses(t *testing.T) {
	r := NewBudgetRenderer()
	nodes := []*types.Node{
		makeBudgetNode("OK envelope", 100.0, 0.8, []types.SpendEntry{
			{Amount: 20.0},
		}),
		makeBudgetNode("Caution envelope", 100.0, 0.8, []types.SpendEntry{
			{Amount: 85.0},
		}),
		makeBudgetNode("Over envelope", 100.0, 0.8, []types.SpendEntry{
			{Amount: 110.0},
		}),
	}

	output := r.Render(nodes, 100)

	if !strings.Contains(output, r.Glyphs.OK) {
		t.Errorf("expected OK glyph in multi-envelope output")
	}
	if !strings.Contains(output, r.Glyphs.Caution) {
		t.Errorf("expected Caution glyph in multi-envelope output")
	}
	if !strings.Contains(output, r.Glyphs.Over) {
		t.Errorf("expected Over glyph in multi-envelope output")
	}
}

func TestBudgetRenderer_ProgressBarFill(t *testing.T) {
	r := NewBudgetRenderer()
	// 50% spent should produce 10 filled segments out of 20.
	node := makeBudgetNode("Savings", 100.0, 0.8, []types.SpendEntry{
		{Amount: 50.0},
	})

	output := r.Render([]*types.Node{node}, 80)

	// The bar should contain both filled and empty characters.
	if !strings.Contains(output, budgetFilled) {
		t.Error("expected filled bar segment in output")
	}
	if !strings.Contains(output, budgetEmpty) {
		t.Error("expected empty bar segment in output")
	}
}

func TestBudgetRenderer_MapSpendLog(t *testing.T) {
	// Verify that spend_log stored as []interface{} of map[string]interface{}
	// (as produced by JSON unmarshal) is summed correctly.
	r := NewBudgetRenderer()
	node := &types.Node{
		ID:    "map-test",
		Body:  "Map test",
		Types: []string{"budget"},
		Properties: map[string]interface{}{
			"allocated": float64(100),
			"warn_at":   float64(0.8),
			"spend_log": []interface{}{
				map[string]interface{}{"date": "2024-01-01", "amount": float64(30)},
				map[string]interface{}{"date": "2024-01-02", "amount": float64(20)},
			},
		},
	}

	output := r.Render([]*types.Node{node}, 80)
	if !strings.Contains(output, "£50.00") {
		t.Errorf("expected £50.00 from map-based spend_log, got:\n%s", output)
	}
}

func TestComputeBudgetStatus(t *testing.T) {
	tests := []struct {
		name      string
		spend     float64
		allocated float64
		warnAt    float64
		want      BudgetStatus
	}{
		{"zero allocated", 0, 0, 0.8, BudgetOK},
		{"well under warn", 10, 100, 0.8, BudgetOK},
		{"exactly at warn", 80, 100, 0.8, BudgetCaution},
		{"above warn, below allocated", 90, 100, 0.8, BudgetCaution},
		{"exactly allocated", 100, 100, 0.8, BudgetOver},
		{"above allocated", 120, 100, 0.8, BudgetOver},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeBudgetStatus(tt.spend, tt.allocated, tt.warnAt)
			if got != tt.want {
				t.Errorf("computeBudgetStatus(%v, %v, %v) = %v, want %v",
					tt.spend, tt.allocated, tt.warnAt, got, tt.want)
			}
		})
	}
}
