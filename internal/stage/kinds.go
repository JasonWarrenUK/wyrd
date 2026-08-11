package stage

import (
	"embed"
	"encoding/json"
	"io/fs"
	"strings"
	"sync"

	"github.com/jasonwarrenuk/wyrd/internal/jsonc"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

//go:embed kinds/*.jsonc
var kindsFS embed.FS

var (
	kindsOnce  sync.Once
	kindsCache []types.Kind
	kindsErr   error
)

// DefaultKinds returns the eleven baked-in default kinds: Task, Goblin,
// Habit, Event, Travel, Talk, Project, Journal, Note, Budget, and Movement.
// Results are parsed once and cached; each call returns a fresh defensive
// copy so callers cannot mutate the cache.
//
// The returned slice is ordered lexicographically by filename, which is
// alphabetical by kind file name — not by Name field. Use MergeKinds to
// combine with user-supplied kinds.
func DefaultKinds() ([]types.Kind, error) {
	kindsOnce.Do(func() {
		kindsCache, kindsErr = loadKindDefaults()
	})
	if kindsErr != nil {
		return nil, kindsErr
	}
	out := make([]types.Kind, len(kindsCache))
	copy(out, kindsCache)
	return out, nil
}

// MergeKinds builds a KindRegistry from baked-in defaults and user-supplied
// kinds. User kinds are appended after defaults so that a user kind with the
// same Name shadows the default — the last-wins-by-name seam documented in
// types.NewKindRegistry. Neither input slice is mutated.
func MergeKinds(defaults, user []types.Kind) *types.KindRegistry {
	merged := make([]types.Kind, 0, len(defaults)+len(user))
	merged = append(merged, defaults...)
	merged = append(merged, user...)
	return types.NewKindRegistry(merged)
}

// loadKindDefaults reads and parses all JSONC files from the embedded kinds/
// directory. Files are processed in lexicographic order.
func loadKindDefaults() ([]types.Kind, error) {
	entries, err := fs.ReadDir(kindsFS, "kinds")
	if err != nil {
		return nil, &types.ParseError{Source: "kinds", Message: err.Error()}
	}

	var kinds []types.Kind
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".jsonc") && !strings.HasSuffix(name, ".json") {
			continue
		}

		path := "kinds/" + name
		data, err := kindsFS.ReadFile(path)
		if err != nil {
			return nil, &types.ParseError{Source: path, Message: err.Error()}
		}

		cleaned := jsonc.Strip(data)
		var k types.Kind
		if err := json.Unmarshal(cleaned, &k); err != nil {
			return nil, &types.ParseError{Source: path, Message: err.Error()}
		}
		kinds = append(kinds, k)
	}
	return kinds, nil
}
