package store

// Tests for TD.11 (a)(b)(c)(f): ReadNode error conflation, buildIndex
// silent skips, core-field collision guards, and the template cache alias
// fix. In-package (not store_test) so the tests can reach unexported
// package state: nodeCoreFields, edgeCoreFields, and s.templates.

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// (a) ReadNode error conflation.

// TestReadNode_PermissionErrorIsNotNotFoundError asserts that a read
// failure caused by a permissions error (as opposed to a genuinely missing
// file) is surfaced as a wrapped error, not conflated with NotFoundError.
// ReadEdge already has this shape; ReadNode was the sole outlier.
func TestReadNode_PermissionErrorIsNotNotFoundError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 0000 does not deny reads on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("root ignores file permission bits")
	}

	s := newTestStore(t)
	defer func() {
		if err := s.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	}()

	node, err := s.CreateNode("permission test", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	path := s.nodePath(node.ID)
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(path, 0o644) //nolint:errcheck // best-effort cleanup

	_, err = s.ReadNode(node.ID)
	if err == nil {
		t.Fatal("expected an error reading a permission-denied node file")
	}
	var notFound *types.NotFoundError
	if errors.As(err, &notFound) {
		t.Fatalf("expected a wrapped read error, got NotFoundError: %v", err)
	}
}

// (b) buildIndex silent skips.

// TestBuildIndex_UnreadableStoreDirReturnsError asserts that a directory
// that exists but cannot be read (permissions) is distinguishable from a
// directory that doesn't exist yet (first run) — buildIndex previously
// returned nil for both.
func TestBuildIndex_UnreadableStoreDirReturnsError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 0000 does not deny directory listing on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("root ignores file permission bits")
	}

	dir := t.TempDir()
	clock := &fixedClock{}

	s, err := New(dir, clock)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	nodesDir := filepath.Join(dir, "nodes")
	if err := os.Chmod(nodesDir, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(nodesDir, 0o755) //nolint:errcheck // best-effort cleanup

	if _, err := New(dir, clock); err == nil {
		t.Fatal("expected New to fail when the nodes directory is unreadable")
	}
}

// TestBuildIndex_SkipsUnparsableNodeFile asserts that a single malformed
// node file is skipped (with a warning) rather than aborting the whole
// index build, and that well-formed sibling files are still indexed.
func TestBuildIndex_SkipsUnparsableNodeFile(t *testing.T) {
	dir := t.TempDir()
	clock := &fixedClock{}

	s, err := New(dir, clock)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	good, err := s.CreateNode("a good node", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Write a garbage node file directly, bypassing the store.
	badPath := filepath.Join(dir, "nodes", "not-json.jsonc")
	if err := os.WriteFile(badPath, []byte("{ this is not valid json"), 0o644); err != nil {
		t.Fatalf("writing garbage node file: %v", err)
	}

	s2, err := New(dir, clock)
	if err != nil {
		t.Fatalf("New should not fail on a single unparsable node file: %v", err)
	}
	defer func() {
		if err := s2.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	}()

	if _, err := s2.Index().GetNode(good.ID); err != nil {
		t.Errorf("expected the well-formed sibling node to still be indexed: %v", err)
	}
}

// (c) Core-field collision guards.

// TestWriteNode_PropertyCollidingWithCoreFieldIsDropped asserts that a
// Properties entry sharing a key with a core node field (e.g. "id") is
// dropped rather than silently overwriting the struct-backed value on disk.
func TestWriteNode_PropertyCollidingWithCoreFieldIsDropped(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	node, err := s.CreateNode("collision test", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	node.Properties["id"] = "attacker-controlled-id"
	node.Properties["custom_field"] = "kept"

	if err := s.WriteNode(node); err != nil {
		t.Fatalf("WriteNode: %v", err)
	}

	reread, err := s.ReadNode(node.ID)
	if err != nil {
		t.Fatalf("ReadNode: %v", err)
	}
	if reread.ID != node.ID {
		t.Errorf("expected core field id=%q to survive the write, got %q", node.ID, reread.ID)
	}
	if v, ok := reread.Properties["custom_field"]; !ok || v != "kept" {
		t.Errorf("expected non-colliding property to survive, got %v (ok=%v)", v, ok)
	}
	if _, ok := reread.Properties["id"]; ok {
		t.Error("expected colliding property key 'id' to be dropped from Properties on read-back")
	}
}

// TestCreateEdge_PropertyCollidingWithCoreFieldIsDropped mirrors the node
// case for CreateEdge's properties parameter.
func TestCreateEdge_PropertyCollidingWithCoreFieldIsDropped(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	a, err := s.CreateNode("a", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode a: %v", err)
	}
	b, err := s.CreateNode("b", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode b: %v", err)
	}

	edge, err := s.CreateEdge("blocks", a.ID, b.ID, map[string]interface{}{
		"type":   "attacker-controlled-type",
		"note":   "kept",
		"custom": "also kept",
	})
	if err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}

	if edge.Type != "blocks" {
		t.Errorf("expected core field type=%q to survive, got %q", "blocks", edge.Type)
	}
	if v, ok := edge.Properties["note"]; !ok || v != "kept" {
		t.Errorf("expected non-colliding property to survive, got %v (ok=%v)", v, ok)
	}
	if _, ok := edge.Properties["type"]; ok {
		t.Error("expected colliding property key 'type' to be dropped from Properties")
	}
}

// TestWriteEdge_PropertyCollidingWithCoreFieldIsDropped covers WriteEdge,
// which previously had no guard at all.
func TestWriteEdge_PropertyCollidingWithCoreFieldIsDropped(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	a, err := s.CreateNode("a", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode a: %v", err)
	}
	b, err := s.CreateNode("b", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode b: %v", err)
	}
	edge, err := s.CreateEdge("blocks", a.ID, b.ID, nil)
	if err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}

	edge.Properties["from"] = "attacker-controlled-from"
	edge.Properties["kept"] = "yes"

	if err := s.WriteEdge(edge); err != nil {
		t.Fatalf("WriteEdge: %v", err)
	}

	reread, err := s.ReadEdge(edge.ID)
	if err != nil {
		t.Fatalf("ReadEdge: %v", err)
	}
	if reread.From != a.ID {
		t.Errorf("expected core field from=%q to survive the write, got %q", a.ID, reread.From)
	}
	if v, ok := reread.Properties["kept"]; !ok || v != "yes" {
		t.Errorf("expected non-colliding property to survive, got %v (ok=%v)", v, ok)
	}
	if _, ok := reread.Properties["from"]; ok {
		t.Error("expected colliding property key 'from' to be dropped from Properties on read-back")
	}
}

// TestNodeCoreFields_IncludesLegacyFlatDateKeys guards the deliberate
// over-inclusion called out in node.go's doc comment: the legacy flat date
// keys must remain guarded so a property literally named "due" cannot
// shadow the nested date block.
func TestNodeCoreFields_IncludesLegacyFlatDateKeys(t *testing.T) {
	for _, key := range []string{"due", "about", "schedule", "start", "snooze_until", "date"} {
		if !nodeCoreFields[key] {
			t.Errorf("expected nodeCoreFields[%q] to be true", key)
		}
	}
}

// (f) Template cache fix.

// TestFieldsForType_ReturnedMapIsNotAliased asserts that mutating the map
// returned by FieldsForType does not corrupt the store's cached template.
func TestFieldsForType_ReturnedMapIsNotAliased(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	fields, err := s.FieldsForType("task")
	if err != nil {
		t.Fatalf("FieldsForType: %v", err)
	}
	if len(fields) == 0 {
		t.Fatal("expected the task template to have at least one field")
	}

	// Mutate the returned map: overwrite an existing entry and add a new one.
	for k := range fields {
		fields[k] = types.FieldDef{Type: types.FieldTypeString, Description: "corrupted"}
		break
	}
	fields["injected_field"] = types.FieldDef{Type: types.FieldTypeString}

	again, err := s.FieldsForType("task")
	if err != nil {
		t.Fatalf("FieldsForType (second call): %v", err)
	}
	if _, ok := again["injected_field"]; ok {
		t.Error("mutation of the first returned map leaked into the cache")
	}
	for k, v := range again {
		if v.Description == "corrupted" {
			t.Errorf("cached field %q was corrupted by a prior caller's mutation", k)
		}
	}
}
