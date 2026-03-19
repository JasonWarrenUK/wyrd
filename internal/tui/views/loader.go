package views

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// LoadViews reads all saved view JSONC files from {storePath}/views/ and
// returns them as a slice of SavedView values. Files that cannot be parsed
// are skipped and reported via the returned error list.
func LoadViews(storePath string) ([]*types.SavedView, []error) {
	viewsDir := filepath.Join(storePath, "views")

	entries, err := os.ReadDir(viewsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []error{fmt.Errorf("reading views directory: %w", err)}
	}

	var views []*types.SavedView
	var errs []error

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".jsonc") && !strings.HasSuffix(name, ".json") {
			continue
		}

		path := filepath.Join(viewsDir, name)
		view, err := loadViewFile(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("loading view %q: %w", name, err))
			continue
		}
		views = append(views, view)
	}

	return views, errs
}

// LoadView reads a single saved view JSONC file by name (without extension).
// It searches for both .jsonc and .json suffixes.
func LoadView(storePath, viewName string) (*types.SavedView, error) {
	viewsDir := filepath.Join(storePath, "views")

	for _, ext := range []string{".jsonc", ".json"} {
		path := filepath.Join(viewsDir, viewName+ext)
		view, err := loadViewFile(path)
		if err == nil {
			return view, nil
		}
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("loading view %q: %w", viewName, err)
		}
	}

	return nil, fmt.Errorf("view %q not found in %s", viewName, viewsDir)
}

// loadViewFile reads and parses a single view file. JSONC comments (// and /* */)
// are stripped before unmarshalling.
func loadViewFile(path string) (*types.SavedView, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cleaned := stripJSONCComments(data)

	var view types.SavedView
	if err := json.Unmarshal(cleaned, &view); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}

	// Use the file stem as the name if the JSON did not provide one.
	if view.Name == "" {
		base := filepath.Base(path)
		view.Name = strings.TrimSuffix(strings.TrimSuffix(base, ".jsonc"), ".json")
	}

	return &view, nil
}

// stripJSONCComments removes single-line (//) and multi-line (/* */) comments
// from JSONC content, producing valid JSON. This is a simple byte-level pass
// that handles the common cases; it does not attempt to parse string literals
// perfectly, but is sufficient for configuration files.
func stripJSONCComments(src []byte) []byte {
	out := make([]byte, 0, len(src))
	i := 0
	for i < len(src) {
		// Multi-line comment.
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '*' {
			i += 2
			for i+1 < len(src) {
				if src[i] == '*' && src[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			continue
		}
		// Single-line comment.
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '/' {
			i += 2
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		}
		// String literal — copy verbatim, including escaped characters.
		if src[i] == '"' {
			out = append(out, src[i])
			i++
			for i < len(src) {
				ch := src[i]
				out = append(out, ch)
				i++
				if ch == '\\' && i < len(src) {
					out = append(out, src[i])
					i++
					continue
				}
				if ch == '"' {
					break
				}
			}
			continue
		}
		out = append(out, src[i])
		i++
	}
	return out
}
