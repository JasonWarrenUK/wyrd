package query

import (
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// These tests extend evaluator_test.go. They use the same helpers defined
// there: buildTestGraph, newTestEngine, fixedClock, refTime, makeNode, makeEdge.

// ---------------------------------------------------------------------------
// WHERE NOT (evalUnary)
// ---------------------------------------------------------------------------

func TestEngine_WhereNot(t *testing.T) {
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	result, err := e.Run(`MATCH (n:task) WHERE NOT n.status = "done" RETURN n`, clock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// n1 (open) and n2 (open) pass; n3 (done) is filtered out.
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 rows for NOT done, got %d", len(result.Rows))
	}
}

// ---------------------------------------------------------------------------
// WHERE IS NULL / IS NOT NULL (evalIsNull)
// ---------------------------------------------------------------------------

func TestEngine_WhereIsNull(t *testing.T) {
	// n4 (project) has no priority property — should appear when filtering IS NULL.
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	result, err := e.Run(`MATCH (n) WHERE n.priority IS NULL RETURN n`, clock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// n4 (project, no priority) should be returned. Status also nil on project.
	if len(result.Rows) == 0 {
		t.Error("expected at least one row where priority IS NULL")
	}
}

func TestEngine_WhereIsNotNull(t *testing.T) {
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	result, err := e.Run(`MATCH (n:task) WHERE n.priority IS NOT NULL RETURN n`, clock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// All three task nodes have a priority property.
	if len(result.Rows) != 3 {
		t.Errorf("expected 3 rows where priority IS NOT NULL, got %d", len(result.Rows))
	}
}

// ---------------------------------------------------------------------------
// WHERE AND / OR (evalBinary)
// ---------------------------------------------------------------------------

func TestEngine_WhereAndOr(t *testing.T) {
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	// status = "open" AND priority = 1 → only n1
	result, err := e.Run(`MATCH (n:task) WHERE n.status = "open" AND n.priority = 1 RETURN n`, clock)
	if err != nil {
		t.Fatalf("AND query error: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("AND: expected 1 row, got %d", len(result.Rows))
	}

	// status = "done" OR priority = 1 → n3 (done) + n1 (priority=1, open) = 2
	result, err = e.Run(`MATCH (n:task) WHERE n.status = "done" OR n.priority = 1 RETURN n`, clock)
	if err != nil {
		t.Fatalf("OR query error: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("OR: expected 2 rows, got %d", len(result.Rows))
	}
}

// ---------------------------------------------------------------------------
// String comparison (compareOrdered)
// ---------------------------------------------------------------------------

func TestEngine_CompareStrings(t *testing.T) {
	// Add a node with a name property we can compare lexicographically.
	g := &mockGraph{
		nodes: []*types.Node{
			makeNode("s1", "Apple", []string{"item"}, map[string]interface{}{"name": "apple"}),
			makeNode("s2", "Banana", []string{"item"}, map[string]interface{}{"name": "banana"}),
			makeNode("s3", "Cherry", []string{"item"}, map[string]interface{}{"name": "cherry"}),
		},
	}
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	// Items where name > "apple" (lexicographic).
	result, err := e.Run(`MATCH (n:item) WHERE n.name > "apple" RETURN n.name`, clock)
	if err != nil {
		t.Fatalf("string comparison error: %v", err)
	}
	// "banana" and "cherry" are > "apple".
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 rows where name > 'apple', got %d", len(result.Rows))
	}
}

// ---------------------------------------------------------------------------
// Scalar functions: id(n), labels(n)
// ---------------------------------------------------------------------------

// Note: type(r) requires edge variable binding syntax ([r]) which the parser
// does not currently support. Edge type is accessible as r.type via evalProperty.

func TestEngine_ScalarFunction_Id(t *testing.T) {
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	result, err := e.Run(`MATCH (n:task) RETURN id(n)`, clock)
	if err != nil {
		t.Fatalf("id(n) query error: %v", err)
	}
	if len(result.Rows) != 3 {
		t.Errorf("expected 3 rows, got %d", len(result.Rows))
	}
	// The id column should contain non-empty strings.
	for i, row := range result.Rows {
		col := result.Columns[0]
		if row[col] == "" || row[col] == nil {
			t.Errorf("row %d: expected non-empty id, got %v", i, row[col])
		}
	}
}

func TestEngine_ScalarFunction_Labels(t *testing.T) {
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	result, err := e.Run(`MATCH (n:task) RETURN labels(n)`, clock)
	if err != nil {
		t.Fatalf("labels(n) query error: %v", err)
	}
	if len(result.Rows) != 3 {
		t.Errorf("expected 3 rows, got %d", len(result.Rows))
	}
}
