package types

// Kind is a user-facing node category. Each kind references a stage group (by
// name) defining the progression its nodes move through, plus a display glyph
// and colour for the TUI. Kinds are loaded from kinds.jsonc in the store's
// parent directory. From SL.5 they will be merged with baked-in defaults.
type Kind struct {
	// Name is the kind's unique identifier and display label (e.g. "Task").
	Name string `json:"name"`

	// StageGroup names the stage group this kind's nodes progress through
	// (e.g. "task-flow"). Resolved against the merged stage-group set by
	// consumers (SL.6, SL.7); an unknown reference is not validated at load
	// time because the full merged set (SL.13) does not exist yet.
	StageGroup string `json:"stage_group"`

	// Glyph is a single Unicode character shown beside nodes of this kind
	// (e.g. "◆"). Presentation-only; not validated.
	Glyph string `json:"glyph"`

	// Colour is a plain hex string (e.g. "#9b70ff"), rendered via
	// lipgloss.Color at display time. Not validated; matches the convention
	// used throughout the theme system.
	Colour string `json:"colour"`

	// ShadowOf records the content hash of the embedded default this entry was
	// forked from; empty for purely user-authored entries.
	ShadowOf string `json:"shadow_of,omitempty"`

	// ShadowReason records why ShadowOf is set — see ShadowReason's doc
	// comment. Empty on entries stamped before this field existed, or on a
	// bare hand-edit that predates ShadowRenameFanOut existing as a
	// distinction; TD.5 treats an empty reason on a shadowed entry the same
	// as ShadowEdited. Excluded from the ShadowOf content hash.
	ShadowReason ShadowReason `json:"shadow_reason,omitempty"`
}

// Validate checks the minimal structural invariants for a usable registry
// entry: non-empty Name and StageGroup reference. Glyph and Colour are
// presentation-only and are not validated, matching the no-validation
// convention for theme colours and glyphs.
func (k Kind) Validate() error {
	if k.Name == "" {
		return &ValidationError{Field: "name", Message: "kind name is required"}
	}
	if k.StageGroup == "" {
		return &ValidationError{Field: "stage_group", Message: "kind must reference a stage group"}
	}
	return nil
}

// KindRegistry is the resolved set of kinds available to the app, keyed by
// name. In SL.4 it holds only the user's kinds.jsonc. From SL.5 it will be
// the merge of baked-in defaults and the user's file.
type KindRegistry struct {
	byName map[string]Kind
	order  []string // insertion order for stable iteration and display

	// userNames records which names originated from the user-supplied slice
	// passed to NewKindRegistryFromMerge, so the registry can answer
	// IsUserDefined itself instead of callers rebuilding the same set from a
	// second read of the user's file (TD.15). Plain NewKindRegistry leaves
	// this nil: outside a defaults+user merge, "user-defined" isn't a
	// meaningful question — every entry in a bare ReadKinds() registry is
	// the user's own by construction, and nothing asks that registry
	// IsUserDefined today.
	userNames map[string]bool
}

// NewKindRegistry builds a registry in load order. Later entries with a
// duplicate Name override earlier ones — this is the seam SL.5 uses so that
// user kinds shadow baked-in defaults of the same name:
//
//	types.NewKindRegistry(append(defaults, userKinds...))
//
// Passing nil or an empty slice is valid and produces an empty registry.
// The result's IsUserDefined always reports false; use
// NewKindRegistryFromMerge when provenance tracking is needed.
func NewKindRegistry(kinds []Kind) *KindRegistry {
	return newKindRegistry(kinds, nil)
}

// NewKindRegistryFromMerge builds a registry from baked-in defaults and
// user-supplied kinds, exactly as NewKindRegistry(append(defaults, user...))
// would, but additionally records which names came from user so the
// registry can answer IsUserDefined without the caller keeping a separate
// side channel (TD.15 — this replaces the userNames map that app.go used to
// rebuild by hand at every call site that also called MergeKinds).
func NewKindRegistryFromMerge(defaults, user []Kind) *KindRegistry {
	merged := make([]Kind, 0, len(defaults)+len(user))
	merged = append(merged, defaults...)
	merged = append(merged, user...)

	userNames := make(map[string]bool, len(user))
	for _, k := range user {
		userNames[k.Name] = true
	}
	return newKindRegistry(merged, userNames)
}

func newKindRegistry(kinds []Kind, userNames map[string]bool) *KindRegistry {
	r := &KindRegistry{byName: make(map[string]Kind, len(kinds)), userNames: userNames}
	for _, k := range kinds {
		if _, seen := r.byName[k.Name]; !seen {
			r.order = append(r.order, k.Name)
		}
		r.byName[k.Name] = k
	}
	return r
}

// IsUserDefined reports whether name was present in the user-supplied slice
// at merge time — i.e. whether it is a purely custom entry or shadows a
// baked-in default, as opposed to being an untouched default. Always false
// for a registry built via plain NewKindRegistry (no merge context).
func (r *KindRegistry) IsUserDefined(name string) bool {
	if r == nil {
		return false
	}
	return r.userNames[name]
}

// Lookup returns the kind registered under name. The bool is false when no
// kind is registered, mirroring map comma-ok idiom and StageGroup.Next.
func (r *KindRegistry) Lookup(name string) (Kind, bool) {
	k, ok := r.byName[name]
	return k, ok
}

// All returns the registered kinds in load order. The slice is freshly built
// on each call so callers cannot mutate registry internals.
func (r *KindRegistry) All() []Kind {
	out := make([]Kind, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.byName[name])
	}
	return out
}

// Names returns the registered kind names in load order. Useful for NotFound
// hints and the kind-select field in SL.7/SL.10 forms.
func (r *KindRegistry) Names() []string {
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}
