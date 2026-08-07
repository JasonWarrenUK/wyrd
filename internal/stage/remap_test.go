package stage_test

import (
	"errors"
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/stage"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// fakeIndex is a minimal types.GraphIndex backed by a fixed node slice.
// DetectOrphans only calls AllNodes; every other method is an unused stub.
type fakeIndex struct {
	nodes []*types.Node
}

func (f *fakeIndex) GetNode(id string) (*types.Node, error)    { return nil, nil }
func (f *fakeIndex) GetEdge(id string) (*types.Edge, error)    { return nil, nil }
func (f *fakeIndex) AllNodes() []*types.Node                   { return f.nodes }
func (f *fakeIndex) AllEdges() []*types.Edge                   { return nil }
func (f *fakeIndex) EdgesFrom(nodeID string) []*types.Edge     { return nil }
func (f *fakeIndex) EdgesTo(nodeID string) []*types.Edge       { return nil }
func (f *fakeIndex) NodesByType(typeName string) []*types.Node { return nil }

func node(id, kind, stageVal string) *types.Node {
	return &types.Node{ID: id, Kind: kind, Stage: stageVal, Types: []string{"task"}}
}

func archivedNode(id, kind, stageVal string) *types.Node {
	n := node(id, kind, stageVal)
	n.Properties = map[string]interface{}{"status": "archived"}
	return n
}

func taskFlowKinds() (*types.KindRegistry, *types.StageGroupRegistry) {
	groups := types.NewStageGroupRegistry([]types.StageGroup{
		{Name: "task-flow", Stages: []string{"Open", "In Progress", "Done"}, Cycle: types.CycleTerminate},
	})
	kinds := types.NewKindRegistry([]types.Kind{
		{Name: "Task", StageGroup: "task-flow"},
		{Name: "Goblin", StageGroup: "task-flow"},
	})
	return kinds, groups
}

func TestDetectOrphansHealthyGraphIsEmpty(t *testing.T) {
	kinds, groups := taskFlowKinds()
	idx := &fakeIndex{nodes: []*types.Node{
		node("n1", "Task", "Open"),
		node("n2", "Task", "Done"),
	}}

	report := stage.DetectOrphans(idx, kinds, groups)

	if !report.IsEmpty() {
		t.Errorf("expected empty report, got %d orphans", len(report.Orphans))
	}
}

func TestDetectOrphansSkipsUntriagedNodes(t *testing.T) {
	kinds, groups := taskFlowKinds()
	idx := &fakeIndex{nodes: []*types.Node{
		node("n1", "", ""),     // no kind assigned at all
		node("n2", "Task", ""), // kind assigned, never staged
	}}

	report := stage.DetectOrphans(idx, kinds, groups)

	if !report.IsEmpty() {
		t.Errorf("expected empty report for untriaged nodes, got %d orphans", len(report.Orphans))
	}
	if len(report.Unresolvable) != 0 {
		t.Errorf("untriaged nodes should not be reported as unresolvable, got %v", report.Unresolvable)
	}
}

func TestDetectOrphansSkipsArchivedNodes(t *testing.T) {
	kinds, groups := taskFlowKinds()
	idx := &fakeIndex{nodes: []*types.Node{
		archivedNode("n1", "Task", "Maybe"), // orphaned stage, but archived
	}}

	report := stage.DetectOrphans(idx, kinds, groups)

	if !report.IsEmpty() {
		t.Errorf("expected archived orphan to be skipped, got %d orphans", len(report.Orphans))
	}
}

func TestDetectOrphansGroupsByKindAndStage(t *testing.T) {
	kinds, groups := taskFlowKinds()
	idx := &fakeIndex{nodes: []*types.Node{
		node("n1", "Task", "Maybe"),
		node("n2", "Task", "Maybe"),
	}}

	report := stage.DetectOrphans(idx, kinds, groups)

	if len(report.Orphans) != 1 {
		t.Fatalf("expected 1 orphan entry for two nodes sharing (kind, stage), got %d", len(report.Orphans))
	}
	o := report.Orphans[0]
	if o.Kind != "Task" || o.Stage != "Maybe" {
		t.Errorf("unexpected orphan identity: %+v", o)
	}
	if len(o.NodeIDs) != 2 {
		t.Errorf("expected 2 node IDs on the merged orphan, got %d: %v", len(o.NodeIDs), o.NodeIDs)
	}
	if report.NodeCount() != 2 {
		t.Errorf("NodeCount() = %d, want 2", report.NodeCount())
	}
}

// TestDetectOrphansSharedGroupAcrossKinds is the case a kind-scoped design
// would get wrong: two kinds share task-flow, and both hold the same
// orphaned stage name. The result must be two distinct Orphan entries (one
// per kind), each carrying only its own kind's nodes — a group-level fix
// (e.g. renaming Maybe) fans out into a decision per affected kind.
func TestDetectOrphansSharedGroupAcrossKinds(t *testing.T) {
	kinds, groups := taskFlowKinds()
	idx := &fakeIndex{nodes: []*types.Node{
		node("n1", "Task", "Maybe"),
		node("n2", "Goblin", "Maybe"),
	}}

	report := stage.DetectOrphans(idx, kinds, groups)

	if len(report.Orphans) != 2 {
		t.Fatalf("expected 2 distinct orphan entries (one per kind), got %d", len(report.Orphans))
	}
	byKind := map[string]stage.Orphan{}
	for _, o := range report.Orphans {
		byKind[o.Kind] = o
	}
	for _, k := range []string{"Task", "Goblin"} {
		o, ok := byKind[k]
		if !ok {
			t.Fatalf("expected an orphan entry for kind %q", k)
		}
		if len(o.NodeIDs) != 1 {
			t.Errorf("kind %q orphan should carry exactly its own node, got %v", k, o.NodeIDs)
		}
	}
}

func TestDetectOrphansUnresolvableUnknownKind(t *testing.T) {
	kinds, groups := taskFlowKinds()
	idx := &fakeIndex{nodes: []*types.Node{
		node("n1", "Sasquatch", "Open"), // kind not registered
	}}

	report := stage.DetectOrphans(idx, kinds, groups)

	if !report.IsEmpty() {
		t.Errorf("unresolvable nodes should not appear in Orphans, got %d", len(report.Orphans))
	}
	if len(report.Unresolvable) != 1 || report.Unresolvable[0] != "n1" {
		t.Errorf("expected [n1] in Unresolvable, got %v", report.Unresolvable)
	}
}

func TestDetectOrphansUnresolvableMissingGroup(t *testing.T) {
	// Kind references a group that isn't in the registry at all — e.g. the
	// group failed Validate and ReadStages silently dropped it.
	groups := types.NewStageGroupRegistry(nil)
	kinds := types.NewKindRegistry([]types.Kind{
		{Name: "Task", StageGroup: "task-flow"},
	})
	idx := &fakeIndex{nodes: []*types.Node{
		node("n1", "Task", "Open"),
	}}

	report := stage.DetectOrphans(idx, kinds, groups)

	if len(report.Unresolvable) != 1 || report.Unresolvable[0] != "n1" {
		t.Errorf("expected [n1] in Unresolvable, got %v", report.Unresolvable)
	}
}

func TestDetectOrphansStableOrder(t *testing.T) {
	kinds, groups := taskFlowKinds()
	idx := &fakeIndex{nodes: []*types.Node{
		node("n1", "Goblin", "Zeta"),
		node("n2", "Task", "Alpha"),
		node("n3", "Task", "Beta"),
	}}

	first := stage.DetectOrphans(idx, kinds, groups)
	second := stage.DetectOrphans(idx, kinds, groups)

	if len(first.Orphans) != len(second.Orphans) {
		t.Fatalf("orphan count differs across runs: %d vs %d", len(first.Orphans), len(second.Orphans))
	}
	for i := range first.Orphans {
		if first.Orphans[i].Kind != second.Orphans[i].Kind || first.Orphans[i].Stage != second.Orphans[i].Stage {
			t.Errorf("order differs at index %d: %+v vs %+v", i, first.Orphans[i], second.Orphans[i])
		}
	}
	// Kind then Stage, ascending: Goblin < Task, and within Task, Alpha < Beta.
	if first.Orphans[0].Kind != "Goblin" {
		t.Errorf("expected Goblin first (kind sorts before Task), got %q", first.Orphans[0].Kind)
	}
}

// TestSuggestTargetNameMatch covers the case-insensitive name-match default:
// a node holds "doing" (lowercase, orphaned) and the group has "Doing" —
// Suggested should propose the exact-cased match rather than falling back
// to the group's first stage.
func TestSuggestTargetNameMatch(t *testing.T) {
	orphanGroup := types.StageGroup{Name: "g", Stages: []string{"Backlog", "Doing", "Done"}, Cycle: types.CycleTerminate}
	kinds := types.NewKindRegistry([]types.Kind{{Name: "Task", StageGroup: "g"}})
	groups := types.NewStageGroupRegistry([]types.StageGroup{orphanGroup})
	idx := &fakeIndex{nodes: []*types.Node{node("n1", "Task", "doing")}} // lowercase, orphaned

	report := stage.DetectOrphans(idx, kinds, groups)
	if len(report.Orphans) != 1 {
		t.Fatalf("expected 1 orphan, got %d", len(report.Orphans))
	}
	if report.Orphans[0].Suggested != "Doing" {
		t.Errorf("Suggested = %q, want case-insensitive match %q", report.Orphans[0].Suggested, "Doing")
	}
}

func TestSuggestTargetFallsBackToFirstStage(t *testing.T) {
	kinds, groups := taskFlowKinds() // task-flow: Open, In Progress, Done
	idx := &fakeIndex{nodes: []*types.Node{node("n1", "Task", "Someday")}}

	report := stage.DetectOrphans(idx, kinds, groups)
	if len(report.Orphans) != 1 {
		t.Fatalf("expected 1 orphan, got %d", len(report.Orphans))
	}
	if report.Orphans[0].Suggested != "Open" {
		t.Errorf("Suggested = %q, want fallback to first stage %q", report.Orphans[0].Suggested, "Open")
	}
}

// fakeStore is a minimal types.StoreFS recording UpdateNode calls. Only
// UpdateNode is exercised by ApplyRemap; every other method is unused,
// except ReadKinds/WriteKinds which rename_test.go's RenameStageGroup tests
// use — kindsSeed configures what ReadKinds returns, kindsReadErr injects a
// read failure, and lastWrittenKinds/kindsWritten record what WriteKinds saw.
type fakeStore struct {
	updates map[string]map[string]interface{}
	failIDs map[string]bool

	kindsSeed        []types.Kind
	kindsReadErr     error
	kindsWriteErr    error
	kindsWritten     bool
	lastWrittenKinds []types.Kind
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		updates: make(map[string]map[string]interface{}),
		failIDs: make(map[string]bool),
	}
}

func (s *fakeStore) UpdateNode(id string, updates map[string]interface{}) (*types.Node, error) {
	if s.failIDs[id] {
		return nil, errors.New("simulated write failure")
	}
	s.updates[id] = updates
	return &types.Node{ID: id}, nil
}

func (s *fakeStore) ReadNode(id string) (*types.Node, error)        { return nil, nil }
func (s *fakeStore) WriteNode(n *types.Node) error                  { return nil }
func (s *fakeStore) ReadEdge(id string) (*types.Edge, error)        { return nil, nil }
func (s *fakeStore) WriteEdge(e *types.Edge) error                  { return nil }
func (s *fakeStore) DeleteEdge(id string) error                     { return nil }
func (s *fakeStore) ArchiveNode(id string) error                    { return nil }
func (s *fakeStore) ReadTemplate(t string) (*types.Template, error) { return nil, nil }
func (s *fakeStore) AllTemplates() ([]*types.Template, error)       { return nil, nil }
func (s *fakeStore) ReadView(n string) (*types.SavedView, error)    { return nil, nil }
func (s *fakeStore) AllViews() ([]*types.SavedView, error)          { return nil, nil }
func (s *fakeStore) ReadRitual(n string) (*types.Ritual, error)     { return nil, nil }
func (s *fakeStore) AllRituals() ([]*types.Ritual, error)           { return nil, nil }
func (s *fakeStore) ReadTheme(n string) (*types.Theme, error)       { return nil, nil }
func (s *fakeStore) ReadConfig() (*types.Config, error)             { return nil, nil }
func (s *fakeStore) WriteConfig(c *types.Config) error              { return nil }
func (s *fakeStore) ReadKinds() (*types.KindRegistry, error) {
	if s.kindsReadErr != nil {
		return nil, s.kindsReadErr
	}
	return types.NewKindRegistry(s.kindsSeed), nil
}
func (s *fakeStore) WriteKinds(k []types.Kind) error {
	s.kindsWritten = true
	s.lastWrittenKinds = k
	return s.kindsWriteErr
}
func (s *fakeStore) ReadStages() (*types.StageGroupRegistry, error) {
	return types.NewStageGroupRegistry(nil), nil
}
func (s *fakeStore) WriteStages(g []types.StageGroup) error { return nil }
func (s *fakeStore) StorePath() string                      { return "/tmp/fake-store" }

func sampleReport() stage.OrphanReport {
	group := types.StageGroup{Name: "task-flow", Stages: []string{"Open", "In Progress", "Done"}, Cycle: types.CycleTerminate}
	return stage.OrphanReport{
		Orphans: []stage.Orphan{
			{Kind: "Task", Stage: "Maybe", Group: group, NodeIDs: []string{"n1", "n2"}, Suggested: "Open"},
			{Kind: "Task", Stage: "Someday", Group: group, NodeIDs: []string{"n3"}, Suggested: "Open"},
		},
	}
}

func TestApplyRemapWritesViaUpdateNode(t *testing.T) {
	store := newFakeStore()
	report := sampleReport()
	choices := map[stage.OrphanKey]string{
		{Kind: "Task", Stage: "Maybe"}: "In Progress",
	}

	written, err := stage.ApplyRemap(store, report, choices, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if written != 2 {
		t.Errorf("written = %d, want 2", written)
	}
	for _, id := range []string{"n1", "n2"} {
		got, ok := store.updates[id]
		if !ok {
			t.Fatalf("expected UpdateNode call for %s", id)
		}
		if len(got) != 1 || got["stage"] != "In Progress" {
			t.Errorf("update map for %s = %v, want exactly {stage: In Progress}", id, got)
		}
	}
	if _, ok := store.updates["n3"]; ok {
		t.Error("n3 should not be written — its orphan key had no choice supplied")
	}
}

func TestApplyRemapSkipsSentinel(t *testing.T) {
	store := newFakeStore()
	report := sampleReport()
	choices := map[stage.OrphanKey]string{
		{Kind: "Task", Stage: "Maybe"}: "", // leave unchanged
	}

	written, err := stage.ApplyRemap(store, report, choices, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if written != 0 {
		t.Errorf("written = %d, want 0", written)
	}
	if len(store.updates) != 0 {
		t.Errorf("expected no writes, got %v", store.updates)
	}
}

func TestApplyRemapSkipsUnchangedTarget(t *testing.T) {
	store := newFakeStore()
	report := sampleReport()
	choices := map[stage.OrphanKey]string{
		{Kind: "Task", Stage: "Maybe"}: "Maybe", // target equals current stage
	}

	written, err := stage.ApplyRemap(store, report, choices, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if written != 0 {
		t.Errorf("written = %d, want 0", written)
	}
}

func TestApplyRemapDryRun(t *testing.T) {
	store := newFakeStore()
	report := sampleReport()
	choices := map[stage.OrphanKey]string{
		{Kind: "Task", Stage: "Maybe"}:   "In Progress",
		{Kind: "Task", Stage: "Someday"}: "Done",
	}

	written, err := stage.ApplyRemap(store, report, choices, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if written != 3 {
		t.Errorf("written = %d, want 3 (2 + 1)", written)
	}
	if len(store.updates) != 0 {
		t.Errorf("dry run must not write anything, got %v", store.updates)
	}
}

func TestApplyRemapPartialFailure(t *testing.T) {
	store := newFakeStore()
	store.failIDs["n2"] = true
	report := sampleReport()
	choices := map[stage.OrphanKey]string{
		{Kind: "Task", Stage: "Maybe"}:   "In Progress", // n1 succeeds, n2 fails
		{Kind: "Task", Stage: "Someday"}: "Done",        // n3 succeeds
	}

	written, err := stage.ApplyRemap(store, report, choices, false)
	if err == nil {
		t.Fatal("expected an error reporting the partial failure")
	}
	if written != 2 {
		t.Errorf("written = %d, want 2 (n1 and n3 succeed despite n2 failing)", written)
	}
	if _, ok := store.updates["n1"]; !ok {
		t.Error("n1 should have been written")
	}
	if _, ok := store.updates["n3"]; !ok {
		t.Error("n3 should have been written")
	}
	if _, ok := store.updates["n2"]; ok {
		t.Error("n2 should not appear in updates — it failed")
	}
}
