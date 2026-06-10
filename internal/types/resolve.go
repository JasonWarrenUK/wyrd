package types

// ResolveStageGroup maps a node to its kind's stage group. It returns
// (StageGroup, true) when the node has a known kind whose stage group resolves
// in groups. It returns (StageGroup{}, false) for any of:
//   - nil node, nil registries
//   - empty node.Kind (untriaged node)
//   - unknown kind name
//   - kind references an unknown stage group name
//
// No TUI dependencies; safe to call from handlers, tests, and future CLI code.
func ResolveStageGroup(kinds *KindRegistry, groups *StageGroupRegistry, node *Node) (StageGroup, bool) {
	if node == nil || node.Kind == "" || kinds == nil || groups == nil {
		return StageGroup{}, false
	}
	k, ok := kinds.Lookup(node.Kind)
	if !ok {
		return StageGroup{}, false
	}
	return groups.Lookup(k.StageGroup)
}
