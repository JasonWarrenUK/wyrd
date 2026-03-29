package query

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// ---------------------------------------------------------------------------
// Binding / row types
// ---------------------------------------------------------------------------

// binding maps variable names to their bound values for a single result row.
// Values are either *types.Node, *types.Edge, or primitive Go values.
type binding map[string]interface{}

// clone returns a shallow copy of the binding.
func (b binding) clone() binding {
	out := make(binding, len(b))
	for k, v := range b {
		out[k] = v
	}
	return out
}

// ---------------------------------------------------------------------------
// Evaluator
// ---------------------------------------------------------------------------

// evaluator holds the execution context for a single query.
type evaluator struct {
	index    types.GraphIndex
	clock    types.Clock
	maxDepth int
	query    string
}

// newEvaluator creates a new evaluator with the given graph index, clock, and
// maximum variable-length path depth.
func newEvaluator(index types.GraphIndex, clock types.Clock, maxDepth int, query string) *evaluator {
	return &evaluator{
		index:    index,
		clock:    clock,
		maxDepth: maxDepth,
		query:    query,
	}
}

// runQuery executes a parsed Query (single-statement or UNION compound) and
// returns the merged QueryResult.
func (ev *evaluator) runQuery(q *Query) (*types.QueryResult, error) {
	// Single statement: delegate directly to run() — no overhead.
	if len(q.Statements) == 1 {
		return ev.run(q.Statements[0])
	}

	// Compound UNION: execute each sub-statement and merge results.
	var columns []string
	var allRows []map[string]interface{}

	for i, stmt := range q.Statements {
		result, err := ev.run(stmt)
		if err != nil {
			return nil, err
		}
		if i == 0 {
			columns = result.Columns
		} else if len(result.Columns) != len(columns) {
			return nil, &UnionColumnMismatchError{
				Index:    i,
				Expected: len(columns),
				Got:      len(result.Columns),
			}
		}
		allRows = append(allRows, result.Rows...)
	}

	// Deduplicate if any junction uses UNION (not UNION ALL).
	needsDedup := false
	for _, all := range q.UnionAll {
		if !all {
			needsDedup = true
			break
		}
	}
	if needsDedup {
		allRows = deduplicateRows(allRows, columns)
	}

	result := &types.QueryResult{Columns: columns, Rows: allRows}

	// Apply compound-level ORDER BY.
	if q.OrderBy != nil {
		if err := ev.evalOrderBy(result, q.OrderBy); err != nil {
			return nil, err
		}
	}

	// Apply compound-level LIMIT.
	if q.Limit != nil && q.Limit.Count < len(result.Rows) {
		result.Rows = result.Rows[:q.Limit.Count]
	}

	return result, nil
}

// deduplicateRows returns a new slice with duplicate rows removed, preserving
// first-occurrence order. Rows are compared by their projected column values.
func deduplicateRows(rows []map[string]interface{}, columns []string) []map[string]interface{} {
	seen := make(map[string]bool, len(rows))
	out := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		key := rowFingerprint(row, columns)
		if !seen[key] {
			seen[key] = true
			out = append(out, row)
		}
	}
	return out
}

// rowFingerprint produces a string key for a row based on its projected column
// values. Uses fmt.Sprintf("%v", ...) which is correct for the scalar types
// (string, int64, float64, bool, time.Time, nil) that RETURN clauses project.
func rowFingerprint(row map[string]interface{}, columns []string) string {
	parts := make([]string, len(columns))
	for i, col := range columns {
		parts[i] = fmt.Sprintf("%v", row[col])
	}
	return strings.Join(parts, "\x00")
}

// run executes the parsed statement and returns a QueryResult.
func (ev *evaluator) run(stmt *Statement) (*types.QueryResult, error) {
	// Step 1: resolve MATCH patterns into candidate bindings.
	rows, err := ev.evalMatch(stmt.Match)
	if err != nil {
		return nil, err
	}

	// Step 2: apply WHERE filter.
	if stmt.Where != nil {
		rows, err = ev.evalWhere(rows, stmt.Where)
		if err != nil {
			return nil, err
		}
	}

	// Step 3: project RETURN clause (handles aggregation).
	result, err := ev.evalReturn(rows, stmt.Return)
	if err != nil {
		return nil, err
	}

	// Step 4: apply ORDER BY.
	if stmt.OrderBy != nil {
		if err := ev.evalOrderBy(result, stmt.OrderBy); err != nil {
			return nil, err
		}
	}

	// Step 5: apply LIMIT.
	if stmt.Limit != nil {
		if stmt.Limit.Count < len(result.Rows) {
			result.Rows = result.Rows[:stmt.Limit.Count]
		}
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// MATCH evaluation
// ---------------------------------------------------------------------------

func (ev *evaluator) evalMatch(mc *MatchClause) ([]binding, error) {
	// Start with a single empty binding and extend for each pattern in sequence.
	rows := []binding{{}}

	for _, pattern := range mc.Patterns {
		extended, err := ev.evalPattern(rows, pattern)
		if err != nil {
			return nil, err
		}
		rows = extended
	}

	return rows, nil
}

// evalPattern evaluates a single path pattern against every existing binding,
// extending each one with the new variables introduced by the pattern.
func (ev *evaluator) evalPattern(rows []binding, pattern *PathPattern) ([]binding, error) {
	var out []binding

	for _, row := range rows {
		newRows, err := ev.matchPattern(row, pattern)
		if err != nil {
			return nil, err
		}
		out = append(out, newRows...)
	}

	return out, nil
}

// matchPattern finds all extensions of the current binding that satisfy the
// given path pattern, starting from the anchor node.
func (ev *evaluator) matchPattern(row binding, pattern *PathPattern) ([]binding, error) {
	// Match start node.
	startCandidates, err := ev.matchNodePattern(row, pattern.Start)
	if err != nil {
		return nil, err
	}

	rows := startCandidates

	// Walk the chain of steps.
	for _, step := range pattern.Steps {
		var extended []binding

		for _, r := range rows {
			// Determine the "current" node (from the previous step's end node variable).
			currentNode, err := ev.currentNode(r, step)
			if err != nil {
				return nil, err
			}
			if currentNode == nil {
				continue
			}

			stepRows, err := ev.matchStep(r, currentNode, step)
			if err != nil {
				return nil, err
			}
			extended = append(extended, stepRows...)
		}

		rows = extended
	}

	return rows, nil
}

// currentNode returns the node at the "end" of the previous step.
// It looks up the most recently bound node variable in the row.
func (ev *evaluator) currentNode(row binding, step *PathStep) (*types.Node, error) {
	_ = step
	// Find the last node variable bound in row (the one without an edge bound to it yet).
	// We rely on the evaluator tracking the current anchor via a sentinel key.
	if v, ok := row["__current__"]; ok {
		if n, ok := v.(*types.Node); ok {
			return n, nil
		}
	}
	return nil, nil
}

// matchNodePattern returns all bindings that satisfy a node pattern, given
// the existing binding context.
func (ev *evaluator) matchNodePattern(row binding, np *NodePattern) ([]binding, error) {
	varName := np.Variable

	// If the variable is already bound, check it matches the labels.
	if varName != "" {
		if existing, ok := row[varName]; ok {
			node, ok := existing.(*types.Node)
			if !ok {
				return nil, nil
			}
			if !nodeMatchesLabels(node, np.Labels) {
				return nil, nil
			}
			r := row.clone()
			r["__current__"] = node
			return []binding{r}, nil
		}
	}

	// Otherwise, scan all nodes (or filtered by type).
	var candidates []*types.Node
	if len(np.Labels) > 0 {
		seen := map[string]bool{}
		for _, label := range np.Labels {
			for _, n := range ev.index.NodesByType(label) {
				if !seen[n.ID] {
					seen[n.ID] = true
					candidates = append(candidates, n)
				}
			}
		}
	} else {
		candidates = ev.index.AllNodes()
	}

	var out []binding
	for _, node := range candidates {
		r := row.clone()
		if varName != "" {
			r[varName] = node
		}
		r["__current__"] = node
		out = append(out, r)
	}
	return out, nil
}

// nodeMatchesLabels returns true when the node has all required label types.
func nodeMatchesLabels(node *types.Node, labels []string) bool {
	if len(labels) == 0 {
		return true
	}
	typeSet := make(map[string]bool, len(node.Types))
	for _, t := range node.Types {
		typeSet[strings.ToLower(t)] = true
	}
	for _, l := range labels {
		if !typeSet[strings.ToLower(l)] {
			return false
		}
	}
	return true
}

// matchStep traverses the graph from currentNode along the specified edge pattern
// and returns all matching extended bindings.
func (ev *evaluator) matchStep(row binding, from *types.Node, step *PathStep) ([]binding, error) {
	ep := step.Edge
	np := step.Node

	if ep.VarLength != nil {
		return ev.matchVarLengthStep(row, from, ep, np, ep.VarLength.Min, ep.VarLength.Max)
	}

	return ev.matchSingleStep(row, from, ep, np)
}

// matchSingleStep handles a single-hop edge traversal.
func (ev *evaluator) matchSingleStep(row binding, from *types.Node, ep *EdgePattern, np *NodePattern) ([]binding, error) {
	var out []binding

	edges := ev.candidateEdges(from.ID, ep.Direction)

	for _, edge := range edges {
		if !edgeMatchesTypes(edge, ep.Types) {
			continue
		}

		// Determine the neighbour node ID.
		var neighbourID string
		switch ep.Direction {
		case "out":
			neighbourID = edge.To
		case "in":
			neighbourID = edge.From
		default:
			// Undirected: the neighbour is whichever end isn't the current node.
			if edge.From == from.ID {
				neighbourID = edge.To
			} else {
				neighbourID = edge.From
			}
		}

		neighbour, err := ev.index.GetNode(neighbourID)
		if err != nil {
			continue // node not found — skip
		}

		if !nodeMatchesLabels(neighbour, np.Labels) {
			continue
		}

		r := row.clone()
		if np.Variable != "" {
			// If the variable is already bound, check it matches.
			if existing, ok := r[np.Variable]; ok {
				existingNode, ok := existing.(*types.Node)
				if !ok || existingNode.ID != neighbour.ID {
					continue
				}
			} else {
				r[np.Variable] = neighbour
			}
		}
		r["__current__"] = neighbour
		out = append(out, r)
	}

	return out, nil
}

// matchVarLengthStep traverses up to maxHops hops, collecting all bindings
// where the terminal node satisfies the end node pattern.
func (ev *evaluator) matchVarLengthStep(
	row binding,
	from *types.Node,
	ep *EdgePattern,
	np *NodePattern,
	minHops, maxHops int,
) ([]binding, error) {
	if maxHops > ev.maxDepth {
		return nil, &PathDepthError{Requested: maxHops, Maximum: ev.maxDepth}
	}

	var out []binding
	visited := map[string]bool{from.ID: true}
	var dfs func(current *types.Node, depth int, currentRow binding) error

	dfs = func(current *types.Node, depth int, currentRow binding) error {
		if depth >= minHops {
			// This node is a candidate terminal node.
			if nodeMatchesLabels(current, np.Labels) {
				r := currentRow.clone()
				if np.Variable != "" {
					if existing, ok := r[np.Variable]; ok {
						existingNode, ok := existing.(*types.Node)
						if !ok || existingNode.ID != current.ID {
							// Variable bound to a different node — skip.
							goto descend
						}
					} else {
						r[np.Variable] = current
					}
				}
				r["__current__"] = current
				out = append(out, r)
			}
		}

	descend:
		if depth >= maxHops {
			return nil
		}

		edges := ev.candidateEdges(current.ID, ep.Direction)
		for _, edge := range edges {
			if !edgeMatchesTypes(edge, ep.Types) {
				continue
			}

			var neighbourID string
			switch ep.Direction {
			case "out":
				neighbourID = edge.To
			case "in":
				neighbourID = edge.From
			default:
				if edge.From == current.ID {
					neighbourID = edge.To
				} else {
					neighbourID = edge.From
				}
			}

			if visited[neighbourID] {
				continue
			}
			neighbour, err := ev.index.GetNode(neighbourID)
			if err != nil {
				continue
			}
			visited[neighbourID] = true
			if err := dfs(neighbour, depth+1, currentRow); err != nil {
				return err
			}
			delete(visited, neighbourID)
		}
		return nil
	}

	if err := dfs(from, 0, row); err != nil {
		return nil, err
	}
	return out, nil
}

// candidateEdges returns the relevant edges based on traversal direction.
func (ev *evaluator) candidateEdges(nodeID, direction string) []*types.Edge {
	switch direction {
	case "out":
		return ev.index.EdgesFrom(nodeID)
	case "in":
		return ev.index.EdgesTo(nodeID)
	default:
		// Undirected: union of both.
		from := ev.index.EdgesFrom(nodeID)
		to := ev.index.EdgesTo(nodeID)
		seen := make(map[string]bool, len(from)+len(to))
		var all []*types.Edge
		for _, e := range from {
			if !seen[e.ID] {
				seen[e.ID] = true
				all = append(all, e)
			}
		}
		for _, e := range to {
			if !seen[e.ID] {
				seen[e.ID] = true
				all = append(all, e)
			}
		}
		return all
	}
}

// edgeMatchesTypes returns true when the edge type is in the allowed list
// (or the list is empty, meaning any type is acceptable).
func edgeMatchesTypes(edge *types.Edge, types []string) bool {
	if len(types) == 0 {
		return true
	}
	edgeTypeLower := strings.ToLower(edge.Type)
	for _, t := range types {
		if strings.ToLower(t) == edgeTypeLower {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// WHERE evaluation
// ---------------------------------------------------------------------------

func (ev *evaluator) evalWhere(rows []binding, where *WhereClause) ([]binding, error) {
	var out []binding
	for _, row := range rows {
		v, err := ev.evalExpr(row, where.Expr)
		if err != nil {
			return nil, err
		}
		if isTruthy(v) {
			out = append(out, row)
		}
	}
	return out, nil
}

// isTruthy converts a value to a boolean for WHERE filtering.
func isTruthy(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	default:
		return true
	}
}

// ---------------------------------------------------------------------------
// Expression evaluation
// ---------------------------------------------------------------------------

// evalExpr evaluates an expression in the context of a single binding row.
func (ev *evaluator) evalExpr(row binding, expr Expression) (interface{}, error) {
	switch e := expr.(type) {
	case *StringLiteral:
		return e.Value, nil

	case *IntLiteral:
		return e.Value, nil

	case *FloatLiteral:
		return e.Value, nil

	case *BoolLiteral:
		return e.Value, nil

	case *NullLiteral:
		return nil, nil

	case *VariableExpr:
		v, ok := row[e.Name]
		if !ok {
			return nil, nil
		}
		return v, nil

	case *PropertyExpr:
		return ev.evalProperty(row, e)

	case *BuiltinVariable:
		t, err := resolveBuiltin(e, ev.clock)
		if err != nil {
			return nil, &QueryError{Query: ev.query, Message: err.Error()}
		}
		return t, nil

	case *BinaryExpr:
		return ev.evalBinary(row, e)

	case *UnaryExpr:
		return ev.evalUnary(row, e)

	case *IsNullExpr:
		return ev.evalIsNull(row, e)

	case *FunctionCall:
		// Aggregates are handled in evalReturn; scalar calls go here.
		return ev.evalScalarFunction(row, e)

	default:
		return nil, &QueryError{Query: ev.query, Message: fmt.Sprintf("unknown expression type %T", expr)}
	}
}

// evalProperty resolves variable.property (or variable.a.b) access.
func (ev *evaluator) evalProperty(row binding, e *PropertyExpr) (interface{}, error) {
	v, ok := row[e.Variable]
	if !ok {
		return nil, nil
	}

	switch obj := v.(type) {
	case *types.Node:
		return nodePropertyChain(obj, e.Properties), nil
	case *types.Edge:
		if len(e.Properties) == 1 {
			return edgeProperty(obj, e.Properties[0]), nil
		}
		return nil, nil
	default:
		return nil, nil
	}
}

// nodePropertyChain resolves a property path of one or more segments on a node.
func nodePropertyChain(node *types.Node, props []string) interface{} {
	if len(props) == 1 {
		return nodeProperty(node, props[0])
	}
	if len(props) == 2 && strings.EqualFold(props[0], "date") {
		return dateFieldProperty(&node.Date, props[1])
	}
	// Fallback: nested map in Properties.
	if node.Properties != nil {
		if m, ok := node.Properties[props[0]].(map[string]interface{}); ok {
			return m[props[1]]
		}
	}
	return nil
}

// dateFieldProperty returns a single field from a DateFields struct by name.
func dateFieldProperty(d *types.DateFields, name string) interface{} {
	switch strings.ToLower(name) {
	case "created":
		return d.Created
	case "modified":
		return d.Modified
	case "due":
		if d.Due != nil {
			return *d.Due
		}
		return nil
	case "about":
		if d.About != nil {
			return *d.About
		}
		return nil
	case "schedule":
		if d.Schedule != nil {
			return *d.Schedule
		}
		return nil
	case "start":
		if d.Start != nil {
			return *d.Start
		}
		return nil
	case "snooze_until":
		if d.SnoozeUntil != nil {
			return *d.SnoozeUntil
		}
		return nil
	}
	return nil
}

// nodeProperty extracts a named property from a node, including built-in fields.
func nodeProperty(node *types.Node, name string) interface{} {
	switch strings.ToLower(name) {
	case "id":
		return node.ID
	case "body":
		return node.Body
	case "title":
		return node.Title
	case "created":
		return node.Created
	case "modified":
		return node.Modified
	case "types":
		out := make([]interface{}, len(node.Types))
		for i, t := range node.Types {
			out[i] = t
		}
		return out
	}
	if node.Properties != nil {
		return node.Properties[name]
	}
	return nil
}

// edgeProperty extracts a named property from an edge, including built-in fields.
func edgeProperty(edge *types.Edge, name string) interface{} {
	switch strings.ToLower(name) {
	case "id":
		return edge.ID
	case "type":
		return edge.Type
	case "from":
		return edge.From
	case "to":
		return edge.To
	case "created":
		return edge.Created
	}
	if edge.Properties != nil {
		return edge.Properties[name]
	}
	return nil
}

// evalBinary evaluates a binary operator expression.
func (ev *evaluator) evalBinary(row binding, e *BinaryExpr) (interface{}, error) {
	switch e.Operator {
	case "AND":
		left, err := ev.evalExpr(row, e.Left)
		if err != nil {
			return nil, err
		}
		if !isTruthy(left) {
			return false, nil
		}
		right, err := ev.evalExpr(row, e.Right)
		if err != nil {
			return nil, err
		}
		return isTruthy(right), nil

	case "OR":
		left, err := ev.evalExpr(row, e.Left)
		if err != nil {
			return nil, err
		}
		if isTruthy(left) {
			return true, nil
		}
		right, err := ev.evalExpr(row, e.Right)
		if err != nil {
			return nil, err
		}
		return isTruthy(right), nil
	}

	left, err := ev.evalExpr(row, e.Left)
	if err != nil {
		return nil, err
	}
	right, err := ev.evalExpr(row, e.Right)
	if err != nil {
		return nil, err
	}

	return compareValues(left, right, e.Operator)
}

// compareValues compares two values using the given operator.
func compareValues(left, right interface{}, op string) (interface{}, error) {
	if left == nil || right == nil {
		switch op {
		case "=":
			return left == nil && right == nil, nil
		case "<>":
			return left != nil || right != nil, nil
		default:
			return nil, nil // null propagates
		}
	}

	switch op {
	case "=":
		return valuesEqual(left, right), nil
	case "<>":
		return !valuesEqual(left, right), nil
	case "<", ">", "<=", ">=":
		return compareOrdered(left, right, op)
	}
	return nil, fmt.Errorf("unknown operator %q", op)
}

// valuesEqual performs deep equality between two query values.
func valuesEqual(a, b interface{}) bool {
	// Time comparisons.
	at, aIsTime := a.(time.Time)
	bt, bIsTime := b.(time.Time)
	if aIsTime && bIsTime {
		return at.Equal(bt)
	}

	// Numeric equality: normalise to float64.
	af, aErr := toFloat(a)
	bf, bErr := toFloat(b)
	if aErr == nil && bErr == nil {
		return af == bf
	}

	// String equality.
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// compareOrdered evaluates ordered comparisons (<, >, <=, >=).
func compareOrdered(a, b interface{}, op string) (bool, error) {
	// Time comparisons.
	at, aIsTime := a.(time.Time)
	bt, bIsTime := b.(time.Time)
	if aIsTime && bIsTime {
		switch op {
		case "<":
			return at.Before(bt), nil
		case ">":
			return at.After(bt), nil
		case "<=":
			return !at.After(bt), nil
		case ">=":
			return !at.Before(bt), nil
		}
	}

	// Numeric comparisons.
	af, aErr := toFloat(a)
	bf, bErr := toFloat(b)
	if aErr == nil && bErr == nil {
		switch op {
		case "<":
			return af < bf, nil
		case ">":
			return af > bf, nil
		case "<=":
			return af <= bf, nil
		case ">=":
			return af >= bf, nil
		}
	}

	// String comparisons.
	as, bs := fmt.Sprintf("%v", a), fmt.Sprintf("%v", b)
	switch op {
	case "<":
		return as < bs, nil
	case ">":
		return as > bs, nil
	case "<=":
		return as <= bs, nil
	case ">=":
		return as >= bs, nil
	}

	return false, fmt.Errorf("cannot apply operator %q to %T and %T", op, a, b)
}

func (ev *evaluator) evalUnary(row binding, e *UnaryExpr) (interface{}, error) {
	operand, err := ev.evalExpr(row, e.Operand)
	if err != nil {
		return nil, err
	}
	switch e.Operator {
	case "NOT":
		return !isTruthy(operand), nil
	}
	return nil, &QueryError{Query: ev.query, Message: fmt.Sprintf("unknown unary operator %q", e.Operator)}
}

func (ev *evaluator) evalIsNull(row binding, e *IsNullExpr) (interface{}, error) {
	v, err := ev.evalExpr(row, e.Operand)
	if err != nil {
		return nil, err
	}
	isNull := v == nil
	if e.Negated {
		return !isNull, nil
	}
	return isNull, nil
}

// evalScalarFunction handles non-aggregate function calls during expression evaluation.
// Aggregate functions in a non-RETURN context are treated as pass-through.
func (ev *evaluator) evalScalarFunction(row binding, e *FunctionCall) (interface{}, error) {
	switch e.Name {
	case "id":
		if len(e.Args) != 1 {
			return nil, &QueryError{Query: ev.query, Message: "id() requires exactly one argument"}
		}
		v, err := ev.evalExpr(row, e.Args[0])
		if err != nil {
			return nil, err
		}
		switch node := v.(type) {
		case *types.Node:
			return node.ID, nil
		case *types.Edge:
			return node.ID, nil
		}
		return v, nil

	case "type":
		if len(e.Args) != 1 {
			return nil, &QueryError{Query: ev.query, Message: "type() requires exactly one argument"}
		}
		v, err := ev.evalExpr(row, e.Args[0])
		if err != nil {
			return nil, err
		}
		if edge, ok := v.(*types.Edge); ok {
			return edge.Type, nil
		}
		return nil, nil

	case "labels":
		if len(e.Args) != 1 {
			return nil, &QueryError{Query: ev.query, Message: "labels() requires exactly one argument"}
		}
		v, err := ev.evalExpr(row, e.Args[0])
		if err != nil {
			return nil, err
		}
		if node, ok := v.(*types.Node); ok {
			out := make([]interface{}, len(node.Types))
			for i, t := range node.Types {
				out[i] = t
			}
			return out, nil
		}
		return nil, nil

	case "tostring":
		if len(e.Args) != 1 {
			return nil, &QueryError{Query: ev.query, Message: "toString() requires exactly one argument"}
		}
		v, err := ev.evalExpr(row, e.Args[0])
		if err != nil {
			return nil, err
		}
		if v == nil {
			return nil, nil
		}
		return fmt.Sprintf("%v", v), nil

	case "tointeger":
		if len(e.Args) != 1 {
			return nil, &QueryError{Query: ev.query, Message: "toInteger() requires exactly one argument"}
		}
		v, err := ev.evalExpr(row, e.Args[0])
		if err != nil {
			return nil, err
		}
		if v == nil {
			return nil, nil
		}
		f, err := toFloat(v)
		if err != nil {
			return nil, nil
		}
		return int64(f), nil

	case "tofloat":
		if len(e.Args) != 1 {
			return nil, &QueryError{Query: ev.query, Message: "toFloat() requires exactly one argument"}
		}
		v, err := ev.evalExpr(row, e.Args[0])
		if err != nil {
			return nil, err
		}
		if v == nil {
			return nil, nil
		}
		return toFloat(v)

	default:
		// Aggregates should not appear in scalar context, but handle gracefully.
		if isAggregateFunction(e.Name) {
			return nil, &QueryError{Query: ev.query, Message: fmt.Sprintf("aggregate function %q is only valid in RETURN", e.Name)}
		}
		return nil, &QueryError{Query: ev.query, Message: fmt.Sprintf("unknown function %q", e.Name)}
	}
}

// ---------------------------------------------------------------------------
// RETURN evaluation (projection + aggregation)
// ---------------------------------------------------------------------------

// evalReturn projects the rows through the RETURN clause.
// If any return item contains an aggregate function, implicit grouping is applied.
func (ev *evaluator) evalReturn(rows []binding, rc *ReturnClause) (*types.QueryResult, error) {
	// Determine whether aggregation is needed.
	hasAgg := false
	for _, item := range rc.Items {
		if containsAggregate(item.Expr) {
			hasAgg = true
			break
		}
	}

	columns := make([]string, len(rc.Items))
	for i, item := range rc.Items {
		columns[i] = returnItemName(item)
	}

	result := &types.QueryResult{Columns: columns}

	if !hasAgg {
		for _, row := range rows {
			projected := make(map[string]interface{}, len(rc.Items))
			for i, item := range rc.Items {
				v, err := ev.evalExpr(row, item.Expr)
				if err != nil {
					return nil, err
				}
				projected[columns[i]] = v
			}
			result.Rows = append(result.Rows, projected)
		}
		return result, nil
	}

	// Aggregation path: group rows by the non-aggregate key expressions.
	type groupKey struct {
		key   string // serialised group key
		order int
	}

	// Identify which items are group-by keys and which are aggregates.
	type itemRole struct {
		isAgg bool
		expr  Expression
		col   string
	}
	roles := make([]itemRole, len(rc.Items))
	for i, item := range rc.Items {
		roles[i] = itemRole{
			isAgg: containsAggregate(item.Expr),
			expr:  item.Expr,
			col:   columns[i],
		}
	}

	// Group rows.
	type aggState struct {
		keyRow     binding
		aggBuffers map[string][][]interface{} // col → [][]values per aggregate in the expression
	}

	groupOrder := []string{}
	groups := map[string]*aggState{}

	for _, row := range rows {
		// Compute the group key from non-aggregate items.
		keyParts := make([]interface{}, 0)
		for _, role := range roles {
			if !role.isAgg {
				v, err := ev.evalExpr(row, role.expr)
				if err != nil {
					return nil, err
				}
				keyParts = append(keyParts, v)
			}
		}
		key := fmt.Sprintf("%v", keyParts)

		if _, ok := groups[key]; !ok {
			groups[key] = &aggState{
				keyRow:     row.clone(),
				aggBuffers: map[string][][]interface{}{},
			}
			groupOrder = append(groupOrder, key)
		}

		// Accumulate aggregate inputs.
		for _, role := range roles {
			if role.isAgg {
				vals, err := ev.collectAggregateInputs(row, role.expr)
				if err != nil {
					return nil, err
				}
				groups[key].aggBuffers[role.col] = append(groups[key].aggBuffers[role.col], vals)
			}
		}
	}

	// Produce output rows.
	for _, key := range groupOrder {
		state := groups[key]
		projected := make(map[string]interface{}, len(rc.Items))

		for _, role := range roles {
			if !role.isAgg {
				v, err := ev.evalExpr(state.keyRow, role.expr)
				if err != nil {
					return nil, err
				}
				projected[role.col] = v
			} else {
				// Flatten all accumulated value slices into a single slice.
				var all []interface{}
				for _, batch := range state.aggBuffers[role.col] {
					all = append(all, batch...)
				}
				v, err := ev.evalAggregateExpr(all, role.expr)
				if err != nil {
					return nil, err
				}
				projected[role.col] = v
			}
		}
		result.Rows = append(result.Rows, projected)
	}

	return result, nil
}

// collectAggregateInputs extracts the raw input values for all aggregate
// function calls within an expression, for a single row.
func (ev *evaluator) collectAggregateInputs(row binding, expr Expression) ([]interface{}, error) {
	fc, ok := expr.(*FunctionCall)
	if !ok {
		return nil, nil
	}
	if !isAggregateFunction(fc.Name) {
		return nil, nil
	}
	if len(fc.Args) == 0 {
		return nil, nil
	}
	if vExpr, ok := fc.Args[0].(*VariableExpr); ok && vExpr.Name == "*" {
		return []interface{}{1}, nil // count(*) counts rows
	}
	v, err := ev.evalExpr(row, fc.Args[0])
	if err != nil {
		return nil, err
	}
	return []interface{}{v}, nil
}

// evalAggregateExpr evaluates an aggregate expression given a flat slice of
// accumulated values from all rows in the group.
func (ev *evaluator) evalAggregateExpr(values []interface{}, expr Expression) (interface{}, error) {
	fc, ok := expr.(*FunctionCall)
	if !ok {
		return nil, &QueryError{Query: ev.query, Message: "expected aggregate function expression"}
	}
	return applyAggregate(fc.Name, values)
}

// containsAggregate reports whether the expression contains any aggregate function call.
func containsAggregate(expr Expression) bool {
	switch e := expr.(type) {
	case *FunctionCall:
		if isAggregateFunction(e.Name) {
			return true
		}
		for _, arg := range e.Args {
			if containsAggregate(arg) {
				return true
			}
		}
	case *BinaryExpr:
		return containsAggregate(e.Left) || containsAggregate(e.Right)
	case *UnaryExpr:
		return containsAggregate(e.Operand)
	case *IsNullExpr:
		return containsAggregate(e.Operand)
	}
	return false
}

// returnItemName derives the column name for a RETURN item.
func returnItemName(item *ReturnItem) string {
	if item.Alias != "" {
		return item.Alias
	}
	switch e := item.Expr.(type) {
	case *PropertyExpr:
		return e.Variable + "." + strings.Join(e.Properties, ".")
	case *VariableExpr:
		return e.Name
	case *FunctionCall:
		return e.Name + "()"
	case *BuiltinVariable:
		return "$" + e.Name
	}
	return "value"
}

// ---------------------------------------------------------------------------
// ORDER BY evaluation
// ---------------------------------------------------------------------------

func (ev *evaluator) evalOrderBy(result *types.QueryResult, ob *OrderByClause) error {
	var sortErr error

	sort.SliceStable(result.Rows, func(i, j int) bool {
		for _, item := range ob.Items {
			col := returnItemName(&ReturnItem{Expr: item.Expr, Alias: ""})
			a := result.Rows[i][col]
			b := result.Rows[j][col]

			less, err := isLess(a, b)
			if err != nil {
				sortErr = err
				return false
			}

			if item.Descending {
				greater, err := isLess(b, a)
				if err != nil {
					sortErr = err
					return false
				}
				if greater {
					return true
				}
				if less {
					return false
				}
			} else {
				if less {
					return true
				}
				greater, err := isLess(b, a)
				if err != nil {
					sortErr = err
					return false
				}
				if greater {
					return false
				}
			}
		}
		return false
	})

	return sortErr
}

// isLess returns true if a < b for ordering purposes.
func isLess(a, b interface{}) (bool, error) {
	if a == nil && b == nil {
		return false, nil
	}
	if a == nil {
		return true, nil // nulls sort first
	}
	if b == nil {
		return false, nil
	}

	at, aIsTime := a.(time.Time)
	bt, bIsTime := b.(time.Time)
	if aIsTime && bIsTime {
		return at.Before(bt), nil
	}

	af, aErr := toFloat(a)
	bf, bErr := toFloat(b)
	if aErr == nil && bErr == nil {
		return af < bf, nil
	}

	return fmt.Sprintf("%v", a) < fmt.Sprintf("%v", b), nil
}
