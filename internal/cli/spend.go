package cli

import (
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/budget"
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

	// Date is an optional explicit date string (YYYY-MM-DD). Empty defaults to today.
	Date string
}

// Spend logs a spend entry by delegating to budget.RecordSpend.
func Spend(store types.StoreFS, index types.GraphIndex, opts SpendOptions) error {
	if opts.Category == "" {
		return &types.ValidationError{Field: "category", Message: "spend category must not be empty"}
	}
	if opts.Amount <= 0 {
		return &types.ValidationError{Field: "amount", Message: "spend amount must be greater than zero"}
	}

	return budget.RecordSpend(store, index, opts.Category, opts.Amount, opts.Note, opts.Date, time.Now())
}
