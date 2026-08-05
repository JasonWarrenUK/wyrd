// Package stage provides the baked-in default stage groups for Wyrd.
// These groups are embedded in the binary and available at all times,
// independent of the user's store directory on disk. SL.13 will merge
// user-defined groups (from stages.jsonc) with these defaults at startup.
package stage

import (
	"bytes"
	"embed"
	"encoding/json"
	"io/fs"
	"strings"
	"sync"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

//go:embed defaults/*.jsonc
var defaultsFS embed.FS

var (
	defaultsOnce  sync.Once
	defaultsCache []types.StageGroup
	defaultsErr   error
)

// DefaultStageGroups returns the seven baked-in stage groups: task-flow,
// event-flow, content-flow, habit-flow, project-flow, budget-flow, and
// movement-flow. Results are parsed once and cached; each call returns a
// fresh defensive copy so callers cannot mutate the cache.
func DefaultStageGroups() ([]types.StageGroup, error) {
	defaultsOnce.Do(func() {
		defaultsCache, defaultsErr = loadDefaults()
	})
	if defaultsErr != nil {
		return nil, defaultsErr
	}
	out := make([]types.StageGroup, len(defaultsCache))
	copy(out, defaultsCache)
	return out, nil
}

// loadDefaults reads and parses all JSONC files from the embedded defaults/
// directory. Files are processed in lexicographic order.
func loadDefaults() ([]types.StageGroup, error) {
	entries, err := fs.ReadDir(defaultsFS, "defaults")
	if err != nil {
		return nil, &types.ParseError{Source: "defaults", Message: err.Error()}
	}

	var groups []types.StageGroup
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".jsonc") && !strings.HasSuffix(name, ".json") {
			continue
		}

		path := "defaults/" + name
		data, err := defaultsFS.ReadFile(path)
		if err != nil {
			return nil, &types.ParseError{Source: path, Message: err.Error()}
		}

		cleaned := stripComments(data)
		var g types.StageGroup
		if err := json.Unmarshal(cleaned, &g); err != nil {
			return nil, &types.ParseError{Source: path, Message: err.Error()}
		}
		groups = append(groups, g)
	}
	return groups, nil
}

// stripComments removes // line comments and /* */ block comments from JSONC,
// correctly skipping over string literals. Also removes trailing commas before
// closing braces and brackets, which are common in hand-authored JSONC.
func stripComments(src []byte) []byte {
	var out bytes.Buffer
	s := string(src)
	i := 0
	inString := false
	for i < len(s) {
		ch := s[i]
		if inString {
			out.WriteByte(ch)
			if ch == '\\' && i+1 < len(s) {
				i++
				out.WriteByte(s[i])
			} else if ch == '"' {
				inString = false
			}
			i++
			continue
		}
		if ch == '"' {
			inString = true
			out.WriteByte(ch)
			i++
			continue
		}
		if ch == '/' && i+1 < len(s) && s[i+1] == '/' {
			for i < len(s) && s[i] != '\n' {
				i++
			}
			continue
		}
		if ch == '/' && i+1 < len(s) && s[i+1] == '*' {
			i += 2
			for i+1 < len(s) {
				if s[i] == '*' && s[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			continue
		}
		out.WriteByte(ch)
		i++
	}
	return []byte(strings.TrimSpace(removeTrailingCommas(out.String())))
}

// removeTrailingCommas removes trailing commas before closing braces/brackets.
func removeTrailingCommas(s string) string {
	var out bytes.Buffer
	i := 0
	for i < len(s) {
		if s[i] == ',' {
			j := i + 1
			for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\n' || s[j] == '\r') {
				j++
			}
			if j < len(s) && (s[j] == '}' || s[j] == ']') {
				i++
				continue
			}
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}
