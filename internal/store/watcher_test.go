package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// nodeEventPath returns the on-disk path handleWatchEvent expects for a node ID.
func nodeEventPath(s *Store, id string) string {
	return filepath.Join(s.path, "nodes", id+".jsonc")
}

// edgeEventPath returns the on-disk path handleWatchEvent expects for an edge ID.
func edgeEventPath(s *Store, id string) string {
	return filepath.Join(s.path, "edges", id+".jsonc")
}

// --- Direct handleWatchEvent calls: no fsnotify watcher, no timing.
//
// Remove/Rename handling stat-guards the event path to tell a genuine
// removal apart from a rename-over-an-existing-file overwrite (see
// handleWatchEvent's doc comment), so these tests still touch the
// filesystem directly — removing the file before firing a synthetic
// Remove/Rename event — to keep the on-disk state consistent with what the
// event claims happened.

// TestHandleWatchEvent_NodeRemoveEvictsFromIndex verifies that a Remove event
// for a node file evicts the node (and its incident edges) from the index.
func TestHandleWatchEvent_NodeRemoveEvictsFromIndex(t *testing.T) {
	s := newTestStore(t)

	a, err := s.CreateNode("node a", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode a: %v", err)
	}
	b, err := s.CreateNode("node b", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode b: %v", err)
	}
	edge, err := s.CreateEdge("blocks", a.ID, b.ID, nil)
	if err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}

	// handleWatchEvent stat-guards Remove/Rename to tell a genuine removal
	// apart from a rename-over-an-existing-file overwrite (the shape every
	// ordinary jsonc.WriteFile call takes) — so the file must actually be
	// gone on disk for the event to read as a real removal.
	if err := os.Remove(nodeEventPath(s, a.ID)); err != nil {
		t.Fatalf("os.Remove: %v", err)
	}

	s.handleWatchEvent(fsnotify.Event{
		Name: nodeEventPath(s, a.ID),
		Op:   fsnotify.Remove,
	})

	if _, err := s.index.GetNode(a.ID); err == nil {
		t.Errorf("GetNode(a) should error after Remove event, got nil error")
	}
	if _, err := s.index.GetEdge(edge.ID); err == nil {
		t.Errorf("GetEdge should error after its endpoint node was removed, got nil error")
	}
	// b was not removed and should remain.
	if _, err := s.index.GetNode(b.ID); err != nil {
		t.Errorf("GetNode(b) should still succeed: %v", err)
	}
}

// TestHandleWatchEvent_NodeRenameEvictsFromIndex verifies that a Rename event
// for a node file also evicts it — the compaction-synced-from-another-machine
// case, where a node file disappears from nodes/ via `git pull` rather than
// a direct local delete.
func TestHandleWatchEvent_NodeRenameEvictsFromIndex(t *testing.T) {
	s := newTestStore(t)

	n, err := s.CreateNode("renamed away", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Simulate the file having actually moved out of nodes/ (e.g. via
	// git pull picking up a compaction from another machine) before the
	// Rename event for the old path is handled.
	if err := os.Remove(nodeEventPath(s, n.ID)); err != nil {
		t.Fatalf("os.Remove: %v", err)
	}

	s.handleWatchEvent(fsnotify.Event{
		Name: nodeEventPath(s, n.ID),
		Op:   fsnotify.Rename,
	})

	if _, err := s.index.GetNode(n.ID); err == nil {
		t.Errorf("GetNode should error after Rename event, got nil error")
	}
}

// TestHandleWatchEvent_NodeRemoveIsIdempotent verifies that removeNode can be
// invoked twice for the same ID without panicking — compaction removes the
// node from the index synchronously, and the watcher then fires for the same
// filesystem removal shortly after.
func TestHandleWatchEvent_NodeRemoveIsIdempotent(t *testing.T) {
	s := newTestStore(t)

	n, err := s.CreateNode("node", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	if err := os.Remove(nodeEventPath(s, n.ID)); err != nil {
		t.Fatalf("os.Remove: %v", err)
	}

	event := fsnotify.Event{Name: nodeEventPath(s, n.ID), Op: fsnotify.Remove}
	s.handleWatchEvent(event)
	s.handleWatchEvent(event)

	if _, err := s.index.GetNode(n.ID); err == nil {
		t.Errorf("GetNode should error after repeated Remove events, got nil error")
	}
}

// TestHandleWatchEvent_NodeWriteUpdatesIndex verifies that a Write event for
// a node file re-reads the file and refreshes the index entry.
func TestHandleWatchEvent_NodeWriteUpdatesIndex(t *testing.T) {
	s := newTestStore(t)

	n, err := s.CreateNode("original body", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Change the body on disk directly, bypassing WriteNode (which would
	// upsert the index itself and defeat the point of this test).
	n.Body = "updated body"
	if err := writeJSONC(nodeEventPath(s, n.ID), nodeToRawMap(t, n)); err != nil {
		t.Fatalf("writeJSONC: %v", err)
	}

	s.handleWatchEvent(fsnotify.Event{
		Name: nodeEventPath(s, n.ID),
		Op:   fsnotify.Write,
	})

	got, err := s.index.GetNode(n.ID)
	if err != nil {
		t.Fatalf("GetNode after Write event: %v", err)
	}
	if got.Body != "updated body" {
		t.Errorf("Body = %q, want %q", got.Body, "updated body")
	}
}

// TestHandleWatchEvent_ChmodDoesNotEvict guards the deliberate omission of
// fsnotify.Chmod from removal handling: a permission change carries no
// removal semantics and must not evict a live node from the index.
func TestHandleWatchEvent_ChmodDoesNotEvict(t *testing.T) {
	s := newTestStore(t)

	n, err := s.CreateNode("node", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	s.handleWatchEvent(fsnotify.Event{
		Name: nodeEventPath(s, n.ID),
		Op:   fsnotify.Chmod,
	})

	if _, err := s.index.GetNode(n.ID); err != nil {
		t.Errorf("GetNode should still succeed after Chmod event: %v", err)
	}
}

// TestHandleWatchEvent_EdgeRenameEvictsFromIndex verifies that Rename is
// handled for edges alongside the pre-existing Remove handling.
func TestHandleWatchEvent_EdgeRenameEvictsFromIndex(t *testing.T) {
	s := newTestStore(t)

	a, err := s.CreateNode("node a", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode a: %v", err)
	}
	b, err := s.CreateNode("node b", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode b: %v", err)
	}
	edge, err := s.CreateEdge("blocks", a.ID, b.ID, nil)
	if err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}
	if err := os.Remove(edgeEventPath(s, edge.ID)); err != nil {
		t.Fatalf("os.Remove: %v", err)
	}

	s.handleWatchEvent(fsnotify.Event{
		Name: edgeEventPath(s, edge.ID),
		Op:   fsnotify.Rename,
	})

	if _, err := s.index.GetEdge(edge.ID); err == nil {
		t.Errorf("GetEdge should error after Rename event, got nil error")
	}
}

// --- Filesystem-driven tests: exercise the real fsnotify watcher started by
// New(). Gated behind testing.Short() since they depend on OS-level event
// delivery timing; a polling helper is used throughout, never a bare sleep.

// pollUntil polls cond every 10ms until it returns true or timeout elapses,
// failing the test if the deadline passes first.
func pollUntil(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !cond() {
		t.Fatalf("condition not met within %s", timeout)
	}
}

// awaitWatcherLive blocks until s's fsnotify watcher has demonstrably
// started delivering events, rather than sleeping a fixed guess. The
// underlying kqueue/inotify registration inside startWatcher is asynchronous
// relative to New() returning, so a burst of filesystem operations performed
// immediately after store creation can have its earliest events dropped —
// this is a real startup race in fsnotify itself, not specific to any one
// event type. Round-tripping a throwaway node write and polling for the
// index to observe it proves the watcher is live before a test's real
// assertion depends on event delivery.
func awaitWatcherLive(t *testing.T, s *Store) {
	t.Helper()
	warm, err := s.CreateNode("watcher warm-up", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode (warm-up): %v", err)
	}
	// CreateNode already upserts synchronously, so mutate the file directly
	// (bypassing the index) and poll for the watcher's own Write handling to
	// pick it up — that's the signal that proves events are flowing.
	warm.Body = "warmed"
	if err := writeJSONC(nodeEventPath(s, warm.ID), nodeToRawMap(t, warm)); err != nil {
		t.Fatalf("writeJSONC (warm-up): %v", err)
	}
	pollUntil(t, 2*time.Second, func() bool {
		got, err := s.index.GetNode(warm.ID)
		return err == nil && got.Body == "warmed"
	})
	if err := os.Remove(nodeEventPath(s, warm.ID)); err != nil {
		t.Fatalf("os.Remove (warm-up cleanup): %v", err)
	}
	pollUntil(t, 2*time.Second, func() bool {
		_, err := s.index.GetNode(warm.ID)
		return err != nil
	})
}

// awaitNodeFileWatched proves the watcher has a live, individually
// registered watch on the given node's specific file before the caller
// removes or renames it. kqueue (macOS, BSD) opens a file descriptor per
// watched file, and that per-file registration happens asynchronously after
// the directory-level Create event is delivered — awaitWatcherLive alone
// only proves the *directory* watch is live, not that this particular
// brand-new file's fd is registered yet. Without this, a remove/rename
// performed immediately after CreateNode can race the watcher's own
// internal setup and the event is silently dropped.
func awaitNodeFileWatched(t *testing.T, s *Store, n *types.Node) {
	t.Helper()
	n.Body = "watched"
	if err := writeJSONC(nodeEventPath(s, n.ID), nodeToRawMap(t, n)); err != nil {
		t.Fatalf("writeJSONC (watch confirmation): %v", err)
	}
	pollUntil(t, 2*time.Second, func() bool {
		got, err := s.index.GetNode(n.ID)
		return err == nil && got.Body == "watched"
	})
}

// TestWatcher_NodeFileRemovalEvictsFromIndex drives a real filesystem removal
// through the fsnotify watcher started by New(), simulating what cli.Compact
// does when it renames an archived node's file out of nodes/.
func TestWatcher_NodeFileRemovalEvictsFromIndex(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping filesystem-driven watcher test in short mode")
	}
	s := newTestStore(t)
	awaitWatcherLive(t, s)

	n, err := s.CreateNode("to be removed", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	awaitNodeFileWatched(t, s, n)

	if err := os.Remove(nodeEventPath(s, n.ID)); err != nil {
		t.Fatalf("os.Remove: %v", err)
	}

	pollUntil(t, 2*time.Second, func() bool {
		_, err := s.index.GetNode(n.ID)
		return err != nil
	})
}

// TestWatcher_NodeFileRenameEvictsFromIndex drives a real rename (as opposed
// to a delete) through the watcher — the shape a `git pull` of someone
// else's compaction takes, moving the file out of nodes/ without an explicit
// os.Remove on this machine.
func TestWatcher_NodeFileRenameEvictsFromIndex(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping filesystem-driven watcher test in short mode")
	}
	s := newTestStore(t)
	awaitWatcherLive(t, s)

	n, err := s.CreateNode("to be renamed away", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	awaitNodeFileWatched(t, s, n)

	dst := filepath.Join(s.path, "archive", "nodes", n.ID+".jsonc")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.Rename(nodeEventPath(s, n.ID), dst); err != nil {
		t.Fatalf("os.Rename: %v", err)
	}

	pollUntil(t, 2*time.Second, func() bool {
		_, err := s.index.GetNode(n.ID)
		return err != nil
	})
}

// TestWatcher_NodeFileWriteUpdatesIndex drives a real write through the
// watcher and confirms the index reflects the new content.
func TestWatcher_NodeFileWriteUpdatesIndex(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping filesystem-driven watcher test in short mode")
	}
	s := newTestStore(t)
	awaitWatcherLive(t, s)

	n, err := s.CreateNode("original", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	n.Body = "changed via watcher"
	if err := s.WriteNode(n); err != nil {
		t.Fatalf("WriteNode: %v", err)
	}

	pollUntil(t, 2*time.Second, func() bool {
		got, err := s.index.GetNode(n.ID)
		return err == nil && got.Body == "changed via watcher"
	})
}

// TestWatcher_ChmodDoesNotEvict drives a real permission change through the
// watcher and confirms the node survives — guarding the deliberate omission
// of fsnotify.Chmod from the removal handling end to end, not just via the
// direct handleWatchEvent call.
func TestWatcher_ChmodDoesNotEvict(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping filesystem-driven watcher test in short mode")
	}
	s := newTestStore(t)
	awaitWatcherLive(t, s)

	n, err := s.CreateNode("chmod me", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	if err := os.Chmod(nodeEventPath(s, n.ID), 0o600); err != nil {
		t.Fatalf("os.Chmod: %v", err)
	}

	// There is no positive event to poll for here — a Chmod either doesn't
	// evict (correct) or the index is silently corrupted (bug). Give the
	// watcher goroutine a beat to process any (wrongly) queued event, then
	// assert the node is still present.
	time.Sleep(200 * time.Millisecond)

	if _, err := s.index.GetNode(n.ID); err != nil {
		t.Errorf("GetNode should still succeed after chmod: %v", err)
	}
}

// nodeToRawMap converts a *types.Node into the map[string]interface{} shape
// writeJSONC expects, mirroring the on-disk structure parseNode reads back.
// Kept minimal — only the fields these tests touch.
func nodeToRawMap(t *testing.T, n *types.Node) map[string]interface{} {
	t.Helper()
	m := map[string]interface{}{
		"id":    n.ID,
		"body":  n.Body,
		"types": n.Types,
	}
	if n.Title != "" {
		m["title"] = n.Title
	}
	for k, v := range n.Properties {
		m[k] = v
	}
	return m
}
