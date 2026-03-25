package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/cli"
	"github.com/jasonwarrenuk/wyrd/internal/store"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

func TestJournal_SavesNode(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer s.Close()

	id, err := cli.Journal(s, cli.JournalOptions{Body: "Today was productive."})
	if err != nil {
		t.Fatalf("Journal: %v", err)
	}
	if id == "" {
		t.Fatal("Journal returned empty ID")
	}

	node, err := s.ReadNode(id)
	if err != nil {
		t.Fatalf("ReadNode: %v", err)
	}
	if len(node.Types) != 1 || node.Types[0] != "journal" {
		t.Errorf("node.Types = %v, want [journal]", node.Types)
	}
	if node.Body != "Today was productive." {
		t.Errorf("node.Body = %q, want %q", node.Body, "Today was productive.")
	}
	// Title should default to today's date.
	wantTitle := time.Now().Format("2006-01-02")
	if node.Title != wantTitle {
		t.Errorf("node.Title = %q, want %q", node.Title, wantTitle)
	}
}

func TestJournal_EmptyBody(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}

	_, err = cli.Journal(s, cli.JournalOptions{Body: ""})
	if err == nil {
		t.Fatal("expected error for empty body, got nil")
	}
}

func TestJournal_WithTitle(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer s.Close()

	id, err := cli.Journal(s, cli.JournalOptions{
		Title: "A Custom Title",
		Body:  "Some content.",
	})
	if err != nil {
		t.Fatalf("Journal: %v", err)
	}

	node, err := s.ReadNode(id)
	if err != nil {
		t.Fatalf("ReadNode: %v", err)
	}
	if node.Title != "A Custom Title" {
		t.Errorf("node.Title = %q, want %q", node.Title, "A Custom Title")
	}
}

func TestJournal_WithLink(t *testing.T) {
	storeDir := t.TempDir()
	s, err := store.New(storeDir, types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer s.Close()

	targetID, err := cli.Add(s, cli.AddOptions{Body: "linked target"})
	if err != nil {
		t.Fatalf("Add (target): %v", err)
	}

	journalID, err := cli.Journal(s, cli.JournalOptions{
		Body:   "Linked entry.",
		LinkID: targetID,
	})
	if err != nil {
		t.Fatalf("Journal with link: %v", err)
	}

	edgesDir := filepath.Join(storeDir, "edges")
	entries, err := os.ReadDir(edgesDir)
	if err != nil {
		t.Fatalf("reading edges dir: %v", err)
	}

	found := false
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".jsonc") {
			continue
		}
		edgeID := name[:len(name)-6]
		edge, err := s.ReadEdge(edgeID)
		if err != nil {
			continue
		}
		if edge.From == journalID && edge.To == targetID && edge.Type == string(types.EdgeRelated) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected related edge from journal node %s to target %s", journalID, targetID)
	}
}

func TestNote_ValidTitle(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}

	id, err := cli.Note(s, cli.NoteOptions{Title: "My Note", Body: "Some content here."})
	if err != nil {
		t.Fatalf("Note returned unexpected error: %v", err)
	}
	if id == "" {
		t.Fatal("Note returned empty ID")
	}

	node, err := s.ReadNode(id)
	if err != nil {
		t.Fatalf("ReadNode(%q) failed: %v", id, err)
	}
	if len(node.Types) != 1 || node.Types[0] != "note" {
		t.Errorf("node.Types = %v, want [note]", node.Types)
	}
	if node.Title != "My Note" {
		t.Errorf("node.Title = %q, want %q", node.Title, "My Note")
	}
}

func TestNote_EmptyTitle(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}

	_, err = cli.Note(s, cli.NoteOptions{Title: "", Body: "Some content."})
	if err == nil {
		t.Fatal("expected error for empty title, got nil")
	}
}

func TestNote_EmptyBody(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}

	_, err = cli.Note(s, cli.NoteOptions{Title: "My Note", Body: ""})
	if err == nil {
		t.Fatal("expected error for empty body, got nil")
	}
}

func TestNote_WithLink(t *testing.T) {
	storeDir := t.TempDir()
	s, err := store.New(storeDir, types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer s.Close()

	targetID, err := cli.Add(s, cli.AddOptions{Body: "linked target"})
	if err != nil {
		t.Fatalf("Add (target): %v", err)
	}

	noteID, err := cli.Note(s, cli.NoteOptions{
		Title:  "Linked Note",
		Body:   "Note content.",
		LinkID: targetID,
	})
	if err != nil {
		t.Fatalf("Note with link: %v", err)
	}

	edgesDir := filepath.Join(storeDir, "edges")
	entries, err := os.ReadDir(edgesDir)
	if err != nil {
		t.Fatalf("reading edges dir: %v", err)
	}

	found := false
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".jsonc") {
			continue
		}
		edgeID := name[:len(name)-6]
		edge, err := s.ReadEdge(edgeID)
		if err != nil {
			continue
		}
		if edge.From == noteID && edge.To == targetID && edge.Type == string(types.EdgeRelated) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected related edge from note %s to target %s", noteID, targetID)
	}
}
