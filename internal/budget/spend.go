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
//  3. Resolve the entry date: use date when non-empty (must be YYYY-MM-DD), otherwise today.
//  4. Warn (but do not fail) if an entry with the same date and note already exists.
//  5. Append a new SpendEntry to the spend_log and update Modified.
//  6. Persist the node via store.WriteNode.
//
// date is an optional explicit date string (YYYY-MM-DD). Pass empty to default to now.
//
// The returned warning is non-empty when a duplicate date+note entry already
// exists; the entry is still recorded and the caller decides how to surface it
// (the CLI prints to stderr, the TUI shows the status bar).
func RecordSpend(
	store types.StoreFS,
	index types.GraphIndex,
	category string,
	amount float64,
	note string,
	date string,
	now time.Time,
) (warning string, err error) {
	if amount <= 0 {
		return "", &types.ValidationError{
			Field:   "amount",
			Message: fmt.Sprintf("amount must be positive, got %g", amount),
		}
	}

	// Resolve the effective date.
	dateStr := date
	if dateStr == "" {
		dateStr = now.Format("2006-01-02")
	} else if _, err := time.Parse("2006-01-02", dateStr); err != nil {
		return "", &types.ValidationError{
			Field:   "date",
			Message: fmt.Sprintf("date must be YYYY-MM-DD, got %q", dateStr),
		}
	}

	// Locate the budget node for this category, then work on a clone: the
	// index returns its live pointer, and mutating that in place would
	// desynchronise the index from disk if the write below fails (and race
	// concurrent index readers). WriteNode re-reads from disk on success,
	// so the index still picks up the change.
	node, err := findBudgetNode(index, category)
	if err != nil {
		return "", err
	}
	node = node.Clone()

	// Initialise the properties map if it has somehow arrived nil.
	if node.Properties == nil {
		node.Properties = make(map[string]interface{})
	}

	entries := SpendLog(node)

	// Duplicate date+note is non-fatal: record it anyway, return the warning.
	for _, e := range entries {
		if e.Date == dateStr && e.Note == note {
			warning = fmt.Sprintf("duplicate spend entry for category %q on %s with note %q", category, dateStr, note)
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

	if err := store.WriteNode(node); err != nil {
		return "", err
	}
	return warning, nil
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
