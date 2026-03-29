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

// ---------------------------------------------------------------------------
// UNION / UNION ALL
// ---------------------------------------------------------------------------

func TestEngine_UnionAll(t *testing.T) {
	// task nodes (n1,n2,n3) UNION ALL project nodes (n4) → 4 rows, no dedup.
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	result, err := e.Run(`MATCH (n:task) RETURN n.id AS id
UNION ALL
MATCH (n:project) RETURN n.id AS id`, clock)
	if err != nil {
		t.Fatalf("UNION ALL error: %v", err)
	}
	if len(result.Rows) != 4 {
		t.Errorf("expected 4 rows (3 tasks + 1 project), got %d", len(result.Rows))
	}
	if len(result.Columns) != 1 || result.Columns[0] != "id" {
		t.Errorf("expected columns [id], got %v", result.Columns)
	}
}

func TestEngine_UnionDeduplicates(t *testing.T) {
	// Both sub-queries return all nodes. UNION should deduplicate.
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	result, err := e.Run(`MATCH (n:task) RETURN n.id AS id
UNION
MATCH (n:task) RETURN n.id AS id`, clock)
	if err != nil {
		t.Fatalf("UNION dedup error: %v", err)
	}
	// Duplicates removed: still 3 unique task rows.
	if len(result.Rows) != 3 {
		t.Errorf("expected 3 deduplicated rows, got %d", len(result.Rows))
	}
}

func TestEngine_UnionAllPreservesDuplicates(t *testing.T) {
	// UNION ALL must NOT deduplicate.
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	result, err := e.Run(`MATCH (n:task) RETURN n.id AS id
UNION ALL
MATCH (n:task) RETURN n.id AS id`, clock)
	if err != nil {
		t.Fatalf("UNION ALL duplicate error: %v", err)
	}
	// 3 tasks × 2 = 6 rows (no dedup).
	if len(result.Rows) != 6 {
		t.Errorf("expected 6 rows (no dedup), got %d", len(result.Rows))
	}
}

func TestEngine_UnionColumnMismatch(t *testing.T) {
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	_, err := e.Run(`MATCH (n:task) RETURN n.id AS id
UNION
MATCH (n:project) RETURN n.id AS id, n.status AS status`, clock)
	if err == nil {
		t.Fatal("expected column mismatch error")
	}
	if _, ok := err.(*UnionColumnMismatchError); !ok {
		t.Errorf("expected UnionColumnMismatchError, got %T: %v", err, err)
	}
}

func TestEngine_UnionCompoundOrderBy(t *testing.T) {
	// Tasks have status "open" or "done"; project has status "active".
	// UNION ALL, then ORDER BY status ASC.
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	result, err := e.Run(`MATCH (n:task) RETURN n.status AS status
UNION ALL
MATCH (n:project) RETURN n.status AS status
ORDER BY status`, clock)
	if err != nil {
		t.Fatalf("compound ORDER BY error: %v", err)
	}
	// 3 tasks (open,open,done) + 1 project (active) = 4 rows.
	if len(result.Rows) != 4 {
		t.Errorf("expected 4 rows, got %d", len(result.Rows))
	}
	// Ascending: "active" < "done" < "open".
	first := result.Rows[0]["status"]
	if first != "active" {
		t.Errorf("expected first row 'active' after sort, got %q", first)
	}
}

func TestEngine_UnionCompoundLimit(t *testing.T) {
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	result, err := e.Run(`MATCH (n:task) RETURN n.id AS id
UNION ALL
MATCH (n:project) RETURN n.id AS id
LIMIT 2`, clock)
	if err != nil {
		t.Fatalf("compound LIMIT error: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 rows after LIMIT 2, got %d", len(result.Rows))
	}
}

func TestEngine_UnionThreeWay(t *testing.T) {
	// Three branches: tasks, then tasks again (ALL), then projects.
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	result, err := e.Run(`MATCH (n:task) RETURN n.id AS id
UNION ALL
MATCH (n:task) RETURN n.id AS id
UNION ALL
MATCH (n:project) RETURN n.id AS id`, clock)
	if err != nil {
		t.Fatalf("three-way UNION ALL error: %v", err)
	}
	// 3 + 3 + 1 = 7 rows (no dedup because all UNION ALL).
	if len(result.Rows) != 7 {
		t.Errorf("expected 7 rows, got %d", len(result.Rows))
	}
}

func TestEngine_UnionSingleStatementUnchanged(t *testing.T) {
	// A Query wrapping a single Statement should behave identically to before.
	g := buildTestGraph()
	e := newTestEngine(g)
	clock := fixedClock(refTime)

	result, err := e.Run(`MATCH (n:task) RETURN n.status AS status ORDER BY n.status LIMIT 2`, clock)
	if err != nil {
		t.Fatalf("single-statement query error: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(result.Rows))
	}
}
