package stage

import "github.com/jasonwarrenuk/wyrd/internal/types"

// MergeStageGroups builds a StageGroupRegistry from baked-in defaults and
// user-supplied groups. User groups are appended after defaults so that a user
// group with the same Name shadows the default — the last-wins-by-name seam
// documented in types.NewStageGroupRegistry. Neither input slice is mutated.
//
// Pass nil for user when no stages.jsonc exists yet; SL.13 will supply user
// groups as the second argument once the reader is implemented.
func MergeStageGroups(defaults, user []types.StageGroup) *types.StageGroupRegistry {
	merged := make([]types.StageGroup, 0, len(defaults)+len(user))
	merged = append(merged, defaults...)
	merged = append(merged, user...)
	return types.NewStageGroupRegistry(merged)
}
