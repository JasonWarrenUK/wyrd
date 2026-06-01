package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// Compact moves archived nodes and any edges that touch them to the archive/
// subdirectory of the store. When dryRun is true, it prints what would be
// moved without touching any files.
func Compact(store types.StoreFS, index types.GraphIndex, dryRun bool, out io.Writer) error {
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
		fmt.Fprintln(out, "Nothing to compact.")
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

	// Move (or preview) archived nodes.
	movedNodes := 0
	for id := range archivedIDs {
		// Find a display label: prefer Title, fall back to truncated Body.
		node, err := index.GetNode(id)
		label := id
		if err == nil {
			if node.Title != "" {
				label = node.Title
			} else if node.Body != "" {
				body := node.Body
				if len(body) > 40 {
					body = body[:40] + "…"
				}
				label = body
			}
		}

		src := filepath.Join(storePath, "nodes", id+".jsonc")
		dst := filepath.Join(storePath, "archive", "nodes", id+".jsonc")

		if dryRun {
			fmt.Fprintf(out, "  would move node: %s (%s)\n", id, label)
		} else {
			if err := os.Rename(src, dst); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("archiving node %s: %w", id, err)
			}
			fmt.Fprintf(out, "  archived node: %s (%s)\n", id, label)
		}
		movedNodes++
	}

	// Move (or preview) orphan edges.
	movedEdges := 0
	for id := range archivedEdgeIDs {
		src := filepath.Join(storePath, "edges", id+".jsonc")
		dst := filepath.Join(storePath, "archive", "edges", id+".jsonc")

		if dryRun {
			fmt.Fprintf(out, "  would move edge: %s\n", id)
		} else {
			if err := os.Rename(src, dst); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("archiving edge %s: %w", id, err)
			}
			fmt.Fprintf(out, "  archived edge: %s\n", id)
		}
		movedEdges++
	}

	// Print summary.
	if dryRun {
		fmt.Fprintf(out, "\nWould move %d node(s) and %d edge(s) (dry run — no files changed).\n",
			movedNodes, movedEdges)
	} else {
		fmt.Fprintf(out, "\nMoved %d node(s) and %d edge(s) to archive/.\n",
			movedNodes, movedEdges)
	}

	return nil
}
