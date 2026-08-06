package stage

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// OrphanKey identifies one (kind, stage) pair a remap decision is made
// against. Keying by kind rather than by stage group is deliberate: several
// kinds can share a group (e.g. Task, Goblin, and Talk all reference
// task-flow), so a group-level edit fans out into distinct decisions per
// kind even though the orphaned stage name is the same.
type OrphanKey struct {
	Kind  string
	Stage string
}

// Orphan describes every live node of a given kind holding a stage absent
// from that kind's currently-resolved stage group.
type Orphan struct {
	// Kind is the affected node's kind name.
	Kind string

	// Stage is the orphaned stage value nodes of this kind currently hold.
	Stage string

	// Group is the kind's current stage group — the remap target. Its
	// Stages populate the options a remap prompt offers.
	Group types.StageGroup

	// NodeIDs lists every live node holding this (Kind, Stage) pair.
	NodeIDs []string

	// Suggested is the default remap target: a case-insensitive name-match
	// against Group.Stages if one exists, else Group.Stages[0]. Empty if
	// Group has no stages (should not occur for a Validate'd group).
	Suggested string
}

// OrphanReport is the result of scanning a graph for orphaned stages.
type OrphanReport struct {
	// Orphans is stable-ordered by Kind then Stage, so a remap form built
	// from it renders deterministically across runs.
	Orphans []Orphan

	// Unresolvable lists IDs of nodes whose kind or stage group cannot be
	// resolved at all (unknown kind, or kind referencing a missing group).
	// These cannot be remapped — there is no target group to choose from —
	// so they are reported separately as a diagnostic rather than mixed
	// into Orphans.
	Unresolvable []string
}

// IsEmpty reports whether the scan found nothing to remap.
func (r OrphanReport) IsEmpty() bool {
	return len(r.Orphans) == 0
}

// NodeCount returns the total number of distinct nodes affected across all
// orphans (a node cannot appear under more than one Orphan, so this is a
// plain sum, not a deduplicated count).
func (r OrphanReport) NodeCount() int {
	n := 0
	for _, o := range r.Orphans {
		n += len(o.NodeIDs)
	}
	return n
}

// DetectOrphans scans every node in index and reports stages absent from
// their kind's resolved stage group. A node is skipped rather than reported
// when:
//   - its Kind is empty (untriaged — pre-status-lattice nodes)
//   - its Stage is empty (kind assigned but never staged)
//   - its status property is "archived"
//   - its stage is present in its kind's resolved group (healthy)
//
// A node whose kind or stage group cannot be resolved at all is recorded in
// Unresolvable instead of Orphans: there is no target group to remap into.
func DetectOrphans(index types.GraphIndex, kinds *types.KindRegistry, groups *types.StageGroupRegistry) OrphanReport {
	// nodeIDs accumulates per (kind, stage) so repeated nodes merge into one
	// Orphan entry instead of duplicating rows.
	nodeIDs := make(map[OrphanKey][]string)
	groupByKind := make(map[string]types.StageGroup)

	var unresolvable []string

	for _, node := range index.AllNodes() {
		if node.Kind == "" || node.Stage == "" {
			continue
		}
		if isArchived(node) {
			continue
		}

		group, ok := types.ResolveStageGroup(kinds, groups, node)
		if !ok {
			unresolvable = append(unresolvable, node.ID)
			continue
		}

		if group.Contains(node.Stage) {
			continue
		}

		key := OrphanKey{Kind: node.Kind, Stage: node.Stage}
		nodeIDs[key] = append(nodeIDs[key], node.ID)
		groupByKind[node.Kind] = group
	}

	orphans := make([]Orphan, 0, len(nodeIDs))
	for key, ids := range nodeIDs {
		group := groupByKind[key.Kind]
		sort.Strings(ids)
		orphans = append(orphans, Orphan{
			Kind:      key.Kind,
			Stage:     key.Stage,
			Group:     group,
			NodeIDs:   ids,
			Suggested: suggestTarget(key.Stage, group),
		})
	}

	sort.Slice(orphans, func(i, j int) bool {
		if orphans[i].Kind != orphans[j].Kind {
			return orphans[i].Kind < orphans[j].Kind
		}
		return orphans[i].Stage < orphans[j].Stage
	})

	sort.Strings(unresolvable)

	return OrphanReport{Orphans: orphans, Unresolvable: unresolvable}
}

// isArchived mirrors the status check in cli.Compact — a node is archived
// when Properties["status"] is the string "archived".
func isArchived(node *types.Node) bool {
	status, ok := node.Properties["status"]
	if !ok {
		return false
	}
	s, ok := status.(string)
	return ok && s == "archived"
}

// suggestTarget picks the default remap target for an orphaned stage: a
// case-insensitive name-match in group.Stages if one exists, else the
// group's first stage. Returns "" if the group has no stages at all (a
// defensive guard — Validate'd groups always have at least one).
func suggestTarget(orphanStage string, group types.StageGroup) string {
	for _, s := range group.Stages {
		if strings.EqualFold(s, orphanStage) {
			return s
		}
	}
	if len(group.Stages) == 0 {
		return ""
	}
	return group.Stages[0]
}

// ApplyRemap rewrites nodes per the supplied choices. choices maps an
// orphan's (Kind, Stage) identity to the target stage the user picked; an
// empty target string means "leave unchanged" and is skipped, as is a
// target equal to the orphan's current stage. Writes go through
// store.UpdateNode — the same path handleStageShift uses — so the
// in-memory index stays live.
//
// On a per-node write failure, ApplyRemap continues rewriting the remaining
// nodes rather than aborting: a partial remap is safely re-runnable, whereas
// an abort partway through would leave no signal about which nodes moved.
// The returned error, if non-nil, wraps every failure encountered.
func ApplyRemap(store types.StoreFS, report OrphanReport, choices map[OrphanKey]string, dryRun bool) (int, error) {
	written := 0
	var errs []error

	for _, orphan := range report.Orphans {
		key := OrphanKey{Kind: orphan.Kind, Stage: orphan.Stage}
		target, ok := choices[key]
		if !ok || target == "" || target == orphan.Stage {
			continue
		}

		for _, id := range orphan.NodeIDs {
			if dryRun {
				written++
				continue
			}
			if _, err := store.UpdateNode(id, map[string]interface{}{"stage": target}); err != nil {
				errs = append(errs, fmt.Errorf("node %s: %w", id, err))
				continue
			}
			written++
		}
	}

	if len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		return written, fmt.Errorf("remap: %d of %d write(s) failed: %s", len(errs), written+len(errs), strings.Join(msgs, "; "))
	}

	return written, nil
}
