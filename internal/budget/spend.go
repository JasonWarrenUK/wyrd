package budget

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// RecordSpend appends a spend entry to the matching budget node's spend_log.
//
// Steps:
//  1. Find the budget node whose Properties["category"] equals category via the index.
//  2. Validate that amount is positive.
//  3. Warn (but do not fail) if an entry with the same date and note already exists.
//  4. Append a new SpendEntry to the spend_log and update Modified.
//  5. Persist the node via store.WriteNode.
func RecordSpend(
	store types.StoreFS,
	index types.GraphIndex,
	category string,
	amount float64,
	note string,
	now time.Time,
) error {
	if amount <= 0 {
		return &types.ValidationError{
			Field:   "amount",
			Message: fmt.Sprintf("amount must be positive, got %g", amount),
		}
	}

	// Locate the budget node for this category.
	node, err := findBudgetNode(index, category)
	if err != nil {
		return err
	}

	// Initialise the properties map if it has somehow arrived nil.
	if node.Properties == nil {
		node.Properties = make(map[string]interface{})
	}

	entries := spendLog(node)

	// Warn on duplicate date+note (non-fatal — caller receives nil but duplicate is logged).
	dateStr := now.Format("2006-01-02")
	for _, e := range entries {
		if e.Date == dateStr && e.Note == note {
			// Duplicate detected — emit a warning to stderr but continue.
			fmt.Printf("warning: duplicate spend entry for category %q on %s with note %q\n", category, dateStr, note)
			break
		}
	}

	entries = append(entries, types.SpendEntry{
		Date:   dateStr,
		Amount: amount,
		Note:   note,
	})

	node.Properties["spend_log"] = entries
	node.Modified = now

	return store.WriteNode(node)
}

// findBudgetNode searches the index for a budget node matching the given category.
func findBudgetNode(index types.GraphIndex, category string) (*types.Node, error) {
	budgetNodes := index.NodesByType("budget")
	var available []string
	for _, n := range budgetNodes {
		if n.Properties == nil {
			continue
		}
		cat, ok := n.Properties["category"].(string)
		if !ok || cat == "" {
			continue
		}
		if cat == category {
			return n, nil
		}
		available = append(available, cat)
	}

	var hint string
	if len(available) == 0 {
		hint = "No budget categories found — create one first."
	} else {
		sort.Strings(available)
		lines := make([]string, len(available))
		for i, c := range available {
			lines[i] = "  • " + c
		}
		hint = "Available categories:\n" + strings.Join(lines, "\n")
	}

	return nil, &types.NotFoundError{Kind: "budget category", ID: category, Hint: hint}
}
