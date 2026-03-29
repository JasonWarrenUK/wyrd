package query

// Query is the top-level AST node returned by Parse(). It wraps one or more
// Statement sub-queries joined by UNION or UNION ALL.
//
// A simple (non-UNION) query has exactly one Statement; OrderBy and Limit are
// nil on the Query itself and may be set on Statements[0] instead.
type Query struct {
	// Statements holds the individual MATCH…RETURN sub-queries in order.
	Statements []*Statement

	// UnionAll records the join mode for each junction between adjacent
	// statements. len(UnionAll) == len(Statements)-1.
	// UnionAll[i] == true means UNION ALL; false means UNION (with dedup).
	UnionAll []bool

	// OrderBy applies to the combined result when ORDER BY appears after the
	// final UNION clause. Nil for single-statement queries (ORDER BY lives on
	// the Statement instead).
	OrderBy *OrderByClause

	// Limit applies to the combined result when LIMIT appears after the final
	// UNION clause. Nil for single-statement queries.
	Limit *LimitClause
}

// Statement is the top-level AST node for a Cypher query.
// Only a single MATCH…RETURN form is supported; anything else produces an error.
type Statement struct {
	Match   *MatchClause   `parser:"@@"`
	Where   *WhereClause   `parser:"@@?"`
	Return  *ReturnClause  `parser:"@@"`
	OrderBy *OrderByClause `parser:"@@?"`
	Limit   *LimitClause   `parser:"@@?"`
}

// MatchClause holds one or more comma-separated path patterns.
type MatchClause struct {
	Patterns []*PathPattern `parser:"\"MATCH\" @@ ( \",\" @@ )*"`
}

// PathPattern describes a chain of node and edge patterns.
// Examples:
//
//	(n:task)
//	(a)-[:blocks]->(b)
//	(a)-[*1..3]->(b)
//	(a)--(b)
type PathPattern struct {
	Start *NodePattern `parser:"@@"`
	Steps []*PathStep  `parser:"@@*"`
}

// PathStep is a single edge+node pair in a path chain.
type PathStep struct {
	Edge *EdgePattern `parser:"@@"`
	Node *NodePattern `parser:"@@"`
}

// NodePattern matches a graph node.
//
//	(varName:TypeLabel)   — both optional
type NodePattern struct {
	Variable string   `parser:"\"(\" @Ident?"`
	Labels   []string `parser:"( \":\" @Ident ( \"|\" @Ident )* )?  \")\""`
}

// EdgePattern matches an edge between two nodes.
// The direction is encoded in the surrounding dashes and angle brackets.
type EdgePattern struct {
	// Direction: "none", "out", "in"
	Direction string
	// Types is the list of edge type labels (empty = any type).
	Types []string
	// VarLength is non-nil when a [*min..max] range is present.
	VarLength *VarLengthSpec
}

// VarLengthSpec holds the bounds of a variable-length path [*min..max].
type VarLengthSpec struct {
	Min int
	Max int
}

// WhereClause holds the filter expression.
type WhereClause struct {
	Expr Expression `parser:"\"WHERE\" @@"`
}

// ReturnClause holds the list of projected expressions.
type ReturnClause struct {
	Items []*ReturnItem `parser:"\"RETURN\" @@ ( \",\" @@ )*"`
}

// ReturnItem is a single projected expression with an optional alias.
type ReturnItem struct {
	Expr  Expression `parser:"@@"`
	Alias string     `parser:"( \"AS\" @Ident )?"`
}

// OrderByClause holds the ordering specification.
type OrderByClause struct {
	Items []*OrderByItem `parser:"\"ORDER\" \"BY\" @@ ( \",\" @@ )*"`
}

// OrderByItem is a single sort key with direction.
type OrderByItem struct {
	Expr       Expression `parser:"@@"`
	Descending bool       `parser:"( @\"DESC\" | \"ASC\" )?"`
}

// LimitClause holds the maximum number of rows to return.
type LimitClause struct {
	Count int `parser:"\"LIMIT\" @Int"`
}

// ---------------------------------------------------------------------------
// Expression hierarchy
// ---------------------------------------------------------------------------

// Expression is implemented by all expression node types.
// The AST is built by the hand-written expression parser (not participle)
// so these are plain Go structs, not grammar-annotated.
type Expression interface {
	exprNode()
}

// BinaryExpr is a binary operation: left OP right.
type BinaryExpr struct {
	Left     Expression
	Operator string // "AND", "OR", "=", "<>", "<", ">", "<=", ">="
	Right    Expression
}

func (*BinaryExpr) exprNode() {}

// UnaryExpr is a unary operation: NOT expr.
type UnaryExpr struct {
	Operator string // "NOT"
	Operand  Expression
}

func (*UnaryExpr) exprNode() {}

// PropertyExpr accesses a property on a variable: n.status or n.date.due
// Properties holds one or more path segments after the variable name.
// Single-segment: ["status"] for n.status
// Two-segment:    ["date", "due"] for n.date.due
type PropertyExpr struct {
	Variable   string
	Properties []string
}

// Property returns the single property name for backward-compatible callers.
// Panics if Properties has more than one segment — use Properties directly
// when chained access is possible.
func (e *PropertyExpr) Property() string {
	if len(e.Properties) == 1 {
		return e.Properties[0]
	}
	return e.Properties[0]
}

func (*PropertyExpr) exprNode() {}

// VariableExpr references a bound variable: n
type VariableExpr struct {
	Name string
}

func (*VariableExpr) exprNode() {}

// StringLiteral is a quoted string value.
type StringLiteral struct {
	Value string
}

func (*StringLiteral) exprNode() {}

// IntLiteral is an integer constant.
type IntLiteral struct {
	Value int64
}

func (*IntLiteral) exprNode() {}

// FloatLiteral is a floating-point constant.
type FloatLiteral struct {
	Value float64
}

func (*FloatLiteral) exprNode() {}

// BoolLiteral is a boolean constant (true/false).
type BoolLiteral struct {
	Value bool
}

func (*BoolLiteral) exprNode() {}

// NullLiteral represents the null value.
type NullLiteral struct{}

func (*NullLiteral) exprNode() {}

// BuiltinVariable is one of $today, $now, $week_start, $month_start,
// optionally followed by an offset: $today + 7d
type BuiltinVariable struct {
	Name   string // "today", "now", "week_start", "month_start"
	Offset *DateOffset
}

func (*BuiltinVariable) exprNode() {}

// DateOffset is an arithmetic offset applied to a built-in date variable.
type DateOffset struct {
	Sign string // "+" or "-"
	// Amount is the numeric magnitude of the offset.
	Amount int
	// Unit is the duration unit: "d" (day), "w" (week), "m" (month), "y" (year).
	Unit string
}

// FunctionCall is a call to a built-in aggregation or scalar function.
type FunctionCall struct {
	Name string
	Args []Expression
}

func (*FunctionCall) exprNode() {}

// IsNullExpr tests whether an expression evaluates to null.
type IsNullExpr struct {
	Operand  Expression
	Negated  bool // true for IS NOT NULL
}

func (*IsNullExpr) exprNode() {}
