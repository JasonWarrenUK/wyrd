package ritual_test

import (
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/tui/ritual"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// ---- InterpolateTemplate tests ---------------------------------------------

func TestInterpolateTemplate_Basic(t *testing.T) {
	result := &types.QueryResult{
		Columns: []string{"total", "done"},
		Rows:    []map[string]interface{}{{"total": 10, "done": 3}},
	}
	output := ritual.InterpolateTemplate("{{total}} items, {{done}} done.", result)
	if output != "10 items, 3 done." {
		t.Errorf("unexpected output: %q", output)
	}
}

func TestInterpolateTemplate_EmptyResult(t *testing.T) {
	result := &types.QueryResult{}
	output := ritual.InterpolateTemplate("You have {{total}} nodes.", result)
	// Placeholders with no data should be removed.
	if output != "You have  nodes." {
		t.Errorf("unexpected output: %q", output)
	}
}

func TestInterpolateTemplate_NilResult(t *testing.T) {
	output := ritual.InterpolateTemplate("Total: {{total}}", nil)
	if output != "Total: " {
		t.Errorf("unexpected output: %q", output)
	}
}

func TestInterpolateTemplate_UnknownField(t *testing.T) {
	result := &types.QueryResult{
		Rows: []map[string]interface{}{{"total": 5}},
	}
	output := ritual.InterpolateTemplate("{{total}} items, {{unknown}} missing.", result)
	// {{unknown}} should be stripped.
	if output != "5 items,  missing." {
		t.Errorf("unexpected output: %q", output)
	}
}

// ---- CycleStatus tests -----------------------------------------------------

func TestCycleStatus_FromInbox(t *testing.T) {
	node := &types.Node{Properties: map[string]interface{}{"status": "inbox"}}
	item := &ritual.ListItem{Node: node}

	ritual.CycleStatus(item)

	if item.PendingStatus != "active" {
		t.Errorf("expected active, got %q", item.PendingStatus)
	}
}

func TestCycleStatus_Wraparound(t *testing.T) {
	node := &types.Node{Properties: map[string]interface{}{"status": "archived"}}
	item := &ritual.ListItem{Node: node}

	ritual.CycleStatus(item)

	if item.PendingStatus != "inbox" {
		t.Errorf("expected wraparound to inbox, got %q", item.PendingStatus)
	}
}

func TestCycleStatus_FromPending(t *testing.T) {
	// If there's already a pending status, cycle from that rather than the
	// node's persisted status.
	node := &types.Node{Properties: map[string]interface{}{"status": "inbox"}}
	item := &ritual.ListItem{Node: node, PendingStatus: "active"}

	ritual.CycleStatus(item)

	if item.PendingStatus != "waiting" {
		t.Errorf("expected waiting after cycling from active, got %q", item.PendingStatus)
	}
}

func TestCycleStatus_UnknownStatus(t *testing.T) {
	node := &types.Node{Properties: map[string]interface{}{"status": "someweirdvalue"}}
	item := &ritual.ListItem{Node: node}

	ritual.CycleStatus(item)

	// Unknown current value should land on the first in the cycle.
	if item.PendingStatus != "inbox" {
		t.Errorf("expected inbox for unknown starting status, got %q", item.PendingStatus)
	}
}

// ---- QuickArchive tests ----------------------------------------------------

func TestQuickArchive(t *testing.T) {
	node := &types.Node{Properties: map[string]interface{}{"status": "active"}}
	item := &ritual.ListItem{Node: node}

	ritual.QuickArchive(item)

	if item.PendingStatus != "archived" {
		t.Errorf("expected archived pending status, got %q", item.PendingStatus)
	}
	if !item.Archived {
		t.Error("expected Archived = true")
	}
}

// ---- CommitListEdits tests -------------------------------------------------

func TestCommitListEdits_StatusChange(t *testing.T) {
	store := newMockStore()
	clock := fixedClock()

	node := &types.Node{
		ID:         "node-1",
		Body:       "Do something",
		Types:      []string{"task"},
		Properties: map[string]interface{}{"status": "inbox"},
	}
	store.nodes["node-1"] = node

	items := []*ritual.ListItem{
		{Node: node, PendingStatus: "active"},
	}

	errs := ritual.CommitListEdits(items, store, clock)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	written := store.nodes["node-1"]
	if written.Properties["status"] != "active" {
		t.Errorf("expected status=active after commit, got %v", written.Properties["status"])
	}
}

func TestCommitListEdits_EdgeCreation(t *testing.T) {
	store := newMockStore()
	clock := fixedClock()

	node := &types.Node{
		ID:         "node-1",
		Body:       "Task",
		Types:      []string{"task"},
		Properties: map[string]interface{}{},
	}
	store.nodes["node-1"] = node

	items := []*ritual.ListItem{
		{
			Node: node,
			PendingEdge: &ritual.PendingEdge{
				EdgeType: "related",
				TargetID: "node-2",
			},
		},
	}

	errs := ritual.CommitListEdits(items, store, clock)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	if len(store.edges) != 1 {
		t.Fatalf("expected 1 edge written, got %d", len(store.edges))
	}

	for _, e := range store.edges {
		if e.From != "node-1" {
			t.Errorf("expected edge from node-1, got %q", e.From)
		}
		if e.To != "node-2" {
			t.Errorf("expected edge to node-2, got %q", e.To)
		}
		if e.Type != "related" {
			t.Errorf("expected edge type related, got %q", e.Type)
		}
	}
}

func TestCommitListEdits_NoPendingChanges(t *testing.T) {
	store := newMockStore()
	clock := fixedClock()

	node := &types.Node{
		ID:         "node-1",
		Body:       "Task",
		Types:      []string{"task"},
		Properties: map[string]interface{}{"status": "inbox"},
	}
	store.nodes["node-1"] = node

	// No pending changes.
	items := []*ritual.ListItem{{Node: node}}

	errs := ritual.CommitListEdits(items, store, clock)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	// Status should remain unchanged.
	if store.nodes["node-1"].Properties["status"] != "inbox" {
		t.Error("status should not have changed when no pending edit")
	}
}

// ---- BuildListItems tests --------------------------------------------------

func TestBuildListItems_WithNodeLoader(t *testing.T) {
	store := newMockStore()
	store.nodes["uuid-1"] = &types.Node{
		ID:    "uuid-1",
		Body:  "Write tests",
		Types: []string{"task"},
	}

	result := &types.QueryResult{
		Columns: []string{"id", "body"},
		Rows: []map[string]interface{}{
			{"id": "uuid-1", "body": "Write tests"},
			{"id": "uuid-missing", "body": "Ghost"},
		},
	}

	items := ritual.BuildListItems(result, store.ReadNode)

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Node == nil {
		t.Error("first item should have Node populated")
	}
	if items[1].Node != nil {
		t.Error("second item (missing node) should have nil Node")
	}
}

// ---- SetPendingEdge tests --------------------------------------------------

func TestSetPendingEdge(t *testing.T) {
	item := &ritual.ListItem{}
	ritual.SetPendingEdge(item, "blocks", "target-id")

	if item.PendingEdge == nil {
		t.Fatal("expected PendingEdge to be set")
	}
	if item.PendingEdge.EdgeType != "blocks" {
		t.Errorf("expected edge type blocks, got %q", item.PendingEdge.EdgeType)
	}
	if item.PendingEdge.TargetID != "target-id" {
		t.Errorf("expected target-id, got %q", item.PendingEdge.TargetID)
	}
}
