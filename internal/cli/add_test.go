package cli_test

import (
	"testing"

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
