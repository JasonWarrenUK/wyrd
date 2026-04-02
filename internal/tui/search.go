package tui

import (
	"sort"
	"strings"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// SearchResultKind distinguishes the entity type of a search result.
type SearchResultKind int

const (
	// SearchResultCommand is a palette command result.
	SearchResultCommand SearchResultKind = iota

	// SearchResultNode is a graph node result.
	SearchResultNode

	// SearchResultEdge is a graph edge result.
	SearchResultEdge
)

// SearchResult is the unified result type returned by searchAll.
// It carries enough information for both the palette View (display) and
// confirm (dispatch) paths.
type SearchResult struct {
	Kind        SearchResultKind
	Score       int
	Title       string // display name
	Description string // secondary text
	Hint        string // keyboard shortcut (commands only)

	// Dispatch fields — confirm() reads these.
	NodeID  string   // non-empty for SearchResultNode; From node ID for SearchResultEdge
	EdgeID  string   // non-empty for SearchResultEdge
	Command *Command // non-nil for SearchResultCommand

	// Types carries the raw node type slice for SearchResultNode entries.
	// Used by the fuzzy overlay to render TypeBadges. Empty for other kinds.
	Types []string
}

// searchAll queries nodes, edges, and commands and returns a ranked result list.
//
// An empty query returns all commands only (no nodes or edges), so the palette
// opens with a usable default state on small graphs and does not overwhelm the
// user on large ones.
//
// When index is nil, only command results are returned. Archived nodes
// (Properties["status"] == "archived") are excluded from results.
// Results are truncated to 50 entries.
func searchAll(query string, commands []Command, index types.GraphIndex) []SearchResult {
	q := strings.TrimSpace(strings.ToLower(query))

	var results []SearchResult

	// Always search commands.
	for i := range commands {
		score, matched := scoreCommand(commands[i], q)
		if matched {
			cmd := commands[i]
			results = append(results, SearchResult{
				Kind:        SearchResultCommand,
				Score:       score,
				Title:       cmd.Name,
				Description: cmd.Description,
				Hint:        cmd.Hint,
				Command:     &cmd,
			})
		}
	}

	// Only search nodes and edges when there is a non-empty query and an index.
	if q != "" && index != nil {
		for _, node := range index.AllNodes() {
			score, matched := scoreNode(node, q)
			if !matched {
				continue
			}
			title := node.Title
			if title == "" {
				title = firstLine(node.Body)
			}
			if title == "" {
				title = node.ID
			}
			description := strings.Join(node.Types, ", ")
			results = append(results, SearchResult{
				Kind:        SearchResultNode,
				Score:       score,
				Title:       title,
				Description: description,
				NodeID:      node.ID,
				Types:       node.Types,
			})
		}

		for _, edge := range index.AllEdges() {
			score, matched, title, description := scoreEdge(edge, q, index)
			if !matched {
				continue
			}
			results = append(results, SearchResult{
				Kind:        SearchResultEdge,
				Score:       score,
				Title:       title,
				Description: description,
				NodeID:      edge.From, // navigate to source node on confirm
				EdgeID:      edge.ID,
			})
		}
	}

	// Sort by score descending. Stable sort preserves insertion order
	// (commands → nodes → edges) within equal-score groups.
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	const maxResults = 50
	if len(results) > maxResults {
		results = results[:maxResults]
	}

	return results
}

// scoreCommand returns the match score for a command against query.
// Returns (0, false) when no match. Uses max-of-fields scoring.
func scoreCommand(cmd Command, query string) (int, bool) {
	if query == "" {
		// Empty query matches all commands at score 0.
		return 0, true
	}
	name := strings.ToLower(cmd.Name)
	desc := strings.ToLower(cmd.Description)

	if name == query {
		return 100, true
	}
	if strings.Contains(name, query) {
		return 80, true
	}
	if strings.Contains(desc, query) {
		return 40, true
	}
	return 0, false
}

// scoreNode returns the match score for a node against query.
// Archived nodes always return (0, false). Uses max-of-fields scoring.
func scoreNode(node *types.Node, query string) (int, bool) {
	if node == nil {
		return 0, false
	}
	// Skip archived nodes.
	if status, ok := node.Properties["status"]; ok {
		if s, ok := status.(string); ok && s == "archived" {
			return 0, false
		}
	}

	best := 0

	if node.Title != "" && strings.Contains(strings.ToLower(node.Title), query) {
		best = max(best, 70)
	}
	for _, t := range node.Types {
		if strings.Contains(strings.ToLower(t), query) {
			best = max(best, 50)
		}
	}
	if strings.Contains(strings.ToLower(firstLine(node.Body)), query) {
		best = max(best, 30)
	}
	if strings.Contains(strings.ToLower(node.ID), query) {
		best = max(best, 20)
	}

	return best, best > 0
}

// scoreEdge returns the match score, match flag, display title, and description
// for an edge against query. Uses max-of-fields scoring.
func scoreEdge(edge *types.Edge, query string, index types.GraphIndex) (int, bool, string, string) {
	if edge == nil {
		return 0, false, "", ""
	}

	best := 0

	if strings.Contains(strings.ToLower(edge.Type), query) {
		best = max(best, 50)
	}

	// Resolve From and To node titles for display and scoring.
	fromTitle := edge.From
	toTitle := edge.To
	if index != nil {
		if fromNode, err := index.GetNode(edge.From); err == nil {
			if fromNode.Title != "" {
				fromTitle = fromNode.Title
			} else {
				fromTitle = firstLine(fromNode.Body)
			}
			if fromTitle == "" {
				fromTitle = edge.From
			}
		}
		if toNode, err := index.GetNode(edge.To); err == nil {
			if toNode.Title != "" {
				toTitle = toNode.Title
			} else {
				toTitle = firstLine(toNode.Body)
			}
			if toTitle == "" {
				toTitle = edge.To
			}
		}
	}

	if strings.Contains(strings.ToLower(fromTitle), query) ||
		strings.Contains(strings.ToLower(toTitle), query) {
		best = max(best, 40)
	}

	title := edge.Type + ": " + fromTitle + " → " + toTitle
	description := "edge"

	return best, best > 0, title, description
}

