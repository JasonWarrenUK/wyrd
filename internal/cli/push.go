package cli

import (
	"fmt"
	"io"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// PushOptions holds the parameters for the push command.
type PushOptions struct {
	// NodeID is the UUID of the node to push to Obsidian.
	NodeID string
}

// Push pushes a node to Obsidian. Currently a stub — the Obsidian sync engine
// is implemented by a separate agent and will be wired in when available.
func Push(store types.StoreFS, opts PushOptions, out io.Writer) error {
	if opts.NodeID == "" {
		return &types.ValidationError{Field: "node-id", Message: "node ID must not be empty"}
	}

	// Verify the node exists before printing the stub message.
	if _, err := store.ReadNode(opts.NodeID); err != nil {
		return fmt.Errorf("node %q not found: %w", opts.NodeID, err)
	}

	fmt.Fprintf(out, "Push to Obsidian is not yet available — node %q queued for when sync engine is ready.\n", opts.NodeID)
	return nil
}
