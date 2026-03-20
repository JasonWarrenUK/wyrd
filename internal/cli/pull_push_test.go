package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/cli"
	"github.com/jasonwarrenuk/wyrd/internal/store"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// ---------------------------------------------------------------------------
// Push tests
// ---------------------------------------------------------------------------

func TestPush_EmptyNodeID(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer s.Close()

	var buf bytes.Buffer
	err = cli.Push(s, cli.PushOptions{NodeID: ""}, &buf)
	if err == nil {
		t.Fatal("expected validation error for empty node ID")
	}
	var ve *types.ValidationError
	if !asValidationError(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestPush_NodeNotFound(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer s.Close()

	var buf bytes.Buffer
	err = cli.Push(s, cli.PushOptions{NodeID: "00000000-0000-0000-0000-000000000099"}, &buf)
	if err == nil {
		t.Fatal("expected error for non-existent node")
	}
}

func TestPush_ExistingNode(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer s.Close()

	node, err := s.CreateNode("Push me to Obsidian", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	var buf bytes.Buffer
	if err := cli.Push(s, cli.PushOptions{NodeID: node.ID}, &buf); err != nil {
		t.Fatalf("Push: %v", err)
	}
	// The stub output must mention the node ID.
	if !strings.Contains(buf.String(), node.ID) {
		t.Errorf("expected node ID %q in output, got: %q", node.ID, buf.String())
	}
}

// ---------------------------------------------------------------------------
// PullObsidian tests
// ---------------------------------------------------------------------------

func TestPullObsidian_EmptyVault(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer s.Close()

	var buf bytes.Buffer
	err = cli.PullObsidian(s, cli.PullObsidianOptions{VaultPath: ""}, &buf)
	if err == nil {
		t.Fatal("expected validation error for empty vault path")
	}
	var ve *types.ValidationError
	if !asValidationError(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestPullObsidian_DryRun(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer s.Close()

	var buf bytes.Buffer
	err = cli.PullObsidian(s, cli.PullObsidianOptions{
		VaultPath: "/tmp/my-vault",
		DryRun:    true,
	}, &buf)
	if err != nil {
		t.Fatalf("PullObsidian dry-run: %v", err)
	}
	if !strings.Contains(buf.String(), "dry-run") {
		t.Errorf("expected 'dry-run' in output, got: %q", buf.String())
	}
}
