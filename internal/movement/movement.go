// Package movement provides the typed accessor layer for movement nodes: a
// dated money movement linked to budgets via draws_from/adds_to edges (SP.7).
// Movement is a projection over types.Node — the canonical data lives on the
// node itself, split across a core field (Body, Date.About), a Kind/Stage
// pair, and a single Properties entry (amount).
//
// This package deliberately does not import internal/stage or any TUI
// package: kind/stage-group registration is a display concern resolved
// elsewhere, while the string constants here are the source of truth for
// what gets written to disk.
package movement

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

const (
	// PropAmount is the Properties key holding a movement's monetary value.
	// Free of collision with store.coreFields (reserved names swallowed on
	// read) and with the budget package's own keys (category, allocated,
	// period, warn_at, spend_log).
	PropAmount = "amount"

	// KindName is the registry name of the movement kind, matching the
	// baked-in kinds/movement.jsonc and the literal value stamped onto
	// Node.Kind. Title case, not the roadmap's lowercase prose.
	KindName = "Movement"

	// StageGroupName is the baked-in stage group movements progress through.
	StageGroupName = "movement-flow"

	// StageExpected is the initial stage: a forecast, not-yet-settled movement.
	StageExpected = "Expected"

	// StageCleared is the terminal stage: a settled movement.
	StageCleared = "Cleared"
)

// Movement is the typed view of a movement node. It is a projection, not a
// replacement — the canonical data still lives on types.Node.
type Movement struct {
	// ID is the underlying node's UUID. Empty for a not-yet-persisted movement.
	ID string

	// Amount is the monetary value, always positive. Direction is carried by
	// edge topology (draws_from / adds_to), never by the sign of Amount.
	Amount float64

	// Date is the transaction date, mirroring Node.Date.About.
	Date time.Time

	// Note is the human-readable description, mirroring Node.Body.
	Note string

	// Stage is the movement's stage: StageExpected or StageCleared (or empty
	// on a not-yet-persisted Movement built via New).
	Stage string
}

// New builds a Movement with the stage set to StageExpected. It does not
// touch the store; callers persist via ApplyTo plus StoreFS.WriteNode.
func New(amount float64, date time.Time, note string) Movement {
	return Movement{
		Amount: amount,
		Date:   date,
		Note:   note,
		Stage:  StageExpected,
	}
}

// FromNode projects a node into a Movement, coercing the amount property
// through the numeric-shape handling in Amount. Returns a *types.ValidationError
// with Field "kind" when the node is not a movement node, so callers walking
// a mixed edge set can distinguish "not a movement" from "malformed movement".
//
// FromNode does not call Validate: a hand-edited or legacy node missing its
// amount yields Movement{Amount: 0} with no error, so a caller summing many
// movements can skip a malformed one rather than aborting the whole
// computation. Validation is a write-path concern.
func FromNode(node *types.Node) (Movement, error) {
	if node == nil || node.Kind != KindName {
		kind := ""
		if node != nil {
			kind = node.Kind
		}
		return Movement{}, &types.ValidationError{
			Field:   "kind",
			Message: fmt.Sprintf("node kind %q is not a movement", kind),
		}
	}

	var date time.Time
	if node.Date.About != nil {
		date = *node.Date.About
	}

	return Movement{
		ID:     node.ID,
		Amount: Amount(node),
		Date:   date,
		Note:   node.Body,
		Stage:  node.Stage,
	}, nil
}

// ApplyTo writes the Movement's fields onto a copy of node and returns the
// copy; node itself is not modified. Kind is always stamped to KindName;
// Stage is set to StageExpected only when node's existing stage is empty, so
// an already-Cleared movement is not reset. Body, Date.About and the amount
// property are all stamped from the Movement's fields.
//
// Passing nil creates a fresh node (without an ID — the caller or store
// assigns that).
//
// The clone-then-mutate shape matches the established invariant at
// budget.RecordSpend: the index hands back a live pointer, and mutating it
// in place would desynchronise the index from disk if a subsequent write
// fails. Node.Clone shallow-copies Properties, which is sufficient here only
// because ApplyTo replaces the amount key wholesale rather than mutating a
// nested value.
//
// SP.6/SP.11's huh movement forms should call ApplyTo here, not form.go's
// applyKindStage — that helper solves the user-facing kind-selector/edit
// problem and depends on formPane state that doesn't exist on this path.
func (m Movement) ApplyTo(node *types.Node) *types.Node {
	var out *types.Node
	if node == nil {
		out = &types.Node{}
	} else {
		out = node.Clone()
	}

	if out.Properties == nil {
		out.Properties = make(map[string]interface{})
	}

	out.Kind = KindName
	if out.Stage == "" {
		out.Stage = StageExpected
	}
	out.Body = m.Note
	date := m.Date
	out.Date.About = &date
	out.Properties[PropAmount] = m.Amount

	return out
}

// Validate checks the invariants required of a persistable movement.
func (m Movement) Validate() error {
	if m.Amount <= 0 {
		return &types.ValidationError{
			Field:   "amount",
			Message: fmt.Sprintf("amount must be positive, got %g", m.Amount),
		}
	}
	if m.Date.IsZero() {
		return &types.ValidationError{
			Field:   "date",
			Message: "movement date is required",
		}
	}
	switch m.Stage {
	case "", StageExpected, StageCleared:
		// valid
	default:
		return &types.ValidationError{
			Field:   "stage",
			Message: fmt.Sprintf("stage must be %q or %q, got %q", StageExpected, StageCleared, m.Stage),
		}
	}
	return nil
}

// Amount reads the amount property from a node's Properties, coercing the
// numeric shapes a value can take across a JSON round-trip: a freshly
// written Go value may be int or float64, while the same value read back
// from disk is always float64, and a decoder configured with UseNumber
// yields json.Number. Returns 0 when the key is absent or the value is not
// numeric.
func Amount(node *types.Node) float64 {
	if node == nil || node.Properties == nil {
		return 0
	}
	v, ok := node.Properties[PropAmount]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return 0
		}
		return f
	case string:
		f, err := strconv.ParseFloat(n, 64)
		if err != nil {
			return 0
		}
		return f
	default:
		return 0
	}
}

// IsMovement reports whether the node carries the movement kind.
func IsMovement(node *types.Node) bool {
	return node != nil && node.Kind == KindName
}

// IsCleared reports whether the node's stage is StageCleared.
func IsCleared(node *types.Node) bool {
	return node != nil && node.Stage == StageCleared
}
