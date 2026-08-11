package stage

import "github.com/jasonwarrenuk/wyrd/internal/types"

// MergeStageGroups builds a StageGroupRegistry from baked-in defaults and
// user-supplied groups. User groups are appended after defaults so that a user
// group with the same Name shadows the default — the last-wins-by-name seam
// documented in types.NewStageGroupRegistry. Neither input slice is mutated.
//
// Pass nil for user when no stages.jsonc exists yet; SL.13 will supply user
// groups as the second argument once the reader is implemented.
//
// The returned registry also records which names came from user (TD.15),
// via types.NewStageGroupRegistryFromMerge — see MergeKinds' matching note.
func MergeStageGroups(defaults, user []types.StageGroup) *types.StageGroupRegistry {
	return types.NewStageGroupRegistryFromMerge(defaults, user)
}
