package query

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustParse(t *testing.T, query string) *Statement {
	t.Helper()
	stmt, err := Parse(query)
	if err != nil {
		t.Fatalf("unexpected parse error for %q: %v", query, err)
	}
	return stmt
}

func expectParseError(t *testing.T, query string) error {
	t.Helper()
	_, err := Parse(query)
	if err == nil {
		t.Fatalf("expected parse error for %q but got none", query)
	}
	return err
}

// ---------------------------------------------------------------------------
// Basic MATCH / RETURN
// ---------------------------------------------------------------------------

func TestParse_SimpleMatchReturn(t *testing.T) {
	stmt := mustParse(t, `MATCH (n:task) RETURN n`)

	if stmt.Match == nil {
		t.Fatal("expected Match clause")
	}
	if len(stmt.Match.Patterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(stmt.Match.Patterns))
	}

	start := stmt.Match.Patterns[0].Start
	if start.Variable != "n" {
		t.Errorf("expected variable 'n', got %q", start.Variable)
	}
	if len(start.Labels) != 1 || start.Labels[0] != "task" {
		t.Errorf("expected label 'task', got %v", start.Labels)
	}

	if stmt.Return == nil || len(stmt.Return.Items) != 1 {
		t.Fatal("expected 1 return item")
	}
}

func TestParse_NoLabelNoVariable(t *testing.T) {
	stmt := mustParse(t, `MATCH (n) RETURN n`)
	start := stmt.Match.Patterns[0].Start
	if start.Variable != "n" {
		t.Errorf("expected variable 'n', got %q", start.Variable)
	}
	if len(start.Labels) != 0 {
		t.Errorf("expected no labels, got %v", start.Labels)
	}
}

func TestParse_MultipleReturnItems(t *testing.T) {
	stmt := mustParse(t, `MATCH (n:task) RETURN n.body, n.created AS created`)
	if len(stmt.Return.Items) != 2 {
		t.Fatalf("expected 2 return items, got %d", len(stmt.Return.Items))
	}

	item2 := stmt.Return.Items[1]
	if item2.Alias != "created" {
		t.Errorf("expected alias 'created', got %q", item2.Alias)
	}
}

// ---------------------------------------------------------------------------
// WHERE clause
// ---------------------------------------------------------------------------

func TestParse_WhereComparison(t *testing.T) {
	stmt := mustParse(t, `MATCH (n:task) WHERE n.status = "open" RETURN n`)
	if stmt.Where == nil {
		t.Fatal("expected WHERE clause")
	}

	be, ok := stmt.Where.Expr.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", stmt.Where.Expr)
	}
	if be.Operator != "=" {
		t.Errorf("expected '=', got %q", be.Operator)
	}
}

func TestParse_WhereAndOr(t *testing.T) {
	stmt := mustParse(t, `MATCH (n) WHERE n.a = 1 AND n.b = 2 OR n.c = 3 RETURN n`)
	if stmt.Where == nil {
		t.Fatal("expected WHERE clause")
	}
	// OR has lower precedence than AND, so the tree is (a=1 AND b=2) OR c=3
	top, ok := stmt.Where.Expr.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr at root, got %T", stmt.Where.Expr)
	}
	if top.Operator != "OR" {
		t.Errorf("expected top-level OR, got %q", top.Operator)
	}
}

func TestParse_WhereNot(t *testing.T) {
	stmt := mustParse(t, `MATCH (n) WHERE NOT n.archived = true RETURN n`)
	if stmt.Where == nil {
		t.Fatal("expected WHERE clause")
	}
	_, ok := stmt.Where.Expr.(*UnaryExpr)
	if !ok {
		t.Fatalf("expected UnaryExpr (NOT), got %T", stmt.Where.Expr)
	}
}

func TestParse_WhereIsNull(t *testing.T) {
	stmt := mustParse(t, `MATCH (n) WHERE n.due IS NULL RETURN n`)
	_, ok := stmt.Where.Expr.(*IsNullExpr)
	if !ok {
		t.Fatalf("expected IsNullExpr, got %T", stmt.Where.Expr)
	}
}

func TestParse_WhereIsNotNull(t *testing.T) {
	stmt := mustParse(t, `MATCH (n) WHERE n.due IS NOT NULL RETURN n`)
	expr, ok := stmt.Where.Expr.(*IsNullExpr)
	if !ok {
		t.Fatalf("expected IsNullExpr, got %T", stmt.Where.Expr)
	}
	if !expr.Negated {
		t.Error("expected Negated = true for IS NOT NULL")
	}
}

// ---------------------------------------------------------------------------
// Built-in variables
// ---------------------------------------------------------------------------

func TestParse_BuiltinVariableToday(t *testing.T) {
	stmt := mustParse(t, `MATCH (n) WHERE n.due < $today RETURN n`)
	be, ok := stmt.Where.Expr.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", stmt.Where.Expr)
	}
	bv, ok := be.Right.(*BuiltinVariable)
	if !ok {
		t.Fatalf("expected BuiltinVariable on right, got %T", be.Right)
	}
	if bv.Name != "today" {
		t.Errorf("expected 'today', got %q", bv.Name)
	}
	if bv.Offset != nil {
		t.Error("expected no offset")
	}
}

func TestParse_BuiltinVariableWithOffset(t *testing.T) {
	stmt := mustParse(t, `MATCH (n) WHERE n.due < $today + 7 d RETURN n`)
	be := stmt.Where.Expr.(*BinaryExpr)
	bv := be.Right.(*BuiltinVariable)
	if bv.Offset == nil {
		t.Fatal("expected offset")
	}
	if bv.Offset.Sign != "+" || bv.Offset.Amount != 7 || bv.Offset.Unit != "d" {
		t.Errorf("unexpected offset: %+v", bv.Offset)
	}
}

func TestParse_BuiltinVariableNegativeOffset(t *testing.T) {
	stmt := mustParse(t, `MATCH (n) WHERE n.created > $today - 30 d RETURN n`)
	be := stmt.Where.Expr.(*BinaryExpr)
	bv := be.Right.(*BuiltinVariable)
	if bv.Offset == nil {
		t.Fatal("expected offset")
	}
	if bv.Offset.Sign != "-" || bv.Offset.Amount != 30 || bv.Offset.Unit != "d" {
		t.Errorf("unexpected offset: %+v", bv.Offset)
	}
}

// ---------------------------------------------------------------------------
// Edge patterns
// ---------------------------------------------------------------------------

func TestParse_OutgoingEdge(t *testing.T) {
	stmt := mustParse(t, `MATCH (a)-[:blocks]->(b) RETURN a, b`)
	steps := stmt.Match.Patterns[0].Steps
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	ep := steps[0].Edge
	if ep.Direction != "out" {
		t.Errorf("expected 'out', got %q", ep.Direction)
	}
	if len(ep.Types) != 1 || ep.Types[0] != "blocks" {
		t.Errorf("expected ['blocks'], got %v", ep.Types)
	}
}

func TestParse_IncomingEdge(t *testing.T) {
	stmt := mustParse(t, `MATCH (t:task)<-[:blocks]-(b) RETURN t, b`)
	ep := stmt.Match.Patterns[0].Steps[0].Edge
	if ep.Direction != "in" {
		t.Errorf("expected 'in', got %q", ep.Direction)
	}
}

func TestParse_UndirectedEdge(t *testing.T) {
	stmt := mustParse(t, `MATCH (a)--(b) RETURN a, b`)
	ep := stmt.Match.Patterns[0].Steps[0].Edge
	if ep.Direction != "none" {
		t.Errorf("expected 'none', got %q", ep.Direction)
	}
}

func TestParse_AnyEdgeOutgoing(t *testing.T) {
	stmt := mustParse(t, `MATCH (a)-[]->(b) RETURN a, b`)
	ep := stmt.Match.Patterns[0].Steps[0].Edge
	if ep.Direction != "out" {
		t.Errorf("expected 'out', got %q", ep.Direction)
	}
	if len(ep.Types) != 0 {
		t.Errorf("expected no types, got %v", ep.Types)
	}
}

func TestParse_VarLengthPath(t *testing.T) {
	stmt := mustParse(t, `MATCH (a)-[*1..3]->(b) RETURN a, b`)
	ep := stmt.Match.Patterns[0].Steps[0].Edge
	if ep.VarLength == nil {
		t.Fatal("expected VarLength spec")
	}
	if ep.VarLength.Min != 1 || ep.VarLength.Max != 3 {
		t.Errorf("expected min=1, max=3, got %+v", ep.VarLength)
	}
}

func TestParse_VarLengthStar(t *testing.T) {
	stmt := mustParse(t, `MATCH (a)-[*]->(b) RETURN a, b`)
	ep := stmt.Match.Patterns[0].Steps[0].Edge
	if ep.VarLength == nil {
		t.Fatal("expected VarLength spec")
	}
	// Defaults: min=1, max=defaultMaxPathDepth
	if ep.VarLength.Min != 1 {
		t.Errorf("expected min=1, got %d", ep.VarLength.Min)
	}
}

// ---------------------------------------------------------------------------
// ORDER BY / LIMIT
// ---------------------------------------------------------------------------

func TestParse_OrderBy(t *testing.T) {
	stmt := mustParse(t, `MATCH (n) RETURN n ORDER BY n.created DESC`)
	if stmt.OrderBy == nil {
		t.Fatal("expected ORDER BY")
	}
	if len(stmt.OrderBy.Items) != 1 {
		t.Fatalf("expected 1 order item, got %d", len(stmt.OrderBy.Items))
	}
	if !stmt.OrderBy.Items[0].Descending {
		t.Error("expected Descending=true")
	}
}

func TestParse_Limit(t *testing.T) {
	stmt := mustParse(t, `MATCH (n) RETURN n LIMIT 10`)
	if stmt.Limit == nil {
		t.Fatal("expected LIMIT")
	}
	if stmt.Limit.Count != 10 {
		t.Errorf("expected 10, got %d", stmt.Limit.Count)
	}
}

// ---------------------------------------------------------------------------
// Aggregation functions
// ---------------------------------------------------------------------------

func TestParse_CountFunction(t *testing.T) {
	stmt := mustParse(t, `MATCH (n:task) RETURN count(n)`)
	item := stmt.Return.Items[0]
	fc, ok := item.Expr.(*FunctionCall)
	if !ok {
		t.Fatalf("expected FunctionCall, got %T", item.Expr)
	}
	if fc.Name != "count" {
		t.Errorf("expected 'count', got %q", fc.Name)
	}
}

func TestParse_CountStar(t *testing.T) {
	stmt := mustParse(t, `MATCH (n) RETURN count(*)`)
	fc := stmt.Return.Items[0].Expr.(*FunctionCall)
	if fc.Name != "count" {
		t.Errorf("expected 'count', got %q", fc.Name)
	}
	if len(fc.Args) != 1 {
		t.Fatalf("expected 1 arg for count(*), got %d", len(fc.Args))
	}
}

// ---------------------------------------------------------------------------
// Error cases
// ---------------------------------------------------------------------------

func TestParse_MutationError(t *testing.T) {
	queries := []string{
		`CREATE (n:task)`,
		`MATCH (n) SET n.status = "done"`,
		`MATCH (n) DELETE n`,
		`MERGE (n:task {id: "123"})`,
	}
	for _, q := range queries {
		err := expectParseError(t, q)
		if _, ok := err.(*MutationError); !ok {
			t.Errorf("expected MutationError for %q, got %T: %v", q, err, err)
		}
	}
}

func TestParse_UnsupportedClause(t *testing.T) {
	queries := []string{
		`MATCH (n) WITH n RETURN n`,
		`UNWIND [1,2,3] AS x RETURN x`,
	}
	for _, q := range queries {
		err := expectParseError(t, q)
		if _, ok := err.(*UnsupportedClauseError); !ok {
			t.Errorf("expected UnsupportedClauseError for %q, got %T: %v", q, err, err)
		}
	}
}

func TestParse_MissingReturn(t *testing.T) {
	err := expectParseError(t, `MATCH (n:task)`)
	if err == nil {
		t.Fatal("expected error for missing RETURN")
	}
}

func TestParse_BadEdgeType(t *testing.T) {
	err := expectParseError(t, `MATCH (n)-[:]->(m) RETURN n`)
	if err == nil {
		t.Fatal("expected error for empty edge type after ':'")
	}
}

func TestParse_UnknownBuiltin(t *testing.T) {
	err := expectParseError(t, `MATCH (n) WHERE n.d < $yesterday RETURN n`)
	if err == nil {
		t.Fatal("expected error for unknown builtin $yesterday")
	}
}
