package types

import "time"

// EdgeType represents the built-in and user-defined relationship types.
type EdgeType string

const (
	// EdgeBlocks indicates that the source node prevents the target from starting.
	EdgeBlocks EdgeType = "blocks"

	// EdgeParent indicates that the source node contains the target.
	EdgeParent EdgeType = "parent"

	// EdgeRelated indicates a loose association between nodes.
	EdgeRelated EdgeType = "related"

	// EdgeWaitingOn indicates that the source node is waiting for the target.
	EdgeWaitingOn EdgeType = "waiting_on"

	// EdgePrecedes indicates the source should happen before the target (soft preference).
	EdgePrecedes EdgeType = "precedes"

	// EdgeDrawsFrom indicates the source movement draws money from the target
	// budget. The movement is always From and the budget always To, so
	// GraphIndex.EdgesTo(budgetID) enumerates a budget's movements.
	EdgeDrawsFrom EdgeType = "draws_from"

	// EdgeAddsTo indicates the source movement adds money to the target
	// budget. Same direction convention as EdgeDrawsFrom.
	EdgeAddsTo EdgeType = "adds_to"
)

// EdgeDates groups the creation and last-modified timestamps for an edge
// into a single nested "date" object on disk. Kept separate from node's
// DateFields rather than shared: an edge has no due/about/schedule/start/
// snooze_until concept, and sharing DateFields would give every edge five
// meaningless nil pointers that leak into query evaluation, letting users
// write a Cypher predicate against an edge date field that can never be set.
type EdgeDates struct {
	// Created is the edge creation timestamp, auto-generated and immutable.
	Created time.Time `json:"created"`

	// Modified is updated whenever the edge is rewritten.
	Modified time.Time `json:"modified"`
}

// Edge represents a directed relationship between two nodes in the graph.
// Edges are first-class entities stored as individual JSONC files under
// /store/edges/{uuid}.jsonc and carry their own UUIDs and properties.
type Edge struct {
	// ID is a UUID v4, auto-generated and immutable.
	ID string `json:"id"`

	// Type is the relationship type (built-in or user-defined).
	Type string `json:"type"`

	// From is the UUID of the source node.
	From string `json:"from"`

	// To is the UUID of the target node.
	To string `json:"to"`

	// Date holds the edge's creation and last-modified timestamps. Written
	// on disk as a nested "date" object. The json:"-" tag prevents
	// double-serialisation; WriteEdge handles it.
	Date EdgeDates `json:"-"`

	// Properties holds optional type-specific and user-defined fields.
	// For example, a "blocks" edge may carry a "reason" string;
	// a "waiting_on" edge may carry "promised_date" and "follow_up_date".
	Properties map[string]interface{} `json:"-"`
}
