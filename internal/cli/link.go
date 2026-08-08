package cli

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// validateLinkTarget checks that targetID is a well-formed UUID and, when
// index is non-nil, that it resolves to an existing node. It mirrors the
// validation in internal/tui/form.go's applyEdgeChanges: malformed IDs are
// always rejected, but existence checking is skipped when no index is
// supplied (matching the nil-safe pattern used there).
//
// Called before the node is written so a failed link leaves nothing behind,
// rather than the previous behaviour of writing the node and only then
// failing on the edge.
func validateLinkTarget(index types.GraphIndex, targetID string) error {
	if _, err := uuid.Parse(targetID); err != nil {
		return &types.ValidationError{Field: "link", Message: fmt.Sprintf("malformed link ID %q: not a valid UUID", targetID)}
	}

	if index == nil {
		return nil
	}

	if _, err := index.GetNode(targetID); err != nil {
		return &types.ValidationError{Field: "link", Message: fmt.Sprintf("link target %q does not exist", targetID)}
	}

	return nil
}
