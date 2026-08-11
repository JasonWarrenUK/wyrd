package stage

import (
	"sort"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// DivergedEntry describes one shadowed Kind or StageGroup whose forked
// default has since changed upstream — its stored ShadowOf no longer
// matches the current embedded default's hash under the same name.
type DivergedEntry struct {
	// Name is the shared name between the user's shadow and the default it
	// forked from.
	Name string

	// Kind is true for a diverged Kind, false for a diverged StageGroup —
	// the two share this struct shape rather than each getting their own,
	// since callers (the status-bar advisory, the overlays) treat them
	// uniformly except for which registry they came from.
	Kind bool

	// StoredHash is the ShadowOf value on disk — the hash of the default as
	// it was at fork (or last-reviewed) time.
	StoredHash string

	// CurrentHash is DefaultKindHash(Name) / DefaultStageGroupHash(Name) as
	// of this call — the hash of the default as it stands now. Always
	// non-empty for a DivergedEntry: an entry whose name no longer matches
	// any current default (e.g. detached by a rename) is not reported as
	// diverged at all — see DetectDiverged's doc comment.
	CurrentHash string
}

// DivergenceReport is the result of comparing a merged registry's shadowed
// entries against the current embedded defaults.
type DivergenceReport struct {
	// Diverged lists every shadowed entry whose default has changed since
	// the fork, sorted by Name for deterministic rendering. Entries stamped
	// ShadowTombstone or ShadowRenameFanOut are never included — see
	// DetectDiverged's doc comment for why each is excluded.
	Diverged []DivergedEntry

	// SchemaDrift is true when every shadowed, name-resolvable entry across
	// both registries mismatched simultaneously — the signature of a
	// types.Kind/types.StageGroup field being added, removed, or renamed
	// (which invalidates every stored hash at once; see hashEntry's doc
	// comment), not of independent user edits. When true, Diverged is
	// always empty: DetectDiverged treats "the schema moved" and
	// "everything diverged" as indistinguishable and refuses to report the
	// latter, since acting on it would mean asking every user with a
	// shadowed entry to review a divergence nobody caused.
	SchemaDrift bool
}

// IsEmpty reports whether the scan found nothing worth surfacing.
func (r DivergenceReport) IsEmpty() bool {
	return len(r.Diverged) == 0 && !r.SchemaDrift
}

// DetectDiverged compares every shadowed entry in kinds and groups against
// the current embedded defaults, reporting which ones have drifted since
// the fork.
//
// Four cases are deliberately excluded from Diverged, not just
// under-reported:
//
//  1. Entries with an empty ShadowOf (purely user-authored — nothing to
//     compare against) or whose Name no longer resolves to any current
//     default (detached by a rename; see kindFormPane's tombstone comment —
//     "a rename is a deliberate detachment from the default it forked
//     from"). Both are simply not shadows of anything nameable right now.
//  2. ShadowTombstone entries. A tombstone is content-identical to the
//     default it shadows by construction (see kindFormPane/stageFormPane's
//     tombstone-write sites) — it will only ever "diverge" when the
//     default itself changes, at which point it is handled like any other
//     shadow. Reporting it here would flag something the user never
//     consciously edited.
//  3. ShadowRenameFanOut entries. RenameStageGroup stamps these on default
//     kinds transitively, with a StageGroup deliberately changed to the
//     renamed group's new name — they can never again hash-equal the
//     default they forked from, but that permanent mismatch is a side
//     effect of a group rename the user drove, not drift the user should
//     be asked to review kind-by-kind. (A rename of the referencing kind's
//     own group choice, were the user to make one deliberately later,
//     would go through the ordinary edit path and get ShadowEntry instead.)
//  4. ShadowEditedAndRenamed entries. RenameStageGroup stamps these on
//     already-shadowed kinds it rewrites in place — same rationale as
//     ShadowRenameFanOut, but for a kind the user had already forked
//     themselves rather than one the rename freshly shadowed. ShadowOf
//     still records the user's original hand-edit; only the StageGroup
//     mismatch it now causes is excluded.
//
// An entry with an empty ShadowReason (stamped before this field existed)
// is treated as ShadowEdited — the original, and by far the most common,
// case this whole mechanism exists for.
func DetectDiverged(kinds *types.KindRegistry, groups *types.StageGroupRegistry) DivergenceReport {
	var candidates []DivergedEntry
	var checked, mismatched int

	if kinds != nil {
		for _, k := range kinds.All() {
			if k.ShadowOf == "" {
				continue
			}
			if k.ShadowReason == types.ShadowTombstone || k.ShadowReason == types.ShadowRenameFanOut ||
				k.ShadowReason == types.ShadowEditedAndRenamed {
				continue
			}
			current := DefaultKindHash(k.Name)
			if current == "" {
				// No current default under this name — detached by a
				// rename, or the default itself was removed upstream.
				// Neither is "diverged" in the sense this report covers.
				continue
			}
			checked++
			if k.ShadowOf != current {
				mismatched++
				candidates = append(candidates, DivergedEntry{
					Name: k.Name, Kind: true,
					StoredHash: k.ShadowOf, CurrentHash: current,
				})
			}
		}
	}

	if groups != nil {
		for _, g := range groups.All() {
			if g.ShadowOf == "" {
				continue
			}
			if g.ShadowReason == types.ShadowTombstone || g.ShadowReason == types.ShadowRenameFanOut {
				continue
			}
			current := DefaultStageGroupHash(g.Name)
			if current == "" {
				continue
			}
			checked++
			if g.ShadowOf != current {
				mismatched++
				candidates = append(candidates, DivergedEntry{
					Name: g.Name, Kind: false,
					StoredHash: g.ShadowOf, CurrentHash: current,
				})
			}
		}
	}

	// Schema-drift guard: require at least two comparable entries before
	// treating a 100% mismatch rate as "the schema moved" rather than
	// ordinary divergence — a single shadowed entry can't statistically
	// distinguish the two, and refusing to report a lone genuine divergence
	// would defeat the point of this function for the common case of a
	// user with exactly one customised kind.
	if checked >= 2 && mismatched == checked {
		return DivergenceReport{SchemaDrift: true}
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Kind != candidates[j].Kind {
			return candidates[i].Kind // kinds sort before stage groups
		}
		return candidates[i].Name < candidates[j].Name
	})

	return DivergenceReport{Diverged: candidates}
}
