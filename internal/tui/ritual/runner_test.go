package ritual_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/tui/ritual"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// ---- Mock implementations --------------------------------------------------

// mockQueryRunner returns fixed results keyed by query string.
type mockQueryRunner struct {
	results map[string]*types.QueryResult
	err     map[string]error
}

func (m *mockQueryRunner) Run(query string, _ types.Clock) (*types.QueryResult, error) {
	if err, ok := m.err[query]; ok {
		return nil, err
	}
	if result, ok := m.results[query]; ok {
		return result, nil
	}
	return &types.QueryResult{}, nil
}

// mockStore records written nodes and edges keyed by ID.
type mockStore struct {
	nodes map[string]*types.Node
	edges map[string]*types.Edge
	views map[string]*types.SavedView
}

func newMockStore() *mockStore {
	return &mockStore{
		nodes: make(map[string]*types.Node),
		edges: make(map[string]*types.Edge),
		views: make(map[string]*types.SavedView),
	}
}

func (m *mockStore) ReadNode(id string) (*types.Node, error) {
	n, ok := m.nodes[id]
	if !ok {
		return nil, fmt.Errorf("node %s not found", id)
	}
	return n, nil
}
func (m *mockStore) WriteNode(n *types.Node) error {
	m.nodes[n.ID] = n
	return nil
}
func (m *mockStore) ReadEdge(id string) (*types.Edge, error) {
	e, ok := m.edges[id]
	if !ok {
		return nil, fmt.Errorf("edge %s not found", id)
	}
	return e, nil
}
func (m *mockStore) WriteEdge(e *types.Edge) error {
	m.edges[e.ID] = e
	return nil
}
func (m *mockStore) DeleteEdge(id string) error {
	delete(m.edges, id)
	return nil
}
func (m *mockStore) ReadTemplate(_ string) (*types.Template, error)  { return nil, nil }
func (m *mockStore) AllTemplates() ([]*types.Template, error)         { return nil, nil }
func (m *mockStore) ReadView(name string) (*types.SavedView, error) {
	v, ok := m.views[name]
	if !ok {
		return nil, fmt.Errorf("view %s not found", name)
	}
	return v, nil
}
func (m *mockStore) AllViews() ([]*types.SavedView, error)            { return nil, nil }
func (m *mockStore) ReadRitual(_ string) (*types.Ritual, error)       { return nil, nil }
func (m *mockStore) AllRituals() ([]*types.Ritual, error)             { return nil, nil }
func (m *mockStore) ReadTheme(_ string) (*types.Theme, error)         { return nil, nil }
func (m *mockStore) ReadConfig() (*types.Config, error)               { return nil, nil }
func (m *mockStore) WriteConfig(_ *types.Config) error                { return nil }
func (m *mockStore) StorePath() string                                 { return "/tmp/store" }

// ---- Fixtures --------------------------------------------------------------

func morningRitual() *types.Ritual {
	return &types.Ritual{
		Name:     "morning",
		Friction: types.FrictionGate,
		Schedule: types.RitualSchedule{
			Days: []string{"mon", "tue", "wed", "thu", "fri"},
			Time: "09:00",
		},
		Steps: []types.RitualStep{
			{
				Type:     types.StepQuerySummary,
				Label:    "Overview",
				Query:    "MATCH (n) RETURN count(n) AS total",
				Template: "You have {{total}} nodes today.",
			},
			{
				Type:         types.StepQueryList,
				Label:        "Inbox",
				Query:        "MATCH (n:task {status: 'inbox'}) RETURN n.id AS id, n.body AS body",
				EmptyMessage: "Nothing in inbox.",
			},
			{
				Type:   types.StepAction,
				Action: "propose_schedule",
			},
		},
	}
}

func fixedClock() types.Clock {
	return types.StubClock{Fixed: time.Date(2025, 3, 17, 9, 0, 0, 0, time.UTC)}
}

// ---- Runner tests ----------------------------------------------------------

func TestRunner_StepThroughMorningRitual(t *testing.T) {
	r := morningRitual()

	qRunner := &mockQueryRunner{
		results: map[string]*types.QueryResult{
			"MATCH (n) RETURN count(n) AS total": {
				Columns: []string{"total"},
				Rows:    []map[string]interface{}{{"total": 42}},
			},
			"MATCH (n:task {status: 'inbox'}) RETURN n.id AS id, n.body AS body": {
				Columns: []string{"id", "body"},
				Rows: []map[string]interface{}{
					{"id": "uuid-1", "body": "Review emails"},
					{"id": "uuid-2", "body": "Write tests"},
				},
			},
		},
	}

	store := newMockStore()
	runner := ritual.NewRunner(r, store, qRunner, fixedClock())

	// Step 0: query_summary
	if err := runner.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if runner.CurrentState() != ritual.StateRunning {
		t.Errorf("expected StateRunning after Start, got %v", runner.CurrentState())
	}
	if runner.CurrentOutput() != "You have 42 nodes today." {
		t.Errorf("unexpected output: %q", runner.CurrentOutput())
	}

	// Step 1: query_list
	if err := runner.Advance(); err != nil {
		t.Fatalf("Advance to step 1: %v", err)
	}
	items := runner.CurrentItems()
	if len(items) != 2 {
		t.Errorf("expected 2 list items, got %d", len(items))
	}

	// Step 2: action
	if err := runner.Advance(); err != nil {
		t.Fatalf("Advance to step 2: %v", err)
	}
	if runner.CurrentState() != ritual.StateRunning {
		t.Errorf("expected still running on action step, got %v", runner.CurrentState())
	}

	// Final advance — should complete.
	if err := runner.Advance(); err != nil {
		t.Fatalf("Advance past last step: %v", err)
	}
	if runner.CurrentState() != ritual.StateComplete {
		t.Errorf("expected StateComplete, got %v", runner.CurrentState())
	}
}

func TestRunner_GateFriction_BlocksLeave(t *testing.T) {
	r := morningRitual() // friction = gate

	qRunner := &mockQueryRunner{
		results: map[string]*types.QueryResult{
			"MATCH (n) RETURN count(n) AS total": {
				Rows: []map[string]interface{}{{"total": 0}},
			},
		},
	}
	store := newMockStore()
	runner := ritual.NewRunner(r, store, qRunner, fixedClock())

	if err := runner.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// TryDefer on a gate ritual should return false.
	deferred := runner.TryDefer()
	if deferred {
		t.Error("TryDefer should return false for gate ritual")
	}
	if runner.CurrentState() != ritual.StateRunning {
		t.Errorf("state should still be running after failed TryDefer, got %v", runner.CurrentState())
	}
}

func TestRunner_GateFriction_DeferSequence(t *testing.T) {
	r := morningRitual()

	qRunner := &mockQueryRunner{
		results: map[string]*types.QueryResult{
			"MATCH (n) RETURN count(n) AS total": {
				Rows: []map[string]interface{}{{"total": 0}},
			},
		},
	}
	store := newMockStore()
	runner := ritual.NewRunner(r, store, qRunner, fixedClock())

	if err := runner.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Type the defer sequence: Esc Esc d
	action, err := runner.HandleKey("Esc")
	if err != nil {
		t.Fatalf("HandleKey Esc: %v", err)
	}
	if action == "deferred" {
		t.Error("should not defer after first Esc")
	}

	action, err = runner.HandleKey("Esc")
	if err != nil {
		t.Fatalf("HandleKey Esc Esc: %v", err)
	}
	if action == "deferred" {
		t.Error("should not defer after second Esc")
	}

	action, err = runner.HandleKey("d")
	if err != nil {
		t.Fatalf("HandleKey d: %v", err)
	}
	if action != "deferred" {
		t.Errorf("expected action=deferred after full sequence, got %q", action)
	}
	if runner.CurrentState() != ritual.StateDeferred {
		t.Errorf("expected StateDeferred, got %v", runner.CurrentState())
	}
}

func TestRunner_GateFriction_BrokenSequence(t *testing.T) {
	r := morningRitual()

	qRunner := &mockQueryRunner{
		results: map[string]*types.QueryResult{
			"MATCH (n) RETURN count(n) AS total": {
				Rows: []map[string]interface{}{{"total": 0}},
			},
		},
	}
	store := newMockStore()
	runner := ritual.NewRunner(r, store, qRunner, fixedClock())

	if err := runner.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Start sequence, then break it with a different key.
	runner.HandleKey("Esc")
	runner.HandleKey("Esc")
	runner.HandleKey("x") // wrong key — breaks the sequence

	if runner.CurrentState() != ritual.StateRunning {
		t.Errorf("state should still be running after broken sequence, got %v", runner.CurrentState())
	}
}

func TestRunner_NudgeFriction_Defer(t *testing.T) {
	r := &types.Ritual{
		Name:     "nudge-ritual",
		Friction: types.FrictionNudge,
		Schedule: types.RitualSchedule{Days: []string{"mon"}, Time: "09:00"},
		Steps: []types.RitualStep{
			{Type: types.StepAction, Action: "sync"},
		},
	}

	qRunner := &mockQueryRunner{}
	store := newMockStore()
	runner := ritual.NewRunner(r, store, qRunner, fixedClock())

	if err := runner.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Nudge can be deferred with a single TryDefer call.
	deferred := runner.TryDefer()
	if !deferred {
		t.Error("TryDefer should return true for nudge ritual")
	}
	if runner.CurrentState() != ritual.StateDeferred {
		t.Errorf("expected StateDeferred after TryDefer on nudge, got %v", runner.CurrentState())
	}
}

func TestRunner_GateStep(t *testing.T) {
	r := &types.Ritual{
		Name:     "gate-test",
		Friction: types.FrictionNudge,
		Schedule: types.RitualSchedule{Days: []string{"mon"}, Time: "09:00"},
		Steps: []types.RitualStep{
			{
				Type:      types.StepGate,
				Condition: "MATCH (n) WHERE n.hasJournal = true RETURN n.hasJournal AS ok",
				Field:     "ok",
				Then: &types.RitualStep{
					Type:     types.StepQuerySummary,
					Template: "Journal done!",
					Query:    "",
				},
			},
		},
	}

	// Gate passes — then sub-step should execute.
	qRunner := &mockQueryRunner{
		results: map[string]*types.QueryResult{
			"MATCH (n) WHERE n.hasJournal = true RETURN n.hasJournal AS ok": {
				Rows: []map[string]interface{}{{"ok": true}},
			},
			"": {Rows: nil},
		},
	}

	store := newMockStore()
	runner := ritual.NewRunner(r, store, qRunner, fixedClock())

	if err := runner.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if runner.CurrentOutput() != "Journal done!" {
		t.Errorf("expected then sub-step output, got %q", runner.CurrentOutput())
	}
}

func TestRunner_PromptCreatesNode(t *testing.T) {
	r := &types.Ritual{
		Name:     "journal",
		Friction: types.FrictionNudge,
		Schedule: types.RitualSchedule{Days: []string{"mon"}, Time: "09:00"},
		Steps: []types.RitualStep{
			{
				Type: types.StepPrompt,
				Text: "What's on your mind today?",
				Creates: &types.PromptCreates{
					Types: []string{"journal"},
				},
			},
		},
	}

	qRunner := &mockQueryRunner{}
	store := newMockStore()
	runner := ritual.NewRunner(r, store, qRunner, fixedClock())

	if err := runner.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if runner.CurrentOutput() != "What's on your mind today?" {
		t.Errorf("unexpected prompt text: %q", runner.CurrentOutput())
	}

	if err := runner.SubmitPrompt("Today I feel focused and ready."); err != nil {
		t.Fatalf("SubmitPrompt: %v", err)
	}

	if runner.CurrentState() != ritual.StateComplete {
		t.Errorf("expected StateComplete after prompt submit, got %v", runner.CurrentState())
	}

	if len(store.nodes) != 1 {
		t.Errorf("expected 1 node written, got %d", len(store.nodes))
	}

	for _, n := range store.nodes {
		if n.Body != "Today I feel focused and ready." {
			t.Errorf("unexpected node body: %q", n.Body)
		}
		if len(n.Types) == 0 || n.Types[0] != "journal" {
			t.Errorf("expected type journal, got %v", n.Types)
		}
	}
}
