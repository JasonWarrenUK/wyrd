package query

import (
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// Engine implements types.QueryRunner against a types.GraphIndex.
// It is safe to call Run concurrently; all state is query-local.
type Engine struct {
	index    types.GraphIndex
	maxDepth int
}

// NewEngine creates a new Engine backed by the given graph index.
// maxDepth sets the ceiling for variable-length path traversal; pass 0 to
// use the default of 5.
func NewEngine(index types.GraphIndex, maxDepth int) *Engine {
	if maxDepth <= 0 {
		maxDepth = defaultMaxPathDepth
	}
	return &Engine{index: index, maxDepth: maxDepth}
}

// Run executes the query and returns results. Returns QueryError on failure.
// Variables like $today and $now are resolved against the provided clock.
// The engine is read-only; mutation keywords produce a MutationError.
func (e *Engine) Run(query string, clock types.Clock) (*types.QueryResult, error) {
	if clock == nil {
		clock = types.RealClock{}
	}

	stmt, err := Parse(query)
	if err != nil {
		return nil, err
	}

	ev := newEvaluator(e.index, clock, e.maxDepth, query)
	return ev.run(stmt)
}
