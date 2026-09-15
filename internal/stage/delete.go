package stage

import (
	"fmt"
	"strings"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// NodesHoldingKind returns the IDs of every node whose Kind is name.
// Unlike DetectOrphans, this skips nothing: a node with no stage, or an
// archived one, is stranded by a kind deletion exactly as a live staged
// node is — the same reasoning RenameKind's doc comment gives for
// rewriting archived nodes.
func NodesHoldingKind(index types.GraphIndex, name string) []string {
	if name == "" {
		return nil
	}
	var ids []string
	for _, node := range index.AllNodes() {
		if node.Kind == name {
			ids = append(ids, node.ID)
		}
	}
	return ids
}

// KindsReferencingGroup returns the names of every kind whose StageGroup is
// name, across the merged registry so untouched defaults are included
// alongside user entries — a confirmation listing which kinds are affected
// by a group deletion needs both.
func KindsReferencingGroup(kinds *types.KindRegistry, name string) []string {
	var names []string
	for _, n := range kinds.Names() {
		k, ok := kinds.Lookup(n)
		if ok && k.StageGroup == name {
			names = append(names, k.Name)
		}
	}
	return names
}

// DeleteKind removes the user entry named name from kinds.jsonc — the
// inverse of the TUI's upsertKind. Returns a *types.NotFoundError if no user
// entry carries that name; the caller is expected to have already resolved
// name to its canonical form (case, provenance) before calling, so a miss
// here means a real inconsistency rather than a user typo.
func DeleteKind(store types.StoreFS, name string) error {
	reg, err := store.ReadKinds()
	if err != nil {
		return fmt.Errorf("reading existing kinds: %w", err)
	}

	existing := reg.All()
	out := make([]types.Kind, 0, len(existing))
	found := false
	for _, k := range existing {
		if k.Name == name {
			found = true
			continue
		}
		out = append(out, k)
	}
	if !found {
		return &types.NotFoundError{Kind: "kind", ID: name}
	}

	return store.WriteKinds(out)
}

// DeleteStageGroup is DeleteKind's twin for stages.jsonc.
func DeleteStageGroup(store types.StoreFS, name string) error {
	reg, err := store.ReadStages()
	if err != nil {
		return fmt.Errorf("reading existing stage groups: %w", err)
	}

	existing := reg.All()
	out := make([]types.StageGroup, 0, len(existing))
	found := false
	for _, g := range existing {
		if g.Name == name {
			found = true
			continue
		}
		out = append(out, g)
	}
	if !found {
		return &types.NotFoundError{Kind: "stage group", ID: name}
	}

	return store.WriteStages(out)
}

// ReassignKind rewrites Kind on every node holding fromKind to toKind.
// Modelled directly on RenameKind: writes via store.UpdateNode so the
// in-memory index stays live, includes archived nodes, and continues past a
// per-node failure (partial progress is safely re-runnable) wrapping every
// error.
//
// Unlike RenameKind, the target is a different, already-existing kind whose
// stage group may not contain the moved nodes' current stages — those nodes
// become Orphans rather than Unresolvable, which the SL.14 remap form can
// repair.
func ReassignKind(store types.StoreFS, index types.GraphIndex, fromKind, toKind string) (int, error) {
	written := 0
	var errs []error

	for _, node := range index.AllNodes() {
		if node.Kind != fromKind {
			continue
		}
		if _, err := store.UpdateNode(node.ID, map[string]interface{}{"kind": toKind}); err != nil {
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
		return written, fmt.Errorf("reassign kind: %d of %d write(s) failed: %s", len(errs), written+len(errs), strings.Join(msgs, "; "))
	}

	return written, nil
}
