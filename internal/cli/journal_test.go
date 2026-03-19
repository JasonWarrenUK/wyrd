package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/cli"
	"github.com/jasonwarrenuk/wyrd/internal/store"
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

	s := store.New(t.TempDir())
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
	s := store.New(t.TempDir())

	_, err := cli.Note(s, cli.NoteOptions{Title: ""})
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

	s := store.New(t.TempDir())
	_, err := cli.Note(s, cli.NoteOptions{Title: "Empty"})
	if err == nil {
		t.Fatal("expected error when no content written, got nil")
	}
}
