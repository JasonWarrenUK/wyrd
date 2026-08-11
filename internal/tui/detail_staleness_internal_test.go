package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jasonwarrenuk/wyrd/internal/query"
	"github.com/jasonwarrenuk/wyrd/internal/store"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// newModelWithThreeNodes builds a Model over a real store containing three
// nodes with distinguishable bodies, so a detail pane's rendered content can
// be attributed back to the node it came from.
func newModelWithThreeNodes(t *testing.T) (m Model, ids [3]string) {
	t.Helper()
	dir := t.TempDir()
	clock := types.StubClock{Fixed: time.Date(2026, 3, 23, 9, 0, 0, 0, time.UTC)}
	s, err := store.New(dir, clock)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	bodies := [3]string{"Node Alpha marker", "Node Beta marker", "Node Gamma marker"}
	for i, body := range bodies {
		n, err := s.CreateNode(body, []string{"task"})
		if err != nil {
			t.Fatalf("CreateNode(%d): %v", i, err)
		}
		ids[i] = n.ID
	}

	engine := query.NewEngine(s.Index(), 10)
	m2, err := New(Config{
		Store:       s,
		StorePath:   dir,
		Index:       s.Index(),
		QueryRunner: engine,
		Clock:       clock,
	})
	if err != nil {
		t.Fatalf("tui.New: %v", err)
	}
	updated, _ := m2.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(Model)
	return m, ids
}

// selectNode moves the mounted nodeListPane's selection to the row whose
// NodeID matches id, failing the test if the pane isn't a nodeListPane or
// the ID isn't present.
func selectNode(t *testing.T, m *Model, id string) {
	t.Helper()
	lp, ok := m.leftPane.(nodeListPane)
	if !ok {
		t.Fatalf("left pane is %T, want nodeListPane", m.leftPane)
	}
	for i, item := range lp.list.Items() {
		if n, ok := item.(nodeListItem); ok && n.NodeID() == id {
			lp.list.Select(i)
			m.leftPane = lp
			return
		}
	}
	t.Fatalf("node %s not found in list", id)
}

// paneContains reports whether the mounted right pane is a viewportPane
// whose rendered content contains marker.
func paneContains(m Model, marker string) bool {
	vp, ok := m.rightPane.(viewportPane)
	if !ok {
		return false
	}
	return strings.Contains(vp.rawContent, marker)
}

// TestDetailReadyStaleResultDropped asserts a detailReadyMsg for a node that
// is no longer selected is dropped rather than mounted — the core TD.7 fix.
func TestDetailReadyStaleResultDropped(t *testing.T) {
	m, ids := newModelWithThreeNodes(t)
	selectNode(t, &m, ids[0])

	// Mount A's detail first so we can tell a drop from a no-op.
	updated, _ := m.Update(detailReadyMsg{nodeID: ids[0], pane: m.renderDetail(ids[0])})
	m = updated.(Model)
	if !paneContains(m, "Node Alpha marker") {
		t.Fatal("expected node A's detail to be mounted before the stale delivery")
	}

	// Selection has since moved to B; a late result for A must be dropped.
	selectNode(t, &m, ids[1])
	stalePane := m.renderDetail(ids[0])
	updated, _ = m.Update(detailReadyMsg{nodeID: ids[0], pane: stalePane})
	m = updated.(Model)

	if !paneContains(m, "Node Alpha marker") {
		t.Fatal("stale detailReadyMsg for A should have been dropped, leaving A's earlier render in place")
	}
}

// TestDetailReadyCurrentResultAccepted asserts a detailReadyMsg matching the
// current selection is mounted normally.
func TestDetailReadyCurrentResultAccepted(t *testing.T) {
	m, ids := newModelWithThreeNodes(t)
	selectNode(t, &m, ids[1])

	updated, _ := m.Update(detailReadyMsg{nodeID: ids[1], pane: m.renderDetail(ids[1])})
	m = updated.(Model)

	if !paneContains(m, "Node Beta marker") {
		t.Fatal("expected node B's detail to be mounted for a current-selection result")
	}
}

// TestDetailReadyEmptyNodeIDStillMounts guards the deliberate msg.nodeID !=
// "" carve-out in the staleness guard: renderDetail intentionally returns an
// empty pane for an empty ID (e.g. an empty node list), and that result must
// still mount rather than being treated as stale.
func TestDetailReadyEmptyNodeIDStillMounts(t *testing.T) {
	m, ids := newModelWithThreeNodes(t)
	selectNode(t, &m, ids[0])

	// Mount something non-empty first so we can observe the empty result
	// actually landing, rather than the pane merely staying as it was.
	updated, _ := m.Update(detailReadyMsg{nodeID: ids[0], pane: m.renderDetail(ids[0])})
	m = updated.(Model)
	if !paneContains(m, "Node Alpha marker") {
		t.Fatal("expected node A's detail to be mounted as a baseline")
	}

	emptyPane := m.renderDetail("")
	updated, _ = m.Update(detailReadyMsg{nodeID: "", pane: emptyPane})
	m = updated.(Model)

	if paneContains(m, "Node Alpha marker") {
		t.Fatal("empty-nodeID detailReadyMsg should mount (per renderDetail's empty-ID contract), not be dropped as stale")
	}
}

// TestDetailReadyOutOfOrderArrival selects A, B, C in turn (as fast j/j/j
// would), then delivers detailReadyMsg for C, A, B in that arrival order —
// deliberately not the selection order. Wrong-order delivery is the race
// TD.7 guards against; this reproduces it deterministically without any
// real concurrency. The final pane must be C's, the last (and current)
// selection, regardless of delivery order.
func TestDetailReadyOutOfOrderArrival(t *testing.T) {
	m, ids := newModelWithThreeNodes(t)

	selectNode(t, &m, ids[0]) // select A
	selectNode(t, &m, ids[1]) // select B
	selectNode(t, &m, ids[2]) // select C — this is the current selection

	paneC := m.renderDetail(ids[2])
	paneA := m.renderDetail(ids[0])
	paneB := m.renderDetail(ids[1])

	// Deliver out of order: C, A, B.
	updated, _ := m.Update(detailReadyMsg{nodeID: ids[2], pane: paneC})
	m = updated.(Model)
	updated, _ = m.Update(detailReadyMsg{nodeID: ids[0], pane: paneA})
	m = updated.(Model)
	updated, _ = m.Update(detailReadyMsg{nodeID: ids[1], pane: paneB})
	m = updated.(Model)

	if !paneContains(m, "Node Gamma marker") {
		t.Fatal("expected the final mounted pane to be node C's (the current selection)")
	}
	if paneContains(m, "Node Alpha marker") || paneContains(m, "Node Beta marker") {
		t.Fatal("a stale out-of-order result overwrote the current selection's detail pane")
	}
}
