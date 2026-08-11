package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/jasonwarrenuk/wyrd/internal/cli"
	"github.com/jasonwarrenuk/wyrd/internal/store"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// newCompactTestStore creates a minimal store for Compact CLI tests.
func newCompactTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(t.TempDir(), types.RealClock{})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	return s
}

// TestCompact_NoArchivedNodes verifies that Compact reports nothing to do when
// all nodes are active.
func TestCompact_NoArchivedNodes(t *testing.T) {
	s := newCompactTestStore(t)

	if _, err := s.CreateNode("active node", []string{"task"}); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	var out bytes.Buffer
	if err := cli.Compact(s, s.Index(), false, &out); err != nil {
		t.Fatalf("Compact: %v", err)
	}

	if !strings.Contains(out.String(), "Nothing to compact") {
		t.Errorf("expected 'Nothing to compact' message, got: %s", out.String())
	}
}

// TestCompact_DryRunLeavesFilesInPlace verifies that dry-run prints a preview
// but does not move any files.
func TestCompact_DryRunLeavesFilesInPlace(t *testing.T) {
	s := newCompactTestStore(t)

	node, err := s.CreateNode("something to archive", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	if err := s.ArchiveNode(node.ID); err != nil {
		t.Fatalf("ArchiveNode: %v", err)
	}

	var out bytes.Buffer
	if err := cli.Compact(s, s.Index(), true, &out); err != nil {
		t.Fatalf("Compact dry-run: %v", err)
	}

	// Output must mention dry run.
	if !strings.Contains(out.String(), "dry run") {
		t.Errorf("expected dry run output, got: %s", out.String())
	}

	// Original file must still be in nodes/.
	originalPath := filepath.Join(s.StorePath(), "nodes", node.ID+".jsonc")
	if _, err := os.Stat(originalPath); os.IsNotExist(err) {
		t.Errorf("dry-run should not have moved file from %s", originalPath)
	}

	// Archive destination must NOT exist.
	archivePath := filepath.Join(s.StorePath(), "archive", "nodes", node.ID+".jsonc")
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Errorf("dry-run should not have created archive file at %s", archivePath)
	}
}

// TestCompact_MovesArchivedNodes verifies that archived nodes are moved to
// archive/nodes/ and active nodes are left in place.
func TestCompact_MovesArchivedNodes(t *testing.T) {
	s := newCompactTestStore(t)

	active, err := s.CreateNode("active node", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode active: %v", err)
	}

	archived, err := s.CreateNode("archived node", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode archived: %v", err)
	}
	if err := s.ArchiveNode(archived.ID); err != nil {
		t.Fatalf("ArchiveNode: %v", err)
	}

	var out bytes.Buffer
	if err := cli.Compact(s, s.Index(), false, &out); err != nil {
		t.Fatalf("Compact: %v", err)
	}

	output := out.String()

	// Summary line must mention 1 node moved.
	if !strings.Contains(output, "1 node(s)") {
		t.Errorf("expected '1 node(s)' in output, got: %s", output)
	}

	// Archived node must be in archive/nodes/.
	archivePath := filepath.Join(s.StorePath(), "archive", "nodes", archived.ID+".jsonc")
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		t.Errorf("archived node not found at %s", archivePath)
	}

	// Archived node original file must be gone.
	originalPath := filepath.Join(s.StorePath(), "nodes", archived.ID+".jsonc")
	if _, err := os.Stat(originalPath); !os.IsNotExist(err) {
		t.Errorf("archived node should have been moved from %s", originalPath)
	}

	// Active node must still be in nodes/.
	activePath := filepath.Join(s.StorePath(), "nodes", active.ID+".jsonc")
	if _, err := os.Stat(activePath); os.IsNotExist(err) {
		t.Errorf("active node should still exist at %s", activePath)
	}
}

// TestCompact_MovesOrphanEdges verifies that edges touching an archived node
// are also moved to archive/edges/.
func TestCompact_MovesOrphanEdges(t *testing.T) {
	s := newCompactTestStore(t)

	nodeA, err := s.CreateNode("node a", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode a: %v", err)
	}
	nodeB, err := s.CreateNode("node b", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode b: %v", err)
	}

	edge, err := s.CreateEdge("blocks", nodeA.ID, nodeB.ID, nil)
	if err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}

	// Archive nodeA — the edge touches it so should also be archived.
	if err := s.ArchiveNode(nodeA.ID); err != nil {
		t.Fatalf("ArchiveNode: %v", err)
	}

	var out bytes.Buffer
	if err := cli.Compact(s, s.Index(), false, &out); err != nil {
		t.Fatalf("Compact: %v", err)
	}

	output := out.String()

	if !strings.Contains(output, "1 edge(s)") {
		t.Errorf("expected '1 edge(s)' in output, got: %s", output)
	}

	archiveEdgePath := filepath.Join(s.StorePath(), "archive", "edges", edge.ID+".jsonc")
	if _, err := os.Stat(archiveEdgePath); os.IsNotExist(err) {
		t.Errorf("orphan edge not found at %s", archiveEdgePath)
	}
}

// TestCompact_EvictsGhostNodesFromLiveIndex verifies that Compact evicts
// archived nodes (and their orphan edges) from the live in-memory index via
// types.Compactor, not just from disk. Before TD.8 this was store.Compact's
// job only, and store.Compact was dead code outside its own tests — a
// running TUI backed by the same *store.Store as cli.Compact would keep
// serving nodes that had just been moved out of nodes/ on disk.
func TestCompact_EvictsGhostNodesFromLiveIndex(t *testing.T) {
	s := newCompactTestStore(t)

	nodeA, err := s.CreateNode("node a", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode a: %v", err)
	}
	nodeB, err := s.CreateNode("node b", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode b: %v", err)
	}
	edge, err := s.CreateEdge("blocks", nodeA.ID, nodeB.ID, nil)
	if err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}
	if err := s.ArchiveNode(nodeA.ID); err != nil {
		t.Fatalf("ArchiveNode: %v", err)
	}

	var out bytes.Buffer
	if err := cli.Compact(s, s.Index(), false, &out); err != nil {
		t.Fatalf("Compact: %v", err)
	}

	if _, err := s.Index().GetNode(nodeA.ID); err == nil {
		t.Errorf("GetNode(nodeA) should error after compaction — index should no longer serve the ghost node")
	}
	if _, err := s.Index().GetEdge(edge.ID); err == nil {
		t.Errorf("GetEdge should error after compaction — orphan edge should be evicted")
	}
	// nodeB was never archived and must remain findable.
	if _, err := s.Index().GetNode(nodeB.ID); err != nil {
		t.Errorf("GetNode(nodeB) should still succeed: %v", err)
	}
}

// TestCompact_DryRunDoesNotEvictFromIndex verifies that a dry run leaves the
// live index untouched, matching the on-disk no-op.
func TestCompact_DryRunDoesNotEvictFromIndex(t *testing.T) {
	s := newCompactTestStore(t)

	node, err := s.CreateNode("archived node", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	if err := s.ArchiveNode(node.ID); err != nil {
		t.Fatalf("ArchiveNode: %v", err)
	}

	var out bytes.Buffer
	if err := cli.Compact(s, s.Index(), true, &out); err != nil {
		t.Fatalf("Compact dry-run: %v", err)
	}

	if _, err := s.Index().GetNode(node.ID); err != nil {
		t.Errorf("GetNode should still succeed after dry-run compaction: %v", err)
	}
}

// TestCompact_DryRunShowsPreviewForMultipleNodes verifies the "would move"
// lines appear for each archived node in dry-run mode.
func TestCompact_DryRunShowsPreviewForMultipleNodes(t *testing.T) {
	s := newCompactTestStore(t)

	for i := 0; i < 3; i++ {
		node, err := s.CreateNode("archived item", []string{"task"})
		if err != nil {
			t.Fatalf("CreateNode %d: %v", i, err)
		}
		if err := s.ArchiveNode(node.ID); err != nil {
			t.Fatalf("ArchiveNode %d: %v", i, err)
		}
	}

	var out bytes.Buffer
	if err := cli.Compact(s, s.Index(), true, &out); err != nil {
		t.Fatalf("Compact dry-run: %v", err)
	}

	output := out.String()

	if !strings.Contains(output, "3 node(s)") {
		t.Errorf("expected '3 node(s)' in dry-run summary, got: %s", output)
	}
	if !strings.Contains(output, "would move node") {
		t.Errorf("expected 'would move node' lines, got: %s", output)
	}
}

// TestCompact_TruncatesMultiByteBodyWithoutMojibake covers TD.17: the
// dry-run label fallback used to byte-slice node.Body (body[:40]), which
// mismeasures multi-byte UTF-8 length and can split a rune mid-sequence.
// A body of CJK characters past the 40-rune threshold must truncate cleanly.
func TestCompact_TruncatesMultiByteBodyWithoutMojibake(t *testing.T) {
	s := newCompactTestStore(t)

	longBody := strings.Repeat("日本語のテスト文字列です。", 5) // well past 40 runes, untitled
	node, err := s.CreateNode(longBody, []string{"note"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	if err := s.ArchiveNode(node.ID); err != nil {
		t.Fatalf("ArchiveNode: %v", err)
	}

	var out bytes.Buffer
	if err := cli.Compact(s, s.Index(), true, &out); err != nil {
		t.Fatalf("Compact dry-run: %v", err)
	}

	output := out.String()
	if !utf8.ValidString(output) {
		t.Fatalf("dry-run output is not valid UTF-8: %q", output)
	}
	if !strings.Contains(output, "…") {
		t.Errorf("expected an ellipsis-truncated label in output, got: %s", output)
	}
}
