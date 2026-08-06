package tui

import "charm.land/lipgloss/v2"

// compositeOverlay places overlay horizontally and vertically centred over
// frame using lipgloss.Place, compositing the result at Z(1) over the base
// frame at Z(0) via the lipgloss Compositor. Returns frame unchanged when
// overlay is empty or width/height are zero.
func compositeOverlay(frame, overlay string, width, height int) string {
	if overlay == "" || width <= 0 || height <= 0 {
		return frame
	}

	// Place centres the overlay across the full terminal dimensions. The
	// whitespace padding it adds around the box overwrites the frame behind
	// it, so the visible backdrop is the terminal background rather than a
	// see-through view of the pane layout — consistent with the user's choice
	// of literal Place centring over a transparent dim.
	placed := lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, overlay)

	base := lipgloss.NewLayer(frame).Z(0)
	top := lipgloss.NewLayer(placed).Z(1)
	return lipgloss.NewCompositor(base, top).Render()
}

// overlayVPHeight computes a content-driven viewport height for an overlay,
// clamping to a safe maximum so the rendered box never overflows the terminal.
//
//   - contentLines: the number of lines the overlay content contains.
//   - termHeight:   the full terminal height.
//   - chromeRows:   rows consumed by the box chrome (border, padding, title,
//     divider, footer — typically 6 or 7).
//
// A margin of 2 rows is reserved so the box never touches the terminal edges.
// The returned height is at least 1.
func overlayVPHeight(contentLines, termHeight, chromeRows int) int {
	const margin = 2
	maxVP := termHeight - chromeRows - margin
	if maxVP < 1 {
		maxVP = 1
	}
	h := contentLines
	if h > maxVP {
		h = maxVP
	}
	if h < 1 {
		h = 1
	}
	return h
}

// provenanceMarker reports whether name is user-owned and, if so, whether it
// shadows a baked-in default. Used by kindsOverlay and stagesOverlay to
// render "(custom)" vs "(edited)" beside each entry.
//
// The distinction matters specifically because of SL.16/SL.17 edit mode:
// editing a baked-in default writes a full shadow copy into the user's file
// under the SAME name, so "name is absent from the defaults list" (the
// original check, before edit mode existed) can no longer tell a purely
// custom entry apart from a diverged-from-upstream one — an edited default
// keeps its name, so it still passes that check and renders unmarked, which
// is wrong: it IS a permanent user override now, just one that happens to
// share a name with something built in. Checking userNames (membership in
// the actual user file, not name-uniqueness) is the correct test in both
// directions, and cross-referencing defaultNames on top of that recovers the
// custom/edited distinction.
//
//   - not in userNames: a baked-in default, untouched. No marker ("").
//   - in userNames, name also in defaultNames: a shadowed/edited default.
//     "(edited)" — this entry is diverged from upstream and won't pick up
//     future improvements to the built-in version.
//   - in userNames, name not in defaultNames: purely user-defined.
//     "(custom)".
func provenanceMarker(name string, userNames, defaultNames map[string]bool) string {
	if !userNames[name] {
		return ""
	}
	if defaultNames[name] {
		return "(edited)"
	}
	return "(custom)"
}
