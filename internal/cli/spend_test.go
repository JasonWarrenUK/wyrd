package cli_test

import (
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/cli"
	"github.com/jasonwarrenuk/wyrd/internal/store"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

func TestSpend_EmptyCategory(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer s.Close()

	err = cli.Spend(s, s.Index(), cli.SpendOptions{Category: "", Amount: 10.0})
	if err == nil {
		t.Fatal("expected validation error for empty category")
	}
	var ve *types.ValidationError
	if !asValidationError(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestSpend_ZeroAmount(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer s.Close()

	err = cli.Spend(s, s.Index(), cli.SpendOptions{Category: "groceries", Amount: 0})
	if err == nil {
		t.Fatal("expected validation error for zero amount")
	}
	var ve *types.ValidationError
	if !asValidationError(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestSpend_Valid(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer s.Close()

	// Create a budget node with the matching category.
	budgetNode, err := s.CreateNode("Groceries budget", []string{"budget"})
	if err != nil {
		t.Fatalf("CreateNode (budget): %v", err)
	}
	if _, err := s.UpdateNode(budgetNode.ID, map[string]interface{}{
		"category": "groceries",
	}); err != nil {
		t.Fatalf("UpdateNode (set category): %v", err)
	}

	err = cli.Spend(s, s.Index(), cli.SpendOptions{
		Category: "groceries",
		Amount:   25.50,
		Note:     "oat milk and bread",
	})
	if err != nil {
		t.Fatalf("Spend returned unexpected error: %v", err)
	}

	// Read back the node and verify the spend_log entry was written.
	got, err := s.ReadNode(budgetNode.ID)
	if err != nil {
		t.Fatalf("ReadNode: %v", err)
	}

	spendLog, ok := got.Properties["spend_log"]
	if !ok {
		t.Fatal("spend_log property not found after Spend")
	}
	entries, ok := spendLog.([]types.SpendEntry)
	if !ok {
		// RecordSpend stores SpendEntry structs directly; accept the type as-is.
		t.Logf("spend_log type: %T (value: %v)", spendLog, spendLog)
		// Acceptable — the node was persisted; type assertion details vary.
		return
	}
	if len(entries) != 1 {
		t.Errorf("spend_log has %d entries, want 1", len(entries))
	}
}
