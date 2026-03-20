package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// JournalOptions holds the parameters for the journal command.
type JournalOptions struct {
	// LinkID is an optional node ID to create a "related" edge to.
	LinkID string
}

// Journal opens the editor with a pre-filled journal template and saves
// the resulting content as a journal node in the store.
func Journal(store types.StoreFS, opts JournalOptions) (string, error) {
	now := time.Now()
	date := now.Format("2006-01-02")

	// Create a temporary file pre-filled with the journal template.
	tmpFile, err := os.CreateTemp("", "wyrd-journal-*.md")
	if err != nil {
		return "", fmt.Errorf("creating temporary file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	heading := fmt.Sprintf("# %s\n\n", date)
	if _, err := tmpFile.WriteString(heading); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("writing journal template: %w", err)
	}
	tmpFile.Close()

	if err := OpenEditor(tmpPath); err != nil {
		return "", err
	}

	content, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("reading edited file: %w", err)
	}

	body := string(content)
	if body == heading || body == "" {
		return "", fmt.Errorf("no content written — journal entry discarded")
	}

	node := &types.Node{
		ID:       uuid.New().String(),
		Body:     body,
		Types:    []string{"journal"},
		Created:  now,
		Modified: now,
		Properties: map[string]interface{}{},
	}
	node.Date.About = &now

	if err := store.WriteNode(node); err != nil {
		return "", fmt.Errorf("writing journal node: %w", err)
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
			return node.ID, fmt.Errorf("creating link edge: %w", err)
		}
	}

	return node.ID, nil
}

// NoteOptions holds the parameters for the note command.
type NoteOptions struct {
	// Title is the note's heading and title property.
	Title string

	// LinkID is an optional node ID to create a "related" edge to.
	LinkID string
}

// Note opens the editor with a pre-filled note template and saves the
// resulting content as a note node in the store.
func Note(store types.StoreFS, opts NoteOptions) (string, error) {
	if opts.Title == "" {
		return "", &types.ValidationError{Field: "title", Message: "note title must not be empty"}
	}

	now := time.Now()

	// Create a temporary file pre-filled with the note template.
	tmpFile, err := os.CreateTemp("", "wyrd-note-*.md")
	if err != nil {
		return "", fmt.Errorf("creating temporary file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	heading := fmt.Sprintf("# %s\n\n", opts.Title)
	if _, err := tmpFile.WriteString(heading); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("writing note template: %w", err)
	}
	tmpFile.Close()

	if err := OpenEditor(tmpPath); err != nil {
		return "", err
	}

	content, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("reading edited file: %w", err)
	}

	body := string(content)
	if body == heading || body == "" {
		return "", fmt.Errorf("no content written — note discarded")
	}

	// Derive a safe filename for the note based on the title.
	slug := safeSlug(opts.Title)
	_ = filepath.Join(store.StorePath(), "nodes", slug)

	node := &types.Node{
		ID:       uuid.New().String(),
		Body:     body,
		Title:    opts.Title,
		Types:    []string{"note"},
		Created:  now,
		Modified: now,
		Properties: map[string]interface{}{},
	}

	if err := store.WriteNode(node); err != nil {
		return "", fmt.Errorf("writing note node: %w", err)
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
			return node.ID, fmt.Errorf("creating link edge: %w", err)
		}
	}

	return node.ID, nil
}

// safeSlug converts a title into a filesystem-safe lowercase slug.
func safeSlug(title string) string {
	out := make([]byte, 0, len(title))
	for _, c := range title {
		switch {
		case c >= 'A' && c <= 'Z':
			out = append(out, byte(c+32))
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			out = append(out, byte(c))
		default:
			out = append(out, '-')
		}
	}
	return string(out)
}
