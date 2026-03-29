package query

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// mustParse parses a query and returns the *Query. Fails the test on error.
func mustParse(t *testing.T, query string) *Query {
	t.Helper()
	q, err := Parse(query)
	if err != nil {
		t.Fatalf("unexpected parse error for %q: %v", query, err)
	}
	return q
}

// mustParseStmt parses a single-statement query and returns its *Statement.
// Fails if the query contains UNION or produces more than one statement.
func mustParseStmt(t *testing.T, query string) *Statement {
	t.Helper()
	q := mustParse(t, query)
	if len(q.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(q.Statements))
	}
	return q.Statements[0]
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
	stmt := mustParseStmt(t, `MATCH (n:task) RETURN n`)

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
	stmt := mustParseStmt(t, `MATCH (n) RETURN n`)
	start := stmt.Match.Patterns[0].Start
	if start.Variable != "n" {
		t.Errorf("expected variable 'n', got %q", start.Variable)
	}
	if len(start.Labels) != 0 {
		t.Errorf("expected no labels, got %v", start.Labels)
	}
}

func TestParse_MultipleReturnItems(t *testing.T) {
	stmt := mustParseStmt(t, `MATCH (n:task) RETURN n.body, n.created AS created`)
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
	stmt := mustParseStmt(t, `MATCH (n:task) WHERE n.status = "open" RETURN n`)
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
	stmt := mustParseStmt(t, `MATCH (n) WHERE n.a = 1 AND n.b = 2 OR n.c = 3 RETURN n`)
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
	stmt := mustParseStmt(t, `MATCH (n) WHERE NOT n.archived = true RETURN n`)
	if stmt.Where == nil {
		t.Fatal("expected WHERE clause")
	}
	_, ok := stmt.Where.Expr.(*UnaryExpr)
	if !ok {
		t.Fatalf("expected UnaryExpr (NOT), got %T", stmt.Where.Expr)
	}
}

func TestParse_WhereIsNull(t *testing.T) {
	stmt := mustParseStmt(t, `MATCH (n) WHERE n.due IS NULL RETURN n`)
	_, ok := stmt.Where.Expr.(*IsNullExpr)
	if !ok {
		t.Fatalf("expected IsNullExpr, got %T", stmt.Where.Expr)
	}
}

func TestParse_WhereIsNotNull(t *testing.T) {
	stmt := mustParseStmt(t, `MATCH (n) WHERE n.due IS NOT NULL RETURN n`)
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
	stmt := mustParseStmt(t, `MATCH (n) WHERE n.due < $today RETURN n`)
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
	stmt := mustParseStmt(t, `MATCH (n) WHERE n.due < $today + 7 d RETURN n`)
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
	stmt := mustParseStmt(t, `MATCH (n) WHERE n.created > $today - 30 d RETURN n`)
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
	stmt := mustParseStmt(t, `MATCH (a)-[:blocks]->(b) RETURN a, b`)
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
	stmt := mustParseStmt(t, `MATCH (t:task)<-[:blocks]-(b) RETURN t, b`)
	ep := stmt.Match.Patterns[0].Steps[0].Edge
	if ep.Direction != "in" {
		t.Errorf("expected 'in', got %q", ep.Direction)
	}
}

func TestParse_UndirectedEdge(t *testing.T) {
	stmt := mustParseStmt(t, `MATCH (a)--(b) RETURN a, b`)
	ep := stmt.Match.Patterns[0].Steps[0].Edge
	if ep.Direction != "none" {
		t.Errorf("expected 'none', got %q", ep.Direction)
	}
}

func TestParse_AnyEdgeOutgoing(t *testing.T) {
	stmt := mustParseStmt(t, `MATCH (a)-[]->(b) RETURN a, b`)
	ep := stmt.Match.Patterns[0].Steps[0].Edge
	if ep.Direction != "out" {
		t.Errorf("expected 'out', got %q", ep.Direction)
	}
	if len(ep.Types) != 0 {
		t.Errorf("expected no types, got %v", ep.Types)
	}
}

func TestParse_VarLengthPath(t *testing.T) {
	stmt := mustParseStmt(t, `MATCH (a)-[*1..3]->(b) RETURN a, b`)
	ep := stmt.Match.Patterns[0].Steps[0].Edge
	if ep.VarLength == nil {
		t.Fatal("expected VarLength spec")
	}
	if ep.VarLength.Min != 1 || ep.VarLength.Max != 3 {
		t.Errorf("expected min=1, max=3, got %+v", ep.VarLength)
	}
}

func TestParse_VarLengthStar(t *testing.T) {
	stmt := mustParseStmt(t, `MATCH (a)-[*]->(b) RETURN a, b`)
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
	stmt := mustParseStmt(t, `MATCH (n) RETURN n ORDER BY n.created DESC`)
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
	stmt := mustParseStmt(t, `MATCH (n) RETURN n LIMIT 10`)
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
	stmt := mustParseStmt(t, `MATCH (n:task) RETURN count(n)`)
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
	stmt := mustParseStmt(t, `MATCH (n) RETURN count(*)`)
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

// ---------------------------------------------------------------------------
// UNION / UNION ALL
// ---------------------------------------------------------------------------

func TestParse_Union(t *testing.T) {
	q := mustParse(t, `MATCH (n:task) RETURN n.title
UNION
MATCH (n:note) RETURN n.title`)

	if len(q.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(q.Statements))
	}
	if len(q.UnionAll) != 1 {
		t.Fatalf("expected 1 union mode, got %d", len(q.UnionAll))
	}
	if q.UnionAll[0] {
		t.Error("expected UNION (not ALL)")
	}
	// Trailing ORDER BY / LIMIT should be nil for plain UNION with no trailing clauses.
	if q.OrderBy != nil {
		t.Error("expected no compound ORDER BY")
	}
	if q.Limit != nil {
		t.Error("expected no compound LIMIT")
	}
}

func TestParse_UnionAll(t *testing.T) {
	q := mustParse(t, `MATCH (n:task) RETURN n.title
UNION ALL
MATCH (n:note) RETURN n.title`)

	if len(q.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(q.Statements))
	}
	if len(q.UnionAll) != 1 || !q.UnionAll[0] {
		t.Error("expected UNION ALL")
	}
}

func TestParse_UnionMultiple(t *testing.T) {
	q := mustParse(t, `MATCH (n:task) RETURN n.title
UNION ALL
MATCH (n:note) RETURN n.title
UNION
MATCH (n:journal) RETURN n.title`)

	if len(q.Statements) != 3 {
		t.Fatalf("expected 3 statements, got %d", len(q.Statements))
	}
	if len(q.UnionAll) != 2 {
		t.Fatalf("expected 2 union modes, got %d", len(q.UnionAll))
	}
	if !q.UnionAll[0] {
		t.Error("expected first junction to be UNION ALL")
	}
	if q.UnionAll[1] {
		t.Error("expected second junction to be UNION (not ALL)")
	}
}

func TestParse_UnionTrailingOrderByLimit(t *testing.T) {
	q := mustParse(t, `MATCH (n:task) RETURN n.title
UNION ALL
MATCH (n:note) RETURN n.title
ORDER BY n.title
LIMIT 20`)

	if len(q.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(q.Statements))
	}
	// Trailing clauses must be on the Query, not on the last sub-statement.
	if q.OrderBy == nil {
		t.Error("expected compound ORDER BY on Query")
	}
	if q.Limit == nil {
		t.Error("expected compound LIMIT on Query")
	}
	if q.Limit.Count != 20 {
		t.Errorf("expected LIMIT 20, got %d", q.Limit.Count)
	}
	// Sub-statements must not have ORDER BY / LIMIT.
	for i, stmt := range q.Statements {
		if stmt.OrderBy != nil {
			t.Errorf("statement %d should not have OrderBy", i)
		}
		if stmt.Limit != nil {
			t.Errorf("statement %d should not have Limit", i)
		}
	}
}

func TestParse_UnionSingleStatementOrderByOnStmt(t *testing.T) {
	// For a non-UNION query, ORDER BY / LIMIT stay on the statement, not Query.
	q := mustParse(t, `MATCH (n) RETURN n ORDER BY n.title LIMIT 5`)
	if len(q.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(q.Statements))
	}
	stmt := q.Statements[0]
	if stmt.OrderBy == nil {
		t.Error("expected ORDER BY on statement")
	}
	if stmt.Limit == nil || stmt.Limit.Count != 5 {
		t.Error("expected LIMIT 5 on statement")
	}
	if q.OrderBy != nil || q.Limit != nil {
		t.Error("compound Query should not have ORDER BY / LIMIT for single-statement query")
	}
}
