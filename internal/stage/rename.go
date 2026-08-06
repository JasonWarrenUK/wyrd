package stage

import (
	"fmt"
	"strings"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// RenameKind rewrites Kind on every live node holding oldName to newName.
// Nodes store their kind as a plain string, so a kind rename that only edits
// the registry entry would otherwise strand every node of that kind: they
// would fail types.KindRegistry.Lookup and be reported as Unresolvable by
// DetectOrphans, a class ApplyRemap cannot repair (see remap.go — it only
// iterates Orphans). RenameKind exists to make renaming a kind an atomic,
// whole-graph operation rather than a registry-only edit that quietly
// breaks every node referencing the old name.
//
// Archived nodes are rewritten too — archival is a status property, not a
// deletion, and an archived node holding a stale kind name is exactly as
// stranded as a live one the next time it's unarchived.
//
// Writes go through store.UpdateNode — the same path ApplyRemap and
// handleStageShift use — so the in-memory index stays live. On a per-node
// write failure, RenameKind continues rewriting the remaining nodes rather
// than aborting: a partial rename is safely re-runnable (nodes already
// renamed simply won't match oldName on a retry), whereas an abort partway
// through would leave no signal about which nodes moved and which didn't.
// The returned error, if non-nil, wraps every failure encountered.
func RenameKind(store types.StoreFS, index types.GraphIndex, oldName, newName string) (int, error) {
	written := 0
	var errs []error

	for _, node := range index.AllNodes() {
		if node.Kind != oldName {
			continue
		}
		if _, err := store.UpdateNode(node.ID, map[string]interface{}{"kind": newName}); err != nil {
			errs = append(errs, fmt.Errorf("node %s: %w", node.ID, err))
			continue
		}
		written++
	}

	if len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		return written, fmt.Errorf("rename kind: %d of %d write(s) failed: %s", len(errs), written+len(errs), strings.Join(msgs, "; "))
	}

	return written, nil
}
