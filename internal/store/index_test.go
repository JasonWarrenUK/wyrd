package store

import (
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// TestMemIndex_RemoveNodeOrphansNoEdges is the core TD.8 regression test:
// removing a node must also remove every edge incident to it (in either
// direction), not just the node's own from/to buckets. Before the fix,
// removeNode left the edge in idx.edges and in the *other* endpoint's
// bucket, so EdgesTo(B) would return an edge whose From no longer existed —
// exactly the shape internal/types/blocked.go trips over.
func TestMemIndex_RemoveNodeOrphansNoEdges(t *testing.T) {
	idx := newMemIndex()

	a := &types.Node{ID: "a"}
	b := &types.Node{ID: "b"}
	c := &types.Node{ID: "c"}
	idx.upsertNode(a)
	idx.upsertNode(b)
	idx.upsertNode(c)

	// a --blocks--> b (removing a should orphan this edge)
	ab := &types.Edge{ID: "ab", Type: "blocks", From: "a", To: "b"}
	// c --blocks--> a (removing a should also orphan this edge, since a is
	// the *To* endpoint here, not just the From endpoint)
	ca := &types.Edge{ID: "ca", Type: "blocks", From: "c", To: "a"}
	// b --blocks--> c (unrelated to a; must survive)
	bc := &types.Edge{ID: "bc", Type: "blocks", From: "b", To: "c"}
	idx.upsertEdge(ab)
	idx.upsertEdge(ca)
	idx.upsertEdge(bc)

	idx.removeNode("a")

	if _, err := idx.GetNode("a"); err == nil {
		t.Errorf("GetNode(a) should error after removal, got nil error")
	}

	if _, err := idx.GetEdge("ab"); err == nil {
		t.Errorf("GetEdge(ab) should error: edge incident to removed node a (as From) must be removed")
	}
	if _, err := idx.GetEdge("ca"); err == nil {
		t.Errorf("GetEdge(ca) should error: edge incident to removed node a (as To) must be removed")
	}
	if _, err := idx.GetEdge("bc"); err != nil {
		t.Errorf("GetEdge(bc) should still succeed: unrelated edge must survive: %v", err)
	}

	// EdgesTo/EdgesFrom on the *other* endpoint must not return a dangling
	// reference to the removed edges.
	if edges := idx.EdgesTo("b"); len(edges) != 0 {
		t.Errorf("EdgesTo(b) = %d edges, want 0 (ab must not dangle in b's 'to' bucket)", len(edges))
	}
	if edges := idx.EdgesFrom("c"); len(edges) != 0 {
		t.Errorf("EdgesFrom(c) = %d edges, want 0 (ca must not dangle in c's 'from' bucket)", len(edges))
	}

	// b and c themselves are untouched.
	if _, err := idx.GetNode("b"); err != nil {
		t.Errorf("GetNode(b) should still succeed: %v", err)
	}
	if _, err := idx.GetNode("c"); err != nil {
		t.Errorf("GetNode(c) should still succeed: %v", err)
	}
}

// TestMemIndex_RemoveNodeIsIdempotent verifies removeNode can be called
// twice for the same ID without panicking. This matters because compaction
// removes from the index synchronously and the watcher then fires for the
// same underlying filesystem removal shortly after.
func TestMemIndex_RemoveNodeIsIdempotent(t *testing.T) {
	idx := newMemIndex()

	a := &types.Node{ID: "a"}
	b := &types.Node{ID: "b"}
	idx.upsertNode(a)
	idx.upsertNode(b)
	idx.upsertEdge(&types.Edge{ID: "ab", Type: "blocks", From: "a", To: "b"})

	idx.removeNode("a")
	idx.removeNode("a") // must not panic on an already-absent node

	if _, err := idx.GetNode("a"); err == nil {
		t.Errorf("GetNode(a) should error after removal, got nil error")
	}
	if _, err := idx.GetEdge("ab"); err == nil {
		t.Errorf("GetEdge(ab) should error after its endpoint was removed, got nil error")
	}
}

// TestMemIndex_RemoveNodeSelfLoopEdge covers the edge case where From and To
// both equal the removed node — the incident-edge set collapses to a single
// ID gathered from both the from and to buckets, and must still be removed
// exactly once without a double-delete panic.
func TestMemIndex_RemoveNodeSelfLoopEdge(t *testing.T) {
	idx := newMemIndex()

	a := &types.Node{ID: "a"}
	idx.upsertNode(a)
	idx.upsertEdge(&types.Edge{ID: "self", Type: "related_to", From: "a", To: "a"})

	idx.removeNode("a")

	if _, err := idx.GetEdge("self"); err == nil {
		t.Errorf("GetEdge(self) should error after removal, got nil error")
	}
}
