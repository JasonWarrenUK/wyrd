package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// Compact moves archived nodes and any edges that touch them to the archive/
// subdirectory of the store. When dryRun is true, it prints what would be
// moved without touching any files.
//
// As a side-effect, Compact also runs MigrateBudgetPeriods to normalise any
// legacy long-form period values (monthly → month, etc.) in budget nodes.
func Compact(store types.StoreFS, index types.GraphIndex, dryRun bool, out io.Writer) error {
	// Migrate legacy budget period strings first (safe to run before archival).
	if n, err := MigrateBudgetPeriods(store, index, dryRun, out); err != nil {
		return fmt.Errorf("migrating budget periods: %w", err)
	} else if n > 0 && dryRun {
		_, _ = fmt.Fprintln(out, "")
	}

	storePath := store.StorePath()

	// Collect all nodes whose status is "archived".
	archivedIDs := make(map[string]bool)
	for _, node := range index.AllNodes() {
		if status, ok := node.Properties["status"]; ok {
			if s, ok := status.(string); ok && s == "archived" {
				archivedIDs[node.ID] = true
			}
		}
	}

	if len(archivedIDs) == 0 {
		_, _ = fmt.Fprintln(out, "Nothing to compact.")
		return nil
	}

	// Identify edges that touch at least one archived node.
	archivedEdgeIDs := make(map[string]bool)
	for _, edge := range index.AllEdges() {
		if archivedIDs[edge.From] || archivedIDs[edge.To] {
			archivedEdgeIDs[edge.ID] = true
		}
	}

	if !dryRun {
		// Ensure archive subdirectories exist before moving any files.
		for _, sub := range []string{
			filepath.Join(storePath, "archive", "nodes"),
			filepath.Join(storePath, "archive", "edges"),
		} {
			if err := os.MkdirAll(sub, 0o755); err != nil {
				return fmt.Errorf("creating archive directory %s: %w", sub, err)
			}
		}
	}

	// If the caller can evict from a live in-memory index (i.e. it's backed
	// by *store.Store), collect the moved IDs so a running TUI's index
	// doesn't keep serving nodes/edges that have just been moved out of
	// nodes/ and edges/ on disk. compactor is nil when store doesn't
	// implement types.Compactor (e.g. a test double), in which case eviction
	// is simply skipped — dry runs never populate it either way.
	compactor, _ := store.(types.Compactor)

	// Move (or preview) archived nodes.
	movedNodes := 0
	var movedNodeIDs []string
	for id := range archivedIDs {
		// Find a display label: prefer Title, fall back to truncated Body.
		node, err := index.GetNode(id)
		label := id
		if err == nil {
			if node.Title != "" {
				label = node.Title
			} else if node.Body != "" {
				label = truncateBody(node.Body, 40)
			}
		}

		src := filepath.Join(storePath, "nodes", id+".jsonc")
		dst := filepath.Join(storePath, "archive", "nodes", id+".jsonc")

		if dryRun {
			_, _ = fmt.Fprintf(out, "  would move node: %s (%s)\n", id, label)
		} else {
			if err := os.Rename(src, dst); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("archiving node %s: %w", id, err)
			}
			_, _ = fmt.Fprintf(out, "  archived node: %s (%s)\n", id, label)
			movedNodeIDs = append(movedNodeIDs, id)
		}
		movedNodes++
	}

	// Move (or preview) orphan edges.
	movedEdges := 0
	var movedEdgeIDs []string
	for id := range archivedEdgeIDs {
		src := filepath.Join(storePath, "edges", id+".jsonc")
		dst := filepath.Join(storePath, "archive", "edges", id+".jsonc")

		if dryRun {
			_, _ = fmt.Fprintf(out, "  would move edge: %s\n", id)
		} else {
			if err := os.Rename(src, dst); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("archiving edge %s: %w", id, err)
			}
			_, _ = fmt.Fprintf(out, "  archived edge: %s\n", id)
			movedEdgeIDs = append(movedEdgeIDs, id)
		}
		movedEdges++
	}

	// Evict the moved entities from the live index so a running TUI (or any
	// other holder of this GraphIndex) stops serving nodes/edges that no
	// longer exist in nodes/ and edges/ on disk. removeNode already handles
	// edges incident to a removed node, so movedEdgeIDs here only needs to
	// cover edges not already implied by movedNodeIDs — RemoveFromIndex's
	// edge pass is a no-op for anything already gone.
	if !dryRun && compactor != nil {
		compactor.RemoveFromIndex(movedNodeIDs, movedEdgeIDs)
	}

	// Print summary.
	if dryRun {
		_, _ = fmt.Fprintf(out, "\nWould move %d node(s) and %d edge(s) (dry run — no files changed).\n",
			movedNodes, movedEdges)
	} else {
		_, _ = fmt.Fprintf(out, "\nMoved %d node(s) and %d edge(s) to archive/.\n",
			movedNodes, movedEdges)
	}

	return nil
}

// truncateBody shortens s to at most max runes, appending an ellipsis,
// without splitting a multi-byte rune. s is a node body and so can contain
// arbitrary multi-byte UTF-8 — byte-slicing (len/[:n]) both mismeasures
// length and can emit mojibake mid-rune. Rune count rather than display-cell
// width, matching internal/tui/views/list.go's padOrTruncate: this is a
// plain stdout summary line, not a fixed-width terminal cell budget.
func truncateBody(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max]) + "…"
}
