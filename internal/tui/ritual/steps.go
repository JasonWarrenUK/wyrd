package ritual

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// StatusCycle is the ordered list of task status values that the 's' key
// cycles through in query_list steps.
var StatusCycle = []string{"inbox", "active", "waiting", "done", "archived"}

// EdgeTypes lists the available edge types a user can select when pressing
// 'e' in a query_list step.
var EdgeTypes = []string{
	string(types.EdgeRelated),
	string(types.EdgeBlocks),
	string(types.EdgeParent),
	string(types.EdgeWaitingOn),
	string(types.EdgePrecedes),
}

// ListItem represents a single row in a query_list step's result set.
// The runner maintains a slice of these as mutable local state.
type ListItem struct {
	// Node is the underlying node (may be nil for non-node rows).
	Node *types.Node

	// Row holds the raw query row for display purposes.
	Row map[string]interface{}

	// PendingStatus is the status value the user has cycled to but not yet
	// written to disk. Empty string means no pending change.
	PendingStatus string

	// Archived records that the user pressed 'x' to quick-archive this item.
	Archived bool

	// PendingEdge holds an edge that the user has requested to create from
	// this item but which has not yet been persisted.
	PendingEdge *PendingEdge
}

// PendingEdge is an edge the user is in the process of constructing via the
// 'e' key in a query_list step.
type PendingEdge struct {
	// EdgeType is the selected relationship type.
	EdgeType string

	// TargetID is the UUID of the target node once the user has picked one.
	TargetID string
}

// CycleStatus advances the item's status to the next value in StatusCycle.
// If PendingStatus is empty the cycle starts from the item's current status
// (read from Node.Properties["status"]), otherwise it advances from the
// pending value.
func CycleStatus(item *ListItem) {
	current := item.PendingStatus
	if current == "" && item.Node != nil {
		if v, ok := item.Node.Properties["status"]; ok {
			current = fmt.Sprintf("%v", v)
		}
	}

	next := nextStatus(current)
	item.PendingStatus = next
}

// nextStatus returns the status that follows current in StatusCycle.
// If current is not found the first status is returned.
func nextStatus(current string) string {
	for i, s := range StatusCycle {
		if s == current {
			return StatusCycle[(i+1)%len(StatusCycle)]
		}
	}
	return StatusCycle[0]
}

// QuickArchive sets the item's pending status to "archived" and marks it
// archived so the UI can dim or remove it immediately.
func QuickArchive(item *ListItem) {
	item.PendingStatus = "archived"
	item.Archived = true
}

// SetPendingEdge attaches a pending edge to the item.
func SetPendingEdge(item *ListItem, edgeType, targetID string) {
	item.PendingEdge = &PendingEdge{
		EdgeType: edgeType,
		TargetID: targetID,
	}
}

// CommitListEdits applies all pending changes from items to the store.
// It writes updated nodes and new edges. Errors are collected and returned
// together rather than halting on the first failure.
func CommitListEdits(items []*ListItem, store types.StoreFS, clock types.Clock) []error {
	var errs []error

	for _, item := range items {
		if item.Node == nil {
			continue
		}

		// Apply status change.
		if item.PendingStatus != "" {
			if item.Node.Properties == nil {
				item.Node.Properties = make(map[string]interface{})
			}
			item.Node.Properties["status"] = item.PendingStatus
			item.Node.Modified = clock.Now()

			if err := store.WriteNode(item.Node); err != nil {
				errs = append(errs, fmt.Errorf("writing node %s: %w", item.Node.ID, err))
			}
		}

		// Apply pending edge.
		if item.PendingEdge != nil && item.PendingEdge.TargetID != "" {
			edge := &types.Edge{
				ID:      uuid.New().String(),
				Type:    item.PendingEdge.EdgeType,
				From:    item.Node.ID,
				To:      item.PendingEdge.TargetID,
				Created: clock.Now(),
			}
			if err := store.WriteEdge(edge); err != nil {
				errs = append(errs, fmt.Errorf("writing edge from %s: %w", item.Node.ID, err))
			}
		}
	}

	return errs
}

// InterpolateTemplate substitutes {{field}} placeholders in tmpl with values
// from the first row of result. If result has no rows, placeholders are
// replaced with an empty string.
func InterpolateTemplate(tmpl string, result *types.QueryResult) string {
	if result == nil || len(result.Rows) == 0 {
		// Replace all {{...}} placeholders with empty string.
		return removePlaceholders(tmpl)
	}

	row := result.Rows[0]
	output := tmpl

	for key, val := range row {
		placeholder := "{{" + key + "}}"
		output = strings.ReplaceAll(output, placeholder, fmt.Sprintf("%v", val))
	}

	// Remove any remaining unresolved placeholders.
	return removePlaceholders(output)
}

// removePlaceholders strips any remaining {{...}} tokens from s.
func removePlaceholders(s string) string {
	for {
		start := strings.Index(s, "{{")
		if start == -1 {
			break
		}
		end := strings.Index(s[start:], "}}")
		if end == -1 {
			break
		}
		s = s[:start] + s[start+end+2:]
	}
	return s
}

// BuildListItems converts a QueryResult into a slice of ListItems. The
// optional nodeLoader is called for each row that has an "id" column so the
// full Node struct can be attached.
func BuildListItems(
	result *types.QueryResult,
	nodeLoader func(id string) (*types.Node, error),
) []*ListItem {
	if result == nil {
		return nil
	}

	items := make([]*ListItem, 0, len(result.Rows))
	for _, row := range result.Rows {
		item := &ListItem{Row: row}

		if nodeLoader != nil {
			if rawID, ok := row["id"]; ok {
				id := fmt.Sprintf("%v", rawID)
				if node, err := nodeLoader(id); err == nil {
					item.Node = node
				}
			}
		}

		items = append(items, item)
	}
	return items
}

// FormatListRow returns a single-line display string for a ListItem row,
// using columns to determine field order. Falls back to the node body if
// columns is empty.
func FormatListRow(item *ListItem, columns []string) string {
	if len(columns) == 0 {
		if item.Node != nil {
			return item.Node.Body
		}
		return fmt.Sprintf("%v", item.Row)
	}

	parts := make([]string, 0, len(columns))
	for _, col := range columns {
		if v, ok := item.Row[col]; ok {
			parts = append(parts, fmt.Sprintf("%v", v))
		}
	}
	return strings.Join(parts, "  ")
}

// nowDate returns today's date as a "YYYY-MM-DD" string for use in node
// properties that require a creation date.
func nowDate(clock types.Clock) string {
	return clock.Now().Format("2006-01-02")
}

// nowTime returns the current time.Time from clock.
func nowTime(clock types.Clock) time.Time {
	return clock.Now()
}
