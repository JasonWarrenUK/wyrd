# Phase 1B: Query Engine

*Agent brief. Read this, then read the referenced docs.*

## Your Mission

Build a read-only Cypher subset query engine that evaluates queries against the in-memory graph index. Every saved view, every ritual step, and every TUI data display runs a Cypher query through your engine.

## Required Reading

- [ADR-003: Cypher subset](../adr/adr-003-cypher-subset.md)
- [Saved view format](../formats/saved-view.md) (contains example queries)
- [Ritual format](../formats/ritual.md) (contains example queries in steps)
- [Budget envelope format](../formats/budget-envelope.md) (contains example queries)
- [Node schema](../schema/node.md)
- [Edge schema](../schema/edge.md)

## The Language

ADR-003 defines three layers. Implement all three.

### Layer 1: Core

`MATCH`, `WHERE`, `RETURN`, `ORDER BY`, `LIMIT`, `AS`. Property filters, boolean logic (`AND`, `OR`, `NOT`), comparison operators (`=`, `<>`, `<`, `>`, `<=`, `>=`). Date arithmetic using built-in variables.

Built-in variables: `$today` (current date), `$now` (current datetime), `$week_start` (Monday of current week), `$month_start` (first of current month). Offset arithmetic: `$today + 7d`, `$today - 30d`.

### Layer 2: Edge Traversal

Named edge traversal in both directions:
```cypher
MATCH (t:task)<-[:blocks]-(b)
MATCH (p)-[:parent]->(child)
```

Untyped edges: `MATCH (a)--(b)` (any edge type).

Variable-length paths: `MATCH (a)-[*1..3]->(b)` with a configurable ceiling (default 5 hops).

### Layer 3: Aggregation

`count()`, `sum()`, `avg()`, `min()`, `max()`, `collect()`. Implicit grouping by non-aggregated return fields.

```cypher
MATCH (t:task {status: 'ready'})
RETURN t.energy, count(t) AS total
ORDER BY total DESC
```

## Deliverables

### 1. Parser

Built with Go's `participle` library. The grammar should be approximately 200 lines. It produces an AST.

Key parse targets: MATCH patterns (node patterns with optional labels and property maps, edge patterns with optional types and direction), WHERE clauses, RETURN clauses with optional aliases, ORDER BY, LIMIT.

Error messages must be useful. "Unexpected token at position 42" is not useful. "Expected edge type after ':' but found ')' at line 1, column 42" is useful.

### 2. AST

Typed Go structs for every grammar production. The evaluator walks this AST.

Minimum types: `Query`, `MatchClause`, `NodePattern`, `EdgePattern`, `WhereClause`, `Expression`, `ReturnClause`, `ReturnItem`, `OrderByClause`, `LimitClause`, `PropertyAccess`, `FunctionCall`, `Literal`, `Variable`.

### 3. Evaluator

Takes an AST and a `GraphIndex` interface. Returns a `QueryResult` (columns + rows).

The evaluation strategy: pattern matching first (find all subgraphs matching the MATCH clause), then filter (WHERE), then project (RETURN), then aggregate, then sort, then limit.

For variable-length paths, use BFS with depth tracking. Abort at the configured ceiling.

### 4. Date Arithmetic

The variables `$today`, `$now`, `$week_start`, `$month_start` resolve against an injectable clock. Offset syntax `+Nd` and `-Nd` adds/subtracts N days. `duration('P7D')` ISO 8601 duration syntax should also work for WHERE clause comparisons.

## Interface Contract

Your engine must implement:

```go
type QueryRunner interface {
    Execute(cypher string) (QueryResult, error)
}

type QueryResult struct {
    Columns []string
    Rows    []map[string]interface{}
}
```

You depend on the `GraphIndex` interface from the store layer. The orchestrator will provide this interface definition before you begin; code against it without waiting for the store layer implementation.

## Constraints

- Read-only. No `CREATE`, `MERGE`, `DELETE`, `SET`. If someone passes a mutation query, return a clear error.
- `WITH`, `OPTIONAL MATCH`, `UNWIND`, `CASE`, string functions, and regex are explicitly deferred. Return a "not supported" error with the specific keyword identified.
- Variable-length path ceiling is configurable, default 5. Queries requesting deeper traversal get an error, not silent truncation.
- The parser must handle JSONC-style property values in MATCH patterns (strings, numbers, booleans).

## Testing Requirements

- Parser tests: valid queries parse correctly; invalid queries produce specific error messages.
- Evaluator tests against a fixture store (the shared seed data from the orchestrator guide).
- Test every query from the saved view and ritual format docs (these are real queries the system will run).
- Test edge cases: empty results, nodes with multiple types, variable-length paths that hit the ceiling, aggregation with no matching rows.
- Benchmark: 1000 nodes, 3000 edges, complex query with 2-hop traversal. Target under 50ms.

## Output Structure

```
internal/query/
  parser.go         # participle grammar + parser
  ast.go            # AST types
  evaluator.go      # Query evaluation
  functions.go      # Aggregation function implementations
  variables.go      # $today, $now, date arithmetic
  errors.go         # Typed error types
  parser_test.go
  evaluator_test.go
  functions_test.go
  variables_test.go
  testdata/         # Fixture queries and expected results
```
