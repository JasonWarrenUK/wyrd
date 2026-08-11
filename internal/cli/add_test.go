package cli_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jasonwarrenuk/wyrd/internal/cli"
	"github.com/jasonwarrenuk/wyrd/internal/store"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

func TestAdd_ValidTask(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}

	id, err := cli.Add(s, cli.AddOptions{Body: "buy oat milk"})
	if err != nil {
		t.Fatalf("Add returned unexpected error: %v", err)
	}
	if id == "" {
		t.Fatal("Add returned empty ID")
	}

	node, err := s.ReadNode(id)
	if err != nil {
		t.Fatalf("ReadNode(%q) failed: %v", id, err)
	}
	if node.Body != "buy oat milk" {
		t.Errorf("node.Body = %q, want %q", node.Body, "buy oat milk")
	}
	if len(node.Types) != 1 || node.Types[0] != "task" {
		t.Errorf("node.Types = %v, want [task]", node.Types)
	}
	if node.Title != "" {
		t.Errorf("node.Title = %q, want empty", node.Title)
	}
}

func TestAdd_WithTitle(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}

	id, err := cli.Add(s, cli.AddOptions{Body: "buy oat milk", Title: "Groceries"})
	if err != nil {
		t.Fatalf("Add returned unexpected error: %v", err)
	}

	node, err := s.ReadNode(id)
	if err != nil {
		t.Fatalf("ReadNode(%q) failed: %v", id, err)
	}
	if node.Title != "Groceries" {
		t.Errorf("node.Title = %q, want %q", node.Title, "Groceries")
	}
}

func TestAdd_CustomType(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}

	id, err := cli.Add(s, cli.AddOptions{Body: "some idea", NodeType: "note"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	node, err := s.ReadNode(id)
	if err != nil {
		t.Fatalf("ReadNode failed: %v", err)
	}
	if len(node.Types) != 1 || node.Types[0] != "note" {
		t.Errorf("node.Types = %v, want [note]", node.Types)
	}
}

func TestAdd_EmptyBody(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}

	_, err = cli.Add(s, cli.AddOptions{Body: ""})
	if err == nil {
		t.Fatal("expected error for empty body, got nil")
	}
}

func TestAdd_WithLink(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}

	// Create a target node first.
	targetID, err := cli.Add(s, cli.AddOptions{Body: "target node"})
	if err != nil {
		t.Fatalf("creating target node: %v", err)
	}

	// Create a node linked to the target.
	sourceID, err := cli.Add(s, cli.AddOptions{Body: "linked node", LinkID: targetID})
	if err != nil {
		t.Fatalf("creating linked node: %v", err)
	}

	if sourceID == "" {
		t.Fatal("linked node ID is empty")
	}
}

func TestAdd_UsesInjectedClock(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer func() { _ = s.Close() }()

	fixed := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	id, err := cli.Add(s, cli.AddOptions{Body: "clocked node", Clock: types.StubClock{Fixed: fixed}})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	node, err := s.ReadNode(id)
	if err != nil {
		t.Fatalf("ReadNode: %v", err)
	}
	if !node.Date.Created.Equal(fixed) {
		t.Errorf("node.Date.Created = %v, want %v (from injected clock)", node.Date.Created, fixed)
	}
}

func TestAdd_LinkMalformedUUID(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer func() { _ = s.Close() }()

	_, err = cli.Add(s, cli.AddOptions{Body: "linked node", LinkID: "not-a-uuid", Index: s.Index()})
	if err == nil {
		t.Fatal("expected error for malformed link UUID, got nil")
	}
	var ve *types.ValidationError
	if !asValidationError(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestAdd_LinkNonexistentTarget(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer func() { _ = s.Close() }()

	missing := uuid.New().String()
	_, err = cli.Add(s, cli.AddOptions{Body: "linked node", LinkID: missing, Index: s.Index()})
	if err == nil {
		t.Fatal("expected error for nonexistent link target, got nil")
	}
	var ve *types.ValidationError
	if !asValidationError(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestAdd_LinkValidTargetCreatesEdge(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer func() { _ = s.Close() }()

	targetID, err := cli.Add(s, cli.AddOptions{Body: "target node"})
	if err != nil {
		t.Fatalf("creating target node: %v", err)
	}

	sourceID, err := cli.Add(s, cli.AddOptions{Body: "linked node", LinkID: targetID, Index: s.Index()})
	if err != nil {
		t.Fatalf("Add with valid link: %v", err)
	}

	edges := s.Index().EdgesFrom(sourceID)
	found := false
	for _, e := range edges {
		if e.To == targetID && e.Type == string(types.EdgeRelated) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected related edge from %s to %s", sourceID, targetID)
	}
}

func TestAdd_FailedLinkWritesNothing(t *testing.T) {
	storeDir := t.TempDir()
	s, err := store.New(storeDir, types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer func() { _ = s.Close() }()

	before := len(s.Index().AllNodes())

	missing := uuid.New().String()
	id, err := cli.Add(s, cli.AddOptions{Body: "should not be written", LinkID: missing, Index: s.Index()})
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
