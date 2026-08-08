package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jasonwarrenuk/wyrd/internal/cli"
	"github.com/jasonwarrenuk/wyrd/internal/store"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

func TestJournal_SavesNode(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer func() { _ = s.Close() }()

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
	defer func() { _ = s.Close() }()

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
	defer func() { _ = s.Close() }()

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

func TestJournal_UsesInjectedClock(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer func() { _ = s.Close() }()

	fixed := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	id, err := cli.Journal(s, cli.JournalOptions{Body: "clocked entry", Clock: types.StubClock{Fixed: fixed}})
	if err != nil {
		t.Fatalf("Journal: %v", err)
	}

	node, err := s.ReadNode(id)
	if err != nil {
		t.Fatalf("ReadNode: %v", err)
	}
	if !node.Date.Created.Equal(fixed) {
		t.Errorf("node.Date.Created = %v, want %v (from injected clock)", node.Date.Created, fixed)
	}
	wantTitle := fixed.Format("2006-01-02")
	if node.Title != wantTitle {
		t.Errorf("node.Title = %q, want %q (default title derived from injected clock)", node.Title, wantTitle)
	}
}

func TestJournal_LinkMalformedUUID(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer func() { _ = s.Close() }()

	_, err = cli.Journal(s, cli.JournalOptions{Body: "entry", LinkID: "not-a-uuid", Index: s.Index()})
	if err == nil {
		t.Fatal("expected error for malformed link UUID, got nil")
	}
	var ve *types.ValidationError
	if !asValidationError(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestJournal_LinkNonexistentTarget(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer func() { _ = s.Close() }()

	missing := uuid.New().String()
	_, err = cli.Journal(s, cli.JournalOptions{Body: "entry", LinkID: missing, Index: s.Index()})
	if err == nil {
		t.Fatal("expected error for nonexistent link target, got nil")
	}
	var ve *types.ValidationError
	if !asValidationError(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestJournal_LinkValidTargetCreatesEdge(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer func() { _ = s.Close() }()

	targetID, err := cli.Add(s, cli.AddOptions{Body: "target node"})
	if err != nil {
		t.Fatalf("creating target node: %v", err)
	}

	journalID, err := cli.Journal(s, cli.JournalOptions{Body: "entry", LinkID: targetID, Index: s.Index()})
	if err != nil {
		t.Fatalf("Journal with valid link: %v", err)
	}

	edges := s.Index().EdgesFrom(journalID)
	found := false
	for _, e := range edges {
		if e.To == targetID && e.Type == string(types.EdgeRelated) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected related edge from %s to %s", journalID, targetID)
	}
}

func TestJournal_FailedLinkWritesNothing(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer func() { _ = s.Close() }()

	before := len(s.Index().AllNodes())

	missing := uuid.New().String()
	id, err := cli.Journal(s, cli.JournalOptions{Body: "should not be written", LinkID: missing, Index: s.Index()})
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
	defer func() { _ = s.Close() }()

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

func TestNote_UsesInjectedClock(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer func() { _ = s.Close() }()

	fixed := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	id, err := cli.Note(s, cli.NoteOptions{Title: "Clocked note", Body: "content", Clock: types.StubClock{Fixed: fixed}})
	if err != nil {
		t.Fatalf("Note: %v", err)
	}

	node, err := s.ReadNode(id)
	if err != nil {
		t.Fatalf("ReadNode: %v", err)
	}
	if !node.Date.Created.Equal(fixed) {
		t.Errorf("node.Date.Created = %v, want %v (from injected clock)", node.Date.Created, fixed)
	}
}

func TestNote_LinkMalformedUUID(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer func() { _ = s.Close() }()

	_, err = cli.Note(s, cli.NoteOptions{Title: "Note", Body: "content", LinkID: "not-a-uuid", Index: s.Index()})
	if err == nil {
		t.Fatal("expected error for malformed link UUID, got nil")
	}
	var ve *types.ValidationError
	if !asValidationError(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestNote_LinkNonexistentTarget(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer func() { _ = s.Close() }()

	missing := uuid.New().String()
	_, err = cli.Note(s, cli.NoteOptions{Title: "Note", Body: "content", LinkID: missing, Index: s.Index()})
	if err == nil {
		t.Fatal("expected error for nonexistent link target, got nil")
	}
	var ve *types.ValidationError
	if !asValidationError(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestNote_LinkValidTargetCreatesEdge(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer func() { _ = s.Close() }()

	targetID, err := cli.Add(s, cli.AddOptions{Body: "target node"})
	if err != nil {
		t.Fatalf("creating target node: %v", err)
	}

	noteID, err := cli.Note(s, cli.NoteOptions{Title: "Note", Body: "content", LinkID: targetID, Index: s.Index()})
	if err != nil {
		t.Fatalf("Note with valid link: %v", err)
	}

	edges := s.Index().EdgesFrom(noteID)
	found := false
	for _, e := range edges {
		if e.To == targetID && e.Type == string(types.EdgeRelated) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected related edge from %s to %s", noteID, targetID)
	}
}

func TestNote_FailedLinkWritesNothing(t *testing.T) {
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer func() { _ = s.Close() }()

	before := len(s.Index().AllNodes())

	missing := uuid.New().String()
	id, err := cli.Note(s, cli.NoteOptions{Title: "Note", Body: "should not be written", LinkID: missing, Index: s.Index()})
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
