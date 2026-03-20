package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/cli"
	"github.com/jasonwarrenuk/wyrd/internal/store"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// mockEditorScript writes known content to the file path provided as $1.
func mockEditorScript(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	script := filepath.Join(dir, "editor.sh")

	// The script writes our known content to the file it receives as its first argument.
	body := "#!/bin/sh\nprintf '%s' '" + content + "' > \"$1\"\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("writing mock editor script: %v", err)
	}
	return script
}

func TestNote_ValidTitle(t *testing.T) {
	editor := mockEditorScript(t, "# My Note\n\nSome content here.")
	t.Setenv("EDITOR", editor)

	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	id, err := cli.Note(s, cli.NoteOptions{Title: "My Note"})
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
}

func TestNote_EmptyTitle(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}

	_, err = cli.Note(s, cli.NoteOptions{Title: ""})
	if err == nil {
		t.Fatal("expected error for empty title, got nil")
	}
}

func TestNote_EditorWritesNoContent(t *testing.T) {
	// Script writes only the template heading back — no additional content.
	// The Note function should discard the entry.
	script := filepath.Join(t.TempDir(), "editor.sh")
	body := "#!/bin/sh\n# do nothing — file already has template content\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("writing noop editor script: %v", err)
	}
	t.Setenv("EDITOR", script)

	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	_, err = cli.Note(s, cli.NoteOptions{Title: "Empty"})
	if err == nil {
		t.Fatal("expected error when no content written, got nil")
	}
}

func TestJournal_SavesNode(t *testing.T) {
	// Use a fixed date string to match what Journal writes as the heading.
	// The script writes the heading plus extra content so the entry is not discarded.
	now := "2026-03-19"
	content := "# " + now + "\n\nToday was productive."
	editor := mockEditorScript(t, content)
	t.Setenv("EDITOR", editor)

	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer s.Close()

	id, err := cli.Journal(s, cli.JournalOptions{})
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
}

func TestJournal_WithLink(t *testing.T) {
	now := "2026-03-19"
	content := "# " + now + "\n\nLinked entry."
	editor := mockEditorScript(t, content)
	t.Setenv("EDITOR", editor)

	storeDir := t.TempDir()
	s, err := store.New(storeDir, types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer s.Close()

	// Create a target node to link from the journal entry.
	targetID, err := cli.Add(s, cli.AddOptions{Body: "linked target"})
	if err != nil {
		t.Fatalf("Add (target): %v", err)
	}

	journalID, err := cli.Journal(s, cli.JournalOptions{LinkID: targetID})
	if err != nil {
		t.Fatalf("Journal with link: %v", err)
	}

	// Journal uses WriteEdge directly (not CreateEdge), so the index may not
	// reflect it immediately. Read the edges directory from disk instead.
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
		// Use the store to read each edge by ID (strip .jsonc extension).
		name := entry.Name()
		if len(name) < 6 {
			continue
		}
		edgeID := name[:len(name)-6] // strip ".jsonc"
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
