package views

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jasonwarrenuk/wyrd/internal/jsonc"
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

	var view types.SavedView
	if err := jsonc.Unmarshal(data, &view); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}

	// Use the file stem as the name if the JSON did not provide one.
	if view.Name == "" {
		base := filepath.Base(path)
		view.Name = strings.TrimSuffix(strings.TrimSuffix(base, ".jsonc"), ".json")
	}

	return &view, nil
}
