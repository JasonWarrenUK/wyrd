package stage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// hashPrefix tags every stamped ShadowOf value with the algorithm used to
// produce it. A future algorithm change bumps this prefix, so a stored hash
// from an older binary reads as "different algorithm" rather than silently
// comparing incomparable strings against a hash computed by the new one.
const hashPrefix = "sha256:"

// hashLen is the number of hex characters kept from the full digest. Sixteen
// hex characters (64 bits) is ample to detect drift from a handful of
// baked-in defaults; a full 64-character SHA-256 digest would be needlessly
// long for a value humans may end up reading in a raw kinds.jsonc/stages.jsonc.
const hashLen = 16

// DefaultKindHash returns the content hash of the baked-in default kind
// registered under name, or "" if no default kind has that name (including
// when DefaultKinds itself fails to load). Used to stamp Kind.ShadowOf when a
// form or rename cascade forks a default.
func DefaultKindHash(name string) string {
	defaults, err := DefaultKinds()
	if err != nil {
		return ""
	}
	for _, k := range defaults {
		if k.Name == name {
			// ShadowOf, ShadowReason and ShadowSource are provenance about
			// the fork, not content of the default — zero all three before
			// hashing so the hash is stable regardless of whether a caller
			// passes a struct that already carries a stamp, and so adding
			// each field doesn't retroactively change every
			// previously-stamped hash. DefaultKinds() never returns an
			// entry with these set in practice (no embedded default carries
			// one), so this is defensive rather than currently reachable —
			// see hashKindLikeProvenance in provenance_test.go for the same
			// defensive zeroing on the test side.
			k.ShadowOf = ""
			k.ShadowReason = ""
			k.ShadowSource = nil
			return hashEntry(k)
		}
	}
	return ""
}

// DefaultKind returns a pointer to the baked-in default kind registered
// under name, with its own provenance fields (ShadowOf, ShadowReason,
// ShadowSource) left zero, or nil if no default kind has that name
// (including when DefaultKinds itself fails to load). Used to snapshot
// Kind.ShadowSource when a form forks a still-unshadowed default (TD.18b) —
// the pointer returned here is safe to store directly as another entry's
// ShadowSource without further copying, since DefaultKinds() already
// returns a defensive copy per element.
func DefaultKind(name string) *types.Kind {
	defaults, err := DefaultKinds()
	if err != nil {
		return nil
	}
	for _, k := range defaults {
		if k.Name == name {
			k.ShadowOf = ""
			k.ShadowReason = ""
			k.ShadowSource = nil
			return &k
		}
	}
	return nil
}

// DefaultStageGroup mirrors DefaultKind for StageGroup.
func DefaultStageGroup(name string) *types.StageGroup {
	defaults, err := DefaultStageGroups()
	if err != nil {
		return nil
	}
	for _, g := range defaults {
		if g.Name == name {
			g.ShadowOf = ""
			g.ShadowReason = ""
			g.ShadowSource = nil
			return &g
		}
	}
	return nil
}

// DefaultStageGroupHash returns the content hash of the baked-in default
// stage group registered under name, or "" if no default group has that name
// (including when DefaultStageGroups itself fails to load). Used to stamp
// StageGroup.ShadowOf when a form or rename cascade forks a default.
func DefaultStageGroupHash(name string) string {
	defaults, err := DefaultStageGroups()
	if err != nil {
		return ""
	}
	for _, g := range defaults {
		if g.Name == name {
			g.ShadowOf = ""
			g.ShadowReason = ""
			g.ShadowSource = nil
			return hashEntry(g)
		}
	}
	return ""
}

// hashEntry serialises v deterministically and returns a truncated, prefixed
// hex digest.
//
// Determinism relies on v being a struct with no map fields: both types.Kind
// and types.StageGroup satisfy this, so json.Marshal always emits fields in
// declaration order and the same value always produces the same bytes.
//
// Caveat that matters for TD.5: the hash is over the Go struct, not the
// JSONC bytes on disk. Reformatting a default file (whitespace, comments,
// key order in hand-edited JSONC) does not change the hash — that's the
// point. But adding, removing, or renaming a field on types.Kind or
// types.StageGroup changes every hash produced by this function, for every
// default, simultaneously. TD.5 must treat a registry-wide hash mismatch
// following a schema change as "the schema moved", not "the user edited
// everything" — the two are indistinguishable from the hash alone.
func hashEntry(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		// types.Kind and types.StageGroup are both trivially marshallable
		// (plain strings and a string slice); this branch is unreachable in
		// practice but a hash function returning "" on failure is safer than
		// panicking mid-form-submit.
		return ""
	}
	sum := sha256.Sum256(data)
	full := hex.EncodeToString(sum[:])
	return hashPrefix + full[:hashLen]
}
