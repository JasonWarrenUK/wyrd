package cli_test

import (
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/cli"
	"github.com/jasonwarrenuk/wyrd/internal/store"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

func TestBudgetCreate_EmptyCategory(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer s.Close()

	_, err = cli.BudgetCreate(s, cli.BudgetCreateOptions{Category: "", Allocated: 100})
	if err == nil {
		t.Fatal("expected validation error for empty category")
	}
	var ve *types.ValidationError
	if !asValidationError(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestBudgetCreate_ZeroAllocated(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer s.Close()

	_, err = cli.BudgetCreate(s, cli.BudgetCreateOptions{Category: "groceries", Allocated: 0})
	if err == nil {
		t.Fatal("expected validation error for zero allocated")
	}
	var ve *types.ValidationError
	if !asValidationError(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestBudgetCreate_InvalidPeriod(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer s.Close()

	_, err = cli.BudgetCreate(s, cli.BudgetCreateOptions{
		Category:  "groceries",
		Allocated: 200,
		Period:    "biweekly",
	})
	if err == nil {
		t.Fatal("expected validation error for invalid period")
	}
	var ve *types.ValidationError
	if !asValidationError(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestBudgetCreate_Valid(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer s.Close()

	id, err := cli.BudgetCreate(s, cli.BudgetCreateOptions{
		Category:  "groceries",
		Allocated: 300,
	})
	if err != nil {
		t.Fatalf("BudgetCreate returned unexpected error: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty node ID")
	}

	// Read back the node and verify properties.
	node, err := s.ReadNode(id)
	if err != nil {
		t.Fatalf("ReadNode: %v", err)
	}

	if node.Title != "groceries" {
		t.Errorf("title = %q, want %q", node.Title, "groceries")
	}
	if len(node.Types) != 1 || node.Types[0] != "budget" {
		t.Errorf("types = %v, want [budget]", node.Types)
	}
	if cat, ok := node.Properties["category"].(string); !ok || cat != "groceries" {
		t.Errorf("category = %v, want groceries", node.Properties["category"])
	}
	if alloc, ok := node.Properties["allocated"].(float64); !ok || alloc != 300 {
		t.Errorf("allocated = %v, want 300", node.Properties["allocated"])
	}
	if period, ok := node.Properties["period"].(string); !ok || period != "monthly" {
		t.Errorf("period = %v, want monthly", node.Properties["period"])
	}
	if warnAt, ok := node.Properties["warn_at"].(float64); !ok || warnAt != 0.8 {
		t.Errorf("warn_at = %v, want 0.8", node.Properties["warn_at"])
	}
}

func TestBudgetCreate_CustomPeriodAndWarnAt(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer s.Close()

	id, err := cli.BudgetCreate(s, cli.BudgetCreateOptions{
		Category:  "entertainment",
		Allocated: 150,
		Period:    "weekly",
		WarnAt:    0.5,
	})
	if err != nil {
		t.Fatalf("BudgetCreate returned unexpected error: %v", err)
	}

	node, err := s.ReadNode(id)
	if err != nil {
		t.Fatalf("ReadNode: %v", err)
	}

	if period, ok := node.Properties["period"].(string); !ok || period != "weekly" {
		t.Errorf("period = %v, want weekly", node.Properties["period"])
	}
	if warnAt, ok := node.Properties["warn_at"].(float64); !ok || warnAt != 0.5 {
		t.Errorf("warn_at = %v, want 0.5", node.Properties["warn_at"])
	}
}
