package types

// StageGroupRegistry is the resolved set of stage groups available to the app,
// keyed by name. In SL.6 it holds only the baked-in defaults. From SL.13 it
// will be the merge of baked-in defaults and any user-defined groups in
// stages.jsonc — the same pattern as KindRegistry.
type StageGroupRegistry struct {
	byName map[string]StageGroup
	order  []string // insertion order for stable iteration and display
}

// NewStageGroupRegistry builds a registry in load order. Later entries with a
// duplicate Name override earlier ones — this is the seam SL.13 uses so that
// user groups shadow baked-in defaults of the same name:
//
//	types.NewStageGroupRegistry(append(defaults, userGroups...))
//
// Passing nil or an empty slice is valid and produces an empty registry.
func NewStageGroupRegistry(groups []StageGroup) *StageGroupRegistry {
	r := &StageGroupRegistry{byName: make(map[string]StageGroup, len(groups))}
	for _, g := range groups {
		if _, seen := r.byName[g.Name]; !seen {
			r.order = append(r.order, g.Name)
		}
		r.byName[g.Name] = g
	}
	return r
}

// Lookup returns the stage group registered under name. The bool is false when
// no group is registered, mirroring the map comma-ok idiom and StageGroup.Next.
func (r *StageGroupRegistry) Lookup(name string) (StageGroup, bool) {
	g, ok := r.byName[name]
	return g, ok
}

// All returns the registered groups in load order. The slice is freshly built
// on each call so callers cannot mutate registry internals.
func (r *StageGroupRegistry) All() []StageGroup {
	out := make([]StageGroup, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.byName[name])
	}
	return out
}

// Names returns the registered group names in load order.
func (r *StageGroupRegistry) Names() []string {
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}
