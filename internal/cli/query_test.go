package cli_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/cli"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// mockQueryRunner is a test double for types.QueryRunner.
type mockQueryRunner struct {
	result *types.QueryResult
	err    error
}

func (m *mockQueryRunner) Run(_ string, _ types.Clock) (*types.QueryResult, error) {
	return m.result, m.err
}

// mockStore is a minimal test double for types.StoreFS that supports ReadView.
type mockStore struct {
	views map[string]*types.SavedView
}

func (m *mockStore) ReadNode(_ string) (*types.Node, error) { return nil, nil }
func (m *mockStore) WriteNode(_ *types.Node) error          { return nil }
func (m *mockStore) ReadEdge(_ string) (*types.Edge, error) { return nil, nil }
func (m *mockStore) WriteEdge(_ *types.Edge) error          { return nil }
func (m *mockStore) DeleteEdge(_ string) error              { return nil }
func (m *mockStore) ReadTemplate(_ string) (*types.Template, error) {
	return nil, &types.NotFoundError{Kind: "template", ID: ""}
}
func (m *mockStore) AllTemplates() ([]*types.Template, error) { return nil, nil }
func (m *mockStore) ReadView(name string) (*types.SavedView, error) {
	v, ok := m.views[name]
	if !ok {
		return nil, &types.NotFoundError{Kind: "view", ID: name}
	}
	return v, nil
}
func (m *mockStore) AllViews() ([]*types.SavedView, error)    { return nil, nil }
func (m *mockStore) ReadRitual(_ string) (*types.Ritual, error) {
	return nil, &types.NotFoundError{Kind: "ritual", ID: ""}
}
func (m *mockStore) AllRituals() ([]*types.Ritual, error)     { return nil, nil }
func (m *mockStore) ReadTheme(_ string) (*types.Theme, error) {
	return nil, &types.NotFoundError{Kind: "theme", ID: ""}
}
func (m *mockStore) ReadConfig() (*types.Config, error) {
	return &types.Config{MaxTraversalDepth: 5}, nil
}
func (m *mockStore) WriteConfig(_ *types.Config) error { return nil }
func (m *mockStore) StorePath() string                 { return "/tmp/mock-store" }

// ---------------------------------------------------------------------------
// RunQuery tests
// ---------------------------------------------------------------------------

func TestRunQuery_EmptyString(t *testing.T) {
	runner := &mockQueryRunner{}
	var buf bytes.Buffer

	err := cli.RunQuery(runner, types.RealClock{}, cli.QueryOptions{QueryString: ""}, &buf)
	if err == nil {
		t.Fatal("expected validation error for empty query string")
	}
	var ve *types.ValidationError
	if !asValidationError(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestRunQuery_NoResults(t *testing.T) {
	runner := &mockQueryRunner{
		result: &types.QueryResult{Columns: []string{"n"}, Rows: []map[string]interface{}{}},
	}
	var buf bytes.Buffer

	err := cli.RunQuery(runner, types.RealClock{}, cli.QueryOptions{QueryString: `MATCH (n) RETURN n`}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "(no results)") {
		t.Errorf("expected (no results) in output, got: %q", buf.String())
	}
}

func TestRunQuery_WithResults(t *testing.T) {
	runner := &mockQueryRunner{
		result: &types.QueryResult{
			Columns: []string{"title", "status"},
			Rows: []map[string]interface{}{
				{"title": "Write tests", "status": "open"},
				{"title": "Deploy prod", "status": "done"},
			},
		},
	}
	var buf bytes.Buffer

	err := cli.RunQuery(runner, types.RealClock{}, cli.QueryOptions{QueryString: `MATCH (n) RETURN n.body AS title, n.status AS status`}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "title") {
		t.Errorf("expected column header 'title' in output, got: %q", output)
	}
	if !strings.Contains(output, "Write tests") {
		t.Errorf("expected row value 'Write tests' in output, got: %q", output)
	}
}

func TestRunQuery_RunnerError(t *testing.T) {
	runner := &mockQueryRunner{
		err: fmt.Errorf("engine failure"),
	}
	var buf bytes.Buffer

	err := cli.RunQuery(runner, types.RealClock{}, cli.QueryOptions{QueryString: `MATCH (n) RETURN n`}, &buf)
	if err == nil {
		t.Fatal("expected error from runner, got nil")
	}
}

// ---------------------------------------------------------------------------
// RunView tests
// ---------------------------------------------------------------------------

func TestRunView_EmptyName(t *testing.T) {
	store := &mockStore{}
	runner := &mockQueryRunner{}
	var buf bytes.Buffer

	err := cli.RunView(store, runner, types.RealClock{}, "", &buf)
	if err == nil {
		t.Fatal("expected validation error for empty view name")
	}
	var ve *types.ValidationError
	if !asValidationError(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestRunView_ViewNotFound(t *testing.T) {
	store := &mockStore{views: map[string]*types.SavedView{}}
	runner := &mockQueryRunner{}
	var buf bytes.Buffer

	err := cli.RunView(store, runner, types.RealClock{}, "nonexistent", &buf)
	if err == nil {
		t.Fatal("expected error for missing view")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func asValidationError(err error, target **types.ValidationError) bool {
	if ve, ok := err.(*types.ValidationError); ok {
		*target = ve
		return true
	}
	return false
}
