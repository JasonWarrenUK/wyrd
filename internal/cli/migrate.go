package cli

import (
	"fmt"
	"io"

	"github.com/jasonwarrenuk/wyrd/internal/budget"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// MigrateBudgetPeriods rewrites any budget node whose period property uses the
// legacy long form (weekly/monthly/quarterly/yearly) to the canonical short
// form (week/month/quarter/year). Nodes that are already correct are skipped.
//
// When dryRun is true the function reports what would change without writing
// anything. out receives a line per migrated (or would-be-migrated) node.
func MigrateBudgetPeriods(store types.StoreFS, index types.GraphIndex, dryRun bool, out io.Writer) (int, error) {
	migrated := 0

	for _, node := range index.AllNodes() {
		if len(node.Types) == 0 || node.Types[0] != "budget" {
			continue
		}

		raw, ok := node.Properties["period"]
		if !ok {
			continue
		}
		oldPeriod, ok := raw.(string)
		if !ok {
			continue
		}

		newPeriod := budget.NormalisePeriod(oldPeriod)
		if newPeriod == oldPeriod {
			// Already canonical; nothing to do.
			continue
		}

		label := node.Title
		if label == "" {
			label = node.ID
		}

		if dryRun {
			fmt.Fprintf(out, "  would migrate budget %q: %q → %q\n", label, oldPeriod, newPeriod)
		} else {
			// Re-read the live node so we don't overwrite concurrent changes.
			live, err := store.ReadNode(node.ID)
			if err != nil {
				return migrated, fmt.Errorf("reading budget node %s: %w", node.ID, err)
			}
			if live.Properties == nil {
				live.Properties = make(map[string]interface{})
			}
			live.Properties["period"] = newPeriod
			if err := store.WriteNode(live); err != nil {
				return migrated, fmt.Errorf("writing budget node %s: %w", node.ID, err)
			}
			fmt.Fprintf(out, "  migrated budget %q: %q → %q\n", label, oldPeriod, newPeriod)
		}
		migrated++
	}

	return migrated, nil
}
