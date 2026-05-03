package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// CompactResult summarises what was (or would be) moved during compaction.
type CompactResult struct {
	ArchivedNodes int
	ArchivedEdges int
	Details       []string
}

// Compact moves archived nodes and their orphaned edges to archive/ subdirectories.
// When dryRun is true the result is populated but no files are moved and the index
// is left untouched.
func (s *Store) Compact(dryRun bool) (*CompactResult, error) {
	result := &CompactResult{}

	// Collect all nodes whose status is "archived".
	archivedNodeIDs := make(map[string]bool)
	for _, node := range s.index.AllNodes() {
		if status, ok := node.Properties["status"]; ok {
			if statusStr, ok := status.(string); ok && statusStr == "archived" {
				archivedNodeIDs[node.ID] = true
			}
		}
	}

	// Identify edges that touch at least one archived node.
	archivedEdgeIDs := make(map[string]bool)
	for _, edge := range s.index.AllEdges() {
		if archivedNodeIDs[edge.From] || archivedNodeIDs[edge.To] {
			archivedEdgeIDs[edge.ID] = true
		}
	}

	if len(archivedNodeIDs) == 0 {
		return result, nil
	}

	// Move nodes.
	for id := range archivedNodeIDs {
		src := filepath.Join(s.path, "nodes", id+".jsonc")
		dst := filepath.Join(s.path, "archive", "nodes", id+".jsonc")

		if !dryRun {
			if err := os.Rename(src, dst); err != nil && !os.IsNotExist(err) {
				return nil, fmt.Errorf("archiving node %s: %w", id, err)
			}
			s.index.removeNode(id)
		}

		result.ArchivedNodes++
		result.Details = append(result.Details, fmt.Sprintf("node %s → archive/nodes/", id))
	}

	// Move orphan edges.
	for id := range archivedEdgeIDs {
		src := filepath.Join(s.path, "edges", id+".jsonc")
		dst := filepath.Join(s.path, "archive", "edges", id+".jsonc")

		if !dryRun {
			if err := os.Rename(src, dst); err != nil && !os.IsNotExist(err) {
				return nil, fmt.Errorf("archiving edge %s: %w", id, err)
			}
			s.index.removeEdge(id)
		}

		result.ArchivedEdges++
		result.Details = append(result.Details, fmt.Sprintf("edge %s → archive/edges/", id))
	}

	return result, nil
}
