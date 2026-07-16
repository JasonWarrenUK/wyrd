package types

// EvalBlockers walks node's incoming "blocks" edges and reports whether any
// still blocks (blocked) and whether that verdict rests on at least one
// unresolvable blocker (unresolved) rather than a confirmed non-terminal one.
//
// Terminality is resolved via ResolveStageGroup + StageGroup.IsTerminal (SL.2).
// When a blocker's terminality cannot be determined — unknown kind, empty
// stage, or the registries were never wired up — the block is treated as
// still active ("presence blocks"): only a *confirmed* terminal blocker lifts
// the block. Use unresolved to tell a confirmed block apart from one resting
// on an unresolvable blocker.
//
// No TUI or query-engine dependencies; safe to call from the query evaluator,
// TUI rendering, and tests alike.
func EvalBlockers(index GraphIndex, kinds *KindRegistry, groups *StageGroupRegistry, node *Node) (blocked bool, unresolved bool) {
	if node == nil || index == nil {
		return false, false
	}
	for _, edge := range index.EdgesTo(node.ID) {
		if edge.Type != string(EdgeBlocks) {
			continue
		}
		source, err := index.GetNode(edge.From)
		if err != nil || source == nil {
			// Dangling edge: the blocker can't be inspected at all.
			blocked = true
			unresolved = true
			continue
		}
		group, ok := ResolveStageGroup(kinds, groups, source)
		if !ok {
			// Unknown kind / empty stage / registries not wired: presence
			// blocks, but flag it as unresolved rather than confirmed.
			blocked = true
			unresolved = true
			continue
		}
		if !group.IsTerminal(source.Stage) {
			blocked = true
		}
	}
	return blocked, unresolved
}

// Blockers returns the nodes currently holding a block on node — those
// reached via an incoming "blocks" edge that are non-terminal or whose
// terminality could not be resolved. A dangling edge (blocker node missing)
// is omitted since there is no node to return, even though it does hold the
// block per EvalBlockers. Order follows GraphIndex.EdgesTo.
func Blockers(index GraphIndex, kinds *KindRegistry, groups *StageGroupRegistry, node *Node) []*Node {
	if node == nil || index == nil {
		return nil
	}
	var blockers []*Node
	for _, edge := range index.EdgesTo(node.ID) {
		if edge.Type != string(EdgeBlocks) {
			continue
		}
		source, err := index.GetNode(edge.From)
		if err != nil || source == nil {
			continue
		}
		group, ok := ResolveStageGroup(kinds, groups, source)
		if !ok {
			blockers = append(blockers, source)
			continue
		}
		if !group.IsTerminal(source.Stage) {
			blockers = append(blockers, source)
		}
	}
	return blockers
}
