// Package query implements a read-only Cypher subset query engine for Wyrd.
// Queries are evaluated against an in-memory GraphIndex.
package query

import "fmt"

// QueryError wraps a query execution failure with context about the query and position.
type QueryError struct {
	// Query is the offending query string.
	Query string
	// Message describes the failure.
	Message string
	// Line is the 1-based line number where the error occurred, or 0 if unknown.
	Line int
	// Column is the 1-based column number where the error occurred, or 0 if unknown.
	Column int
}

func (e *QueryError) Error() string {
	if e.Line > 0 && e.Column > 0 {
		return fmt.Sprintf("query error at line %d column %d: %s", e.Line, e.Column, e.Message)
	}
	return fmt.Sprintf("query error: %s", e.Message)
}

// UnsupportedClauseError is returned when a recognised but unimplemented clause
// keyword is encountered (e.g. WITH, OPTIONAL MATCH, UNWIND, CASE).
type UnsupportedClauseError struct {
	// Keyword is the clause keyword that is not supported.
	Keyword string
}

func (e *UnsupportedClauseError) Error() string {
	return fmt.Sprintf("clause %q is not supported in this query engine", e.Keyword)
}

// MutationError is returned when a mutation keyword is detected (CREATE, SET,
// DELETE, MERGE, REMOVE). The engine is read-only.
type MutationError struct {
	// Keyword is the mutation keyword that was detected.
	Keyword string
}

func (e *MutationError) Error() string {
	return fmt.Sprintf("mutation keyword %q is not permitted; the query engine is read-only", e.Keyword)
}

// PathDepthError is returned when a variable-length path pattern exceeds the
// configured maximum depth.
type PathDepthError struct {
	// Requested is the upper bound specified in the query (e.g. 10 for [*1..10]).
	Requested int
	// Maximum is the configured ceiling.
	Maximum int
}

func (e *PathDepthError) Error() string {
	return fmt.Sprintf(
		"variable-length path upper bound %d exceeds the maximum allowed depth of %d",
		e.Requested, e.Maximum,
	)
}
