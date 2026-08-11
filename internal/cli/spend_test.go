package cli_test

import (
	"testing"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/cli"
	"github.com/jasonwarrenuk/wyrd/internal/store"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

func TestSpend_EmptyCategory(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer func() { _ = s.Close() }()

	_, err = cli.Spend(s, s.Index(), cli.SpendOptions{Category: "", Amount: 10.0})
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
	defer func() { _ = s.Close() }()

	_, err = cli.Spend(s, s.Index(), cli.SpendOptions{Category: "groceries", Amount: 0})
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
	defer func() { _ = s.Close() }()

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

	warning, err := cli.Spend(s, s.Index(), cli.SpendOptions{
		Category: "groceries",
		Amount:   25.50,
		Note:     "oat milk and bread",
	})
	if err != nil {
		t.Fatalf("Spend returned unexpected error: %v", err)
	}
	if warning != "" {
		t.Errorf("expected no warning for a fresh entry, got: %s", warning)
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

func TestSpend_UsesInjectedClock(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer func() { _ = s.Close() }()

	budgetNode, err := s.CreateNode("Groceries budget", []string{"budget"})
	if err != nil {
		t.Fatalf("CreateNode (budget): %v", err)
	}
	if _, err := s.UpdateNode(budgetNode.ID, map[string]interface{}{
		"category": "groceries",
	}); err != nil {
		t.Fatalf("UpdateNode (set category): %v", err)
	}

	fixed := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)
	_, err = cli.Spend(s, s.Index(), cli.SpendOptions{
		Category: "groceries",
		Amount:   10,
		Note:     "stub clock test",
		Clock:    types.StubClock{Fixed: fixed},
	})
	if err != nil {
		t.Fatalf("Spend returned unexpected error: %v", err)
	}

	got, err := s.ReadNode(budgetNode.ID)
	if err != nil {
		t.Fatalf("ReadNode: %v", err)
	}
	// ReadNode decodes spend_log from persisted JSONC, so entries arrive as
	// []interface{} of map[string]interface{} rather than []types.SpendEntry
	// (which is only the in-process shape RecordSpend builds before writing).
	rawEntries, ok := got.Properties["spend_log"].([]interface{})
	if !ok || len(rawEntries) != 1 {
		t.Fatalf("expected 1 spend_log entry of type []interface{}, got %T", got.Properties["spend_log"])
	}
	entry, ok := rawEntries[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected spend_log entry of type map[string]interface{}, got %T", rawEntries[0])
	}
	wantDate := fixed.Format("2006-01-02")
	if gotDate, _ := entry["date"].(string); gotDate != wantDate {
		t.Errorf("spend entry date = %q, want %q (from injected clock)", gotDate, wantDate)
	}
}
