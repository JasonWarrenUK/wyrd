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

// RenameStageGroup rewrites every kind's StageGroup reference from oldName to
// newName and writes the result to the user's kinds.jsonc in one pass.
//
// Unlike RenameKind, nodes are untouched — a node's Stage is a bare string
// with no group reference of its own; only Kind.StageGroup points at a
// group by name. So the fan-out here is across KINDS, not nodes: task-flow
// might be referenced by Task, Goblin, and Talk simultaneously (the same
// sharing DetectOrphans' OrphanKey doc comment describes), and every one of
// them needs its StageGroup field updated or the rename silently breaks
// stage resolution for every node of that kind.
//
// A referencing kind falls into one of two cases:
//   - already in the user's file: rewritten in place.
//   - a baked-in default not yet shadowed: cannot be edited (it's compiled
//     in), so a full shadowing copy is appended with StageGroup updated to
//     newName — the same override mechanic newKindFormPane's upsertKind
//     uses when editing a default kind, applied here transitively across
//     every default kind the group rename touches rather than the one kind
//     a user explicitly opened the edit form for.
//
// The caller is responsible for the group registry write itself
// (upsertStageGroup) and for the old-name tombstone when the renamed group
// is itself a baked-in default — this function only touches kinds.jsonc.
func RenameStageGroup(store types.StoreFS, oldName, newName string) (int, error) {
	userKinds, err := store.ReadKinds()
	if err != nil {
		return 0, fmt.Errorf("reading existing kinds: %w", err)
	}

	defaults, err := DefaultKinds()
	if err != nil {
		return 0, fmt.Errorf("loading built-in kind defaults: %w", err)
	}

	existing := userKinds.All()
	shadowed := make(map[string]bool, len(existing))
	rewritten := 0

	out := make([]types.Kind, len(existing))
	copy(out, existing)
	for i, k := range out {
		shadowed[k.Name] = true
		if k.StageGroup == oldName {
			// out[i] starts as a copy of the existing user entry, so
			// ShadowOf (whatever it was — set or empty) carries through
			// untouched. Only StageGroup changes.
			out[i].StageGroup = newName
			rewritten++
		}
	}

	// Built-in kinds referencing oldName that aren't already shadowed need a
	// fresh shadow copy — the embedded default can't be edited in place, and
	// leaving it unshadowed would mean it keeps pointing at oldName forever
	// (a group name that, once the caller's registry write lands, no longer
	// resolves — every node of that kind becomes Unresolvable exactly as an
	// unhandled kind rename would).
	//
	// These shadows are created transitively — entirely outside the form
	// path a user consciously drives — so ShadowOf is stamped here too.
	// Left unstamped, this fan-out would be a permanent blind spot for
	// TD.5's drift detection: these entries are indistinguishable from
	// purely user-authored kinds despite never having been hand-edited.
	for _, d := range defaults {
		if shadowed[d.Name] || d.StageGroup != oldName {
			continue
		}
		shadow := d
		shadow.StageGroup = newName
		shadow.ShadowOf = DefaultKindHash(d.Name)
		out = append(out, shadow)
		rewritten++
	}

	if err := store.WriteKinds(out); err != nil {
		return 0, err
	}

	return rewritten, nil
}
