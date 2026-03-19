package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// fixedClock returns a deterministic clock for testing.
type fixedClock struct{ t time.Time }

func (c *fixedClock) Now() time.Time { return c.t }

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	clock := &fixedClock{t: time.Date(2026, 3, 17, 10, 30, 0, 0, time.UTC)}

	// Copy test templates into the store.
	tmplSrc := filepath.Join("testdata", "templates", "task.jsonc")
	tmplDst := filepath.Join(dir, "templates", "task.jsonc")

	s, err := New(dir, clock)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Copy task template.
	data, err := readJSONC(tmplSrc)
	if err == nil {
		_ = writeJSONC(tmplDst, mustUnmarshal(data))
		// Invalidate cache so it reloads.
		s.templateMu.Lock()
		delete(s.templates, "task")
		s.templateMu.Unlock()
	}

	return s
}

func mustUnmarshal(data []byte) map[string]interface{} {
	var m map[string]interface{}
	_ = unmarshalJSON(data, &m)
	return m
}

func TestCreateAndReadNode(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	node, err := s.CreateNode("My first task", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	if node.ID == "" {
		t.Error("expected non-empty ID")
	}
	if node.Body != "My first task" {
		t.Errorf("body = %q, want %q", node.Body, "My first task")
	}
	if len(node.Types) != 1 || node.Types[0] != "task" {
		t.Errorf("types = %v, want [task]", node.Types)
	}

	// Read back from disk.
	got, err := s.ReadNode(node.ID)
	if err != nil {
		t.Fatalf("ReadNode: %v", err)
	}
	if got.ID != node.ID {
		t.Errorf("ID = %q, want %q", got.ID, node.ID)
	}
}

func TestUpdateNode(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	node, err := s.CreateNode("Update me", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	updated, err := s.UpdateNode(node.ID, map[string]interface{}{"status": "done"})
	if err != nil {
		t.Fatalf("UpdateNode: %v", err)
	}
	if updated.Properties["status"] != "done" {
		t.Errorf("status = %v, want done", updated.Properties["status"])
	}
}

func TestArchiveNode(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	node, err := s.CreateNode("Archive me", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	if err := s.ArchiveNode(node.ID); err != nil {
		t.Fatalf("ArchiveNode: %v", err)
	}

	got, err := s.ReadNode(node.ID)
	if err != nil {
		t.Fatalf("ReadNode: %v", err)
	}
	if got.Properties["status"] != "archived" {
		t.Errorf("status = %v, want archived", got.Properties["status"])
	}
}

func TestReadNodeNotFound(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	_, err := s.ReadNode("00000000-0000-0000-0000-000000000099")
	if err == nil {
		t.Fatal("expected error for missing node")
	}
	var nfe *types.NotFoundError
	if !errorAs(err, &nfe) {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestCreateAndDeleteEdge(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	n1, _ := s.CreateNode("Node A", []string{"task"})
	n2, _ := s.CreateNode("Node B", []string{"task"})

	edge, err := s.CreateEdge("blocks", n1.ID, n2.ID, nil)
	if err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}

	got, err := s.ReadEdge(edge.ID)
	if err != nil {
		t.Fatalf("ReadEdge: %v", err)
	}
	if got.Type != "blocks" {
		t.Errorf("type = %q, want blocks", got.Type)
	}
	if got.From != n1.ID || got.To != n2.ID {
		t.Errorf("from/to mismatch")
	}

	if err := s.DeleteEdge(edge.ID); err != nil {
		t.Fatalf("DeleteEdge: %v", err)
	}

	_, err = s.ReadEdge(edge.ID)
	if err == nil {
		t.Error("expected error after deletion")
	}
}

func TestIndexLookup(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	n1, _ := s.CreateNode("A", []string{"task"})
	n2, _ := s.CreateNode("B", []string{"task"})
	e, _ := s.CreateEdge("blocks", n1.ID, n2.ID, nil)

	idx := s.Index()
	if _, err := idx.GetNode(n1.ID); err != nil {
		t.Errorf("GetNode: %v", err)
	}
	edges := idx.EdgesFrom(n1.ID)
	found := false
	for _, ed := range edges {
		if ed.ID == e.ID {
			found = true
		}
	}
	if !found {
		t.Error("expected edge in index")
	}

	byType := idx.NodesByType("task")
	if len(byType) < 2 {
		t.Errorf("NodesByType returned %d nodes, want >= 2", len(byType))
	}
}

// errorAs is a helper to avoid importing errors package at top level.
func errorAs(err error, target interface{}) bool {
	type asInterface interface {
		As(interface{}) bool
	}
	// Simple type assertion approach.
	switch t := target.(type) {
	case **types.NotFoundError:
		if nfe, ok := err.(*types.NotFoundError); ok {
			*t = nfe
			return true
		}
	}
	return false
}
