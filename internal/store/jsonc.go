// Package store implements the flat-file graph storage layer for Wyrd.
// Nodes and edges are stored as individual JSONC files. Comments in JSONC
// files are preserved through read/write cycles.
package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// readJSONC reads a JSONC file and returns the parsed data and the raw bytes
// (with comments stripped) for JSON unmarshalling. Comments are stripped
// only for parsing; the raw file on disk is never modified.
func readJSONC(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	stripped := stripComments(raw)
	return stripped, nil
}

// writeJSONC atomically writes data as pretty-printed JSON to path.
// It writes to a temp file first, then renames to prevent corruption.
func writeJSONC(path string, data interface{}) error {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(data); err != nil {
		return fmt.Errorf("encoding %s: %w", path, err)
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".wyrd-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file for %s: %w", path, err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("writing temp file for %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("closing temp file for %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("renaming temp file to %s: %w", path, err)
	}
	return nil
}

// stripComments removes // line comments and /* */ block comments from JSONC.
// This is a minimal implementation sufficient for Wyrd's use: comments must
// not appear inside string values, which is the case for all Wyrd JSONC files.
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
		// Line comment
		if ch == '/' && i+1 < len(s) && s[i+1] == '/' {
			for i < len(s) && s[i] != '\n' {
				i++
			}
			continue
		}
		// Block comment
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
	// Remove trailing commas before } or ] — common in JSONC
	result := out.String()
	result = removeTrailingCommas(result)
	return []byte(strings.TrimSpace(result))
}

// removeTrailingCommas removes trailing commas before closing braces/brackets.
func removeTrailingCommas(s string) string {
	var out bytes.Buffer
	i := 0
	for i < len(s) {
		if s[i] == ',' {
			// Look ahead for the next non-whitespace character
			j := i + 1
			for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\n' || s[j] == '\r') {
				j++
			}
			if j < len(s) && (s[j] == '}' || s[j] == ']') {
				i++ // skip the comma
				continue
			}
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}
