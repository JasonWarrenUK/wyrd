package cli

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// JournalOptions holds the parameters for the journal command.
type JournalOptions struct {
	// Title is the journal entry title. Defaults to today's date if empty.
	Title string

	// Body is the journal entry content. Required.
	Body string

	// LinkID is an optional node ID to create a "related" edge to.
	LinkID string

	// Index resolves LinkID against the graph before the node is written.
	// Nil skips existence checking (malformed UUIDs are still rejected).
	Index types.GraphIndex

	// Clock supplies the current time. Defaults to types.RealClock{} when nil.
	Clock types.Clock
}

// Journal creates a journal node from the given options.
// Returns the ID of the created node.
func Journal(store types.StoreFS, opts JournalOptions) (string, error) {
	if opts.Body == "" {
		return "", &types.ValidationError{Field: "body", Message: "journal body must not be empty"}
	}

	clock := opts.Clock
	if clock == nil {
		clock = types.RealClock{}
	}

	if opts.LinkID != "" {
		if err := validateLinkTarget(opts.Index, opts.LinkID); err != nil {
			return "", err
		}
	}

	now := clock.Now()

	title := opts.Title
	if title == "" {
		title = now.Format("2006-01-02")
	}

	node := &types.Node{
		ID:         uuid.New().String(),
		Title:      title,
		Body:       opts.Body,
		Types:      []string{"journal"},
		Properties: map[string]interface{}{},
	}
	node.Date.Created = now
	node.Date.About = &now

	if err := store.WriteNode(node); err != nil {
		return "", fmt.Errorf("writing journal node: %w", err)
	}

	if opts.LinkID != "" {
		edge := &types.Edge{
			ID:   uuid.New().String(),
			Type: string(types.EdgeRelated),
			From: node.ID,
			To:   opts.LinkID,
		}
		edge.Date.Created = now
		if err := store.WriteEdge(edge); err != nil {
			return node.ID, fmt.Errorf("creating link edge: %w", err)
		}
	}

	return node.ID, nil
}

// NoteOptions holds the parameters for the note command.
type NoteOptions struct {
	// Title is the note's heading and title property. Required.
	Title string

	// Body is the note content. Required.
	Body string

	// LinkID is an optional node ID to create a "related" edge to.
	LinkID string

	// Index resolves LinkID against the graph before the node is written.
	// Nil skips existence checking (malformed UUIDs are still rejected).
	Index types.GraphIndex

	// Clock supplies the current time. Defaults to types.RealClock{} when nil.
	Clock types.Clock
}

// Note creates a note node from the given options.
// Returns the ID of the created node.
func Note(store types.StoreFS, opts NoteOptions) (string, error) {
	if opts.Title == "" {
		return "", &types.ValidationError{Field: "title", Message: "note title must not be empty"}
	}
	if opts.Body == "" {
		return "", &types.ValidationError{Field: "body", Message: "note body must not be empty"}
	}

	clock := opts.Clock
	if clock == nil {
		clock = types.RealClock{}
	}

	if opts.LinkID != "" {
		if err := validateLinkTarget(opts.Index, opts.LinkID); err != nil {
			return "", err
		}
	}

	now := clock.Now()

	node := &types.Node{
		ID:         uuid.New().String(),
		Title:      opts.Title,
		Body:       opts.Body,
		Types:      []string{"note"},
		Properties: map[string]interface{}{},
	}
	node.Date.Created = now

	if err := store.WriteNode(node); err != nil {
		return "", fmt.Errorf("writing note node: %w", err)
	}

	if opts.LinkID != "" {
		edge := &types.Edge{
			ID:   uuid.New().String(),
			Type: string(types.EdgeRelated),
			From: node.ID,
			To:   opts.LinkID,
		}
		edge.Date.Created = now
		if err := store.WriteEdge(edge); err != nil {
			return node.ID, fmt.Errorf("creating link edge: %w", err)
		}
	}

	return node.ID, nil
}
