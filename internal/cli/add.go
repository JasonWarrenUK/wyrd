package cli

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// AddOptions holds the parameters for the add command.
type AddOptions struct {
	// Body is the primary content for the new node.
	Body string

	// Title is an optional short display title.
	Title string

	// NodeType overrides the default "task" type.
	NodeType string

	// LinkID is an optional node ID to create a "related" edge to.
	LinkID string
}

// Add creates a new node in the store from the given options.
// Returns the ID of the created node.
func Add(store types.StoreFS, opts AddOptions) (string, error) {
	if opts.Body == "" {
		return "", &types.ValidationError{Field: "body", Message: "node body must not be empty"}
	}

	nodeType := opts.NodeType
	if nodeType == "" {
		nodeType = "task"
	}

	now := time.Now()
	node := &types.Node{
		ID:       uuid.New().String(),
		Body:     opts.Body,
		Title:    opts.Title,
		Types:    []string{nodeType},
		Created:  now,
		Modified: now,
		Properties: map[string]interface{}{
			"status": "inbox",
		},
	}

	if err := store.WriteNode(node); err != nil {
		return "", fmt.Errorf("writing node: %w", err)
	}

	if opts.LinkID != "" {
		edge := &types.Edge{
			ID:      uuid.New().String(),
			Type:    string(types.EdgeRelated),
			From:    node.ID,
			To:      opts.LinkID,
			Created: now,
		}
		if err := store.WriteEdge(edge); err != nil {
			// Node was written; report the edge failure but include the node ID.
			return node.ID, fmt.Errorf("creating link edge: %w", err)
		}
	}

	return node.ID, nil
}
