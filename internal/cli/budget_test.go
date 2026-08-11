package cli_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jasonwarrenuk/wyrd/internal/cli"
	"github.com/jasonwarrenuk/wyrd/internal/store"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

func TestBudgetCreate_EmptyCategory(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer func() { _ = s.Close() }()

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
	defer func() { _ = s.Close() }()

	// Zero is a valid allocation (e.g. a category being wound down).
	id, err := cli.BudgetCreate(s, cli.BudgetCreateOptions{Category: "groceries", Allocated: 0})
	if err != nil {
		t.Fatalf("BudgetCreate with zero allocated: %v", err)
	}
	node, err := s.ReadNode(id)
	if err != nil {
		t.Fatalf("ReadNode: %v", err)
	}
	if alloc, ok := node.Properties["allocated"].(float64); !ok || alloc != 0 {
		t.Errorf("allocated = %v, want 0", node.Properties["allocated"])
	}
}

func TestBudgetCreate_NegativeAllocated(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer func() { _ = s.Close() }()

	_, err = cli.BudgetCreate(s, cli.BudgetCreateOptions{Category: "groceries", Allocated: -50})
	if err == nil {
		t.Fatal("expected validation error for negative allocated")
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
	defer func() { _ = s.Close() }()

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
	defer func() { _ = s.Close() }()

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
	if period, ok := node.Properties["period"].(string); !ok || period != "month" {
		t.Errorf("period = %v, want month", node.Properties["period"])
	}
	if warnAt, ok := node.Properties["warn_at"].(float64); !ok || warnAt != 1 {
		t.Errorf("warn_at = %v, want 1", node.Properties["warn_at"])
	}
}

func TestBudgetCreate_CustomPeriodAndWarnAt(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer func() { _ = s.Close() }()

	id, err := cli.BudgetCreate(s, cli.BudgetCreateOptions{
		Category:  "entertainment",
		Allocated: 150,
		Period:    "week",
		WarnAt:    0.5,
	})
	if err != nil {
		t.Fatalf("BudgetCreate returned unexpected error: %v", err)
	}

	node, err := s.ReadNode(id)
	if err != nil {
		t.Fatalf("ReadNode: %v", err)
	}

	if period, ok := node.Properties["period"].(string); !ok || period != "week" {
		t.Errorf("period = %v, want week", node.Properties["period"])
	}
	if warnAt, ok := node.Properties["warn_at"].(float64); !ok || warnAt != 0.5 {
		t.Errorf("warn_at = %v, want 0.5", node.Properties["warn_at"])
	}
}

func TestBudgetCreate_UsesInjectedClock(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer func() { _ = s.Close() }()

	fixed := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	id, err := cli.BudgetCreate(s, cli.BudgetCreateOptions{
		Category:  "groceries",
		Allocated: 100,
		Clock:     types.StubClock{Fixed: fixed},
	})
	if err != nil {
		t.Fatalf("BudgetCreate: %v", err)
	}

	node, err := s.ReadNode(id)
	if err != nil {
		t.Fatalf("ReadNode: %v", err)
	}
	if !node.Date.Created.Equal(fixed) {
		t.Errorf("node.Date.Created = %v, want %v (from injected clock)", node.Date.Created, fixed)
	}
}

func TestBudgetCreate_LinkMalformedUUID(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer func() { _ = s.Close() }()

	_, err = cli.BudgetCreate(s, cli.BudgetCreateOptions{
		Category:  "groceries",
		Allocated: 100,
		LinkID:    "not-a-uuid",
		Index:     s.Index(),
	})
	if err == nil {
		t.Fatal("expected error for malformed link UUID, got nil")
	}
	var ve *types.ValidationError
	if !asValidationError(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestBudgetCreate_LinkNonexistentTarget(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer func() { _ = s.Close() }()

	missing := uuid.New().String()
	_, err = cli.BudgetCreate(s, cli.BudgetCreateOptions{
		Category:  "groceries",
		Allocated: 100,
		LinkID:    missing,
		Index:     s.Index(),
	})
	if err == nil {
		t.Fatal("expected error for nonexistent link target, got nil")
	}
	var ve *types.ValidationError
	if !asValidationError(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestBudgetCreate_LinkValidTargetCreatesEdge(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer func() { _ = s.Close() }()

	targetID, err := cli.Add(s, cli.AddOptions{Body: "target node"})
	if err != nil {
		t.Fatalf("creating target node: %v", err)
	}

	budgetID, err := cli.BudgetCreate(s, cli.BudgetCreateOptions{
		Category:  "groceries",
		Allocated: 100,
		LinkID:    targetID,
		Index:     s.Index(),
	})
	if err != nil {
		t.Fatalf("BudgetCreate with valid link: %v", err)
	}

	edges := s.Index().EdgesFrom(budgetID)
	found := false
	for _, e := range edges {
		if e.To == targetID && e.Type == string(types.EdgeRelated) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected related edge from %s to %s", budgetID, targetID)
	}
}

func TestBudgetCreate_FailedLinkWritesNothing(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer func() { _ = s.Close() }()

	before := len(s.Index().AllNodes())

	missing := uuid.New().String()
	id, err := cli.BudgetCreate(s, cli.BudgetCreateOptions{
		Category:  "groceries",
		Allocated: 100,
		LinkID:    missing,
		Index:     s.Index(),
	})
	if err == nil {
		t.Fatal("expected error for nonexistent link target, got nil")
	}
	if id != "" {
		t.Errorf("expected empty node ID on failed link, got %q", id)
	}

	after := len(s.Index().AllNodes())
	if after != before {
		t.Errorf("node count changed from %d to %d; failed link should write nothing", before, after)
	}
}
