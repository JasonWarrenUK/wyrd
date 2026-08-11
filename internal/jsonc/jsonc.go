// Package jsonc provides a single, string-aware implementation of JSONC
// (JSON with Comments) parsing and writing, consolidating what used to be
// seven independent implementations scattered across the store, stage,
// tui, sync and plugin packages.
//
// Strip removes // line comments, /* */ block comments, and trailing commas
// in a single pass, tracking string state throughout so that none of these
// removals can corrupt a string value — for example a node body containing
// a URL such as "https://example.com" must survive untouched.
package jsonc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Strip removes JSONC-only syntax — // line comments, /* */ block comments,
// and trailing commas before } or ] — from src, returning valid JSON bytes.
// String values are tracked throughout so comment markers, trailing commas
// and escaped quotes inside strings are copied verbatim rather than parsed.
//
// Strip does not trim leading or trailing whitespace: json.Unmarshal is
// whitespace-insensitive, so trimming would be incidental behaviour, not a
// contract callers should rely on.
func Strip(src []byte) []byte {
	var out bytes.Buffer
	s := string(src)
	i := 0
	inString := false

	// pendingComma buffers a comma seen outside a string so we can decide,
	// once we know what follows it, whether it is a trailing comma to drop
	// or an ordinary separator to keep. Whitespace and comments in between
	// are both skipped while deciding — "x", // note\n} is a trailing comma
	// with a comment between it and the closing brace, and must be dropped
	// the same as "x",\n}. Doing this in the same pass as comment stripping
	// — rather than a second pass over already-stripped output — means
	// inString is always accurate: a second pass would be string-blind,
	// since comment stripping has already thrown away the information
	// needed to tell a real comma from one inside what used to be a string.
	pendingComma := false

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

		if pendingComma {
			if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
				i++
				continue
			}
			// A comment between the comma and whatever follows doesn't
			// change the answer — skip over it without resolving
			// pendingComma yet, then keep looking.
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
			if ch != '}' && ch != ']' {
				out.WriteByte(',')
			}
			pendingComma = false
			// Fall through to process ch normally below.
		}

		if ch == '"' {
			inString = true
			out.WriteByte(ch)
			i++
			continue
		}

		// Line comment.
		if ch == '/' && i+1 < len(s) && s[i+1] == '/' {
			for i < len(s) && s[i] != '\n' {
				i++
			}
			continue
		}

		// Block comment.
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

		if ch == ',' {
			pendingComma = true
			i++
			continue
		}

		out.WriteByte(ch)
		i++
	}

	// A trailing comma right at end-of-input (no closing bracket followed)
	// is not valid JSON either way; emit it verbatim and let json.Unmarshal
	// report the syntax error.
	if pendingComma {
		out.WriteByte(',')
	}

	return out.Bytes()
}

// Unmarshal strips JSONC syntax from src and unmarshals the result into v.
func Unmarshal(src []byte, v any) error {
	return json.Unmarshal(Strip(src), v)
}

// ReadFile reads path and strips JSONC comments and trailing commas,
// returning JSON bytes ready for json.Unmarshal. The raw file on disk is
// never modified — stripping only affects the bytes returned in memory.
//
// A missing file's error wraps os.ErrNotExist via %w, so callers can test
// it with errors.Is(err, os.ErrNotExist) or os.IsNotExist(err) regardless
// of how many layers the error has been wrapped through since.
func ReadFile(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return Strip(raw), nil
}

// WriteFile serialises v as pretty-printed JSON (two-space indent, HTML
// escaping disabled) and writes it to path atomically — via a temp file in
// the same directory followed by a rename — so a process killed mid-write
// (as a merge driver can be, mid-rebase) never leaves a truncated file.
func WriteFile(path string, v any) error {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encoding %s: %w", path, err)
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".wyrd-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file for %s: %w", path, err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(buf.Bytes()); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("writing temp file for %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("closing temp file for %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("renaming temp file to %s: %w", path, err)
	}
	return nil
}
