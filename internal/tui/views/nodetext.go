package views

import (
	"strings"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// NodeTitle returns the node's Title when set, falling back to the first
// line of Body. Relocated from internal/tui/detail.go (TD.20) so
// ProseRenderer can share the exact title-resolution rule DetailRenderer
// uses, rather than drifting with its own copy — views cannot import
// package tui, so detail.go now calls this version instead.
func NodeTitle(node *types.Node) string {
	if node.Title != "" {
		return node.Title
	}
	return FirstLine(node.Body)
}

// FirstLine returns the first non-empty line of s.
func FirstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return s
}

// BodyWithoutTitle returns node.Body with the first line stripped when it
// would duplicate the rendered title. Relocated from internal/tui/detail.go
// (TD.20) — see NodeTitle's doc comment for why. Covers two cases:
//   - node.Title is empty: the title was derived from FirstLine(body), so
//     drop that line to avoid repeating it.
//   - node.Title is set and the body's first non-empty line is the same
//     text (possibly as a markdown heading, e.g. "# My Title"): drop that
//     line too.
func BodyWithoutTitle(node *types.Node) string {
	body := strings.TrimRight(node.Body, "\n")
	if body == "" {
		return ""
	}

	lines := strings.SplitN(body, "\n", 2)
	firstRaw := strings.TrimSpace(lines[0])
	firstPlain := strings.TrimLeft(firstRaw, "# ")

	var titlePlain string
	if node.Title != "" {
		titlePlain = node.Title
	} else {
		titlePlain = firstPlain
	}

	if strings.EqualFold(firstPlain, titlePlain) {
		if len(lines) < 2 {
			return ""
		}
		return strings.TrimLeft(lines[1], "\n")
	}
	return body
}
