package cli

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// SpendOptions holds the parameters for the spend command.
type SpendOptions struct {
	// Category is the budget envelope category.
	Category string

	// Amount is the monetary amount spent.
	Amount float64

	// Note is a human-readable description of the spend.
	Note string
}

// Spend logs a spend entry by creating a node of type "spend" in the store.
// If a budget engine is available it should be called here; for now we write
// the spend directly as a node.
func Spend(store types.StoreFS, opts SpendOptions) (string, error) {
	if opts.Category == "" {
		return "", &types.ValidationError{Field: "category", Message: "spend category must not be empty"}
	}
	if opts.Amount <= 0 {
		return "", &types.ValidationError{Field: "amount", Message: "spend amount must be greater than zero"}
	}

	now := time.Now()
	date := now.Format("2006-01-02")

	body := fmt.Sprintf("Spent %.2f on %s", opts.Amount, opts.Category)
	if opts.Note != "" {
		body = fmt.Sprintf("%s — %s", body, opts.Note)
	}

	node := &types.Node{
		ID:       uuid.New().String(),
		Body:     body,
		Types:    []string{"spend"},
		Created:  now,
		Modified: now,
		Properties: map[string]interface{}{
			"category": opts.Category,
			"amount":   opts.Amount,
			"date":     date,
			"note":     opts.Note,
		},
	}

	if err := store.WriteNode(node); err != nil {
		return "", fmt.Errorf("writing spend node: %w", err)
	}

	return node.ID, nil
}
