package cli

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// BudgetCreateOptions holds the parameters for the budget create command.
type BudgetCreateOptions struct {
	// Category is the budget envelope name. Required.
	Category string

	// Allocated is the amount allocated for the period. Required, must be > 0.
	Allocated float64

	// Period is the budget period (weekly, monthly, quarterly, yearly).
	// Defaults to "monthly" when empty.
	Period string

	// WarnAt is the fraction of allocation that triggers a warning (0–1).
	// Defaults to 0.8 when zero.
	WarnAt float64

	// LinkID is an optional node ID to create a "related" edge to.
	LinkID string
}

// validPeriods lists the accepted budget period values.
var validPeriods = map[string]bool{
	"weekly":    true,
	"monthly":   true,
	"quarterly": true,
	"yearly":    true,
}

// BudgetCreate creates a budget node from the given options.
// Returns the ID of the created node.
func BudgetCreate(store types.StoreFS, opts BudgetCreateOptions) (string, error) {
	if opts.Category == "" {
		return "", &types.ValidationError{Field: "category", Message: "budget category must not be empty"}
	}
	if opts.Allocated <= 0 {
		return "", &types.ValidationError{Field: "allocated", Message: "allocated amount must be greater than zero"}
	}

	// Default period.
	if opts.Period == "" {
		opts.Period = "monthly"
	}
	if !validPeriods[opts.Period] {
		return "", &types.ValidationError{
			Field:   "period",
			Message: fmt.Sprintf("invalid period %q: must be weekly, monthly, quarterly, or yearly", opts.Period),
		}
	}

	// Default warn_at.
	if opts.WarnAt == 0 {
		opts.WarnAt = 0.8
	}
	if opts.WarnAt < 0 || opts.WarnAt > 1 {
		return "", &types.ValidationError{
			Field:   "warn_at",
			Message: "warn_at must be between 0 and 1",
		}
	}

	now := time.Now()

	node := &types.Node{
		ID:       uuid.New().String(),
		Title:    opts.Category,
		Types:    []string{"budget"},
		Created:  now,
		Modified: now,
		Properties: map[string]interface{}{
			"category":  opts.Category,
			"allocated": opts.Allocated,
			"period":    opts.Period,
			"warn_at":   opts.WarnAt,
		},
	}

	if err := store.WriteNode(node); err != nil {
		return "", fmt.Errorf("writing budget node: %w", err)
	}

	if opts.LinkID != "" {
		edge := &types.Edge{
			ID:      uuid.New().String(),
			Type:    string(types.EdgeRelated),
			From:    node.ID,
			To:      opts.LinkID,
			Created: now,
		}
		if err := store.WriteEdge(edge); err != nil {
			return node.ID, fmt.Errorf("creating link edge: %w", err)
		}
	}

	return node.ID, nil
}
