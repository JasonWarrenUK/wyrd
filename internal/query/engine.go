package query

import (
	clog "github.com/charmbracelet/log"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// Engine implements types.QueryRunner against a types.GraphIndex.
// It is safe to call Run concurrently; all state is query-local.
// EngineOption configures optional Engine behaviour.
type EngineOption func(*Engine)

// WithLogger sets the structured logger for the query engine. When nil
// (the default), log calls are silently discarded.
func WithLogger(l *clog.Logger) EngineOption {
	return func(e *Engine) {
		e.logger = l
	}
}

type Engine struct {
	index    types.GraphIndex
	maxDepth int
	logger   *clog.Logger
}

// NewEngine creates a new Engine backed by the given graph index.
// maxDepth sets the ceiling for variable-length path traversal; pass 0 to
// use the default of 5.
func NewEngine(index types.GraphIndex, maxDepth int, opts ...EngineOption) *Engine {
	if maxDepth <= 0 {
		maxDepth = defaultMaxPathDepth
	}
	e := &Engine{index: index, maxDepth: maxDepth}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Run executes the query and returns results. Returns QueryError on failure.
// Variables like $today and $now are resolved against the provided clock.
// The engine is read-only; mutation keywords produce a MutationError.
// UNION and UNION ALL are supported: each sub-query is evaluated independently
// and results are merged (with deduplication for UNION).
func (e *Engine) Run(query string, clock types.Clock) (*types.QueryResult, error) {
	if clock == nil {
		clock = types.RealClock{}
	}

	q, err := Parse(query)
	if err != nil {
		return nil, err
	}

	if e.logger != nil {
		e.logger.Debug("running query", "query", query)
	}

	ev := newEvaluator(e.index, clock, e.maxDepth, query)
	result, err := ev.runQuery(q)
	if err != nil {
		return nil, err
	}

	if e.logger != nil {
		e.logger.Debug("query complete", "rows", len(result.Rows))
	}

	return result, nil
}
