package jsonc

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStrip_NoComments(t *testing.T) {
	input := `{"key": "value"}`
	got := string(Strip([]byte(input)))
	if got != input {
		t.Errorf("Strip() = %q, want %q", got, input)
	}
}

func TestStrip_LineComment(t *testing.T) {
	input := "{\n  // this is a comment\n  \"key\": \"value\"\n}"
	want := "{\n  \n  \"key\": \"value\"\n}"
	got := string(Strip([]byte(input)))
	if got != want {
		t.Errorf("Strip() = %q, want %q", got, want)
	}
}

func TestStrip_BlockComment(t *testing.T) {
	input := `{"key": /* comment */ "value"}`
	want := `{"key":  "value"}`
	got := string(Strip([]byte(input)))
	if got != want {
		t.Errorf("Strip() = %q, want %q", got, want)
	}
}

func TestStrip_TrailingComma(t *testing.T) {
	input := `{"key": "value",}`
	want := `{"key": "value"}`
	got := string(Strip([]byte(input)))
	if got != want {
		t.Errorf("Strip() = %q, want %q", got, want)
	}
}

// TestStrip_CommentInsideString is the SY.2 case: a node body containing a
// URL must survive Strip untouched. The old sync/merge.go regex stripper
// (`//[^\n]*`) corrupted this by treating the "//" in "https://" as a line
// comment marker, truncating everything after it on the same line.
func TestStrip_CommentInsideString(t *testing.T) {
	input := `{"body": "see https://example.com/x"}`
	got := string(Strip([]byte(input)))
	if got != input {
		t.Errorf("Strip() = %q, want %q (unchanged)", got, input)
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(got), &m); err != nil {
		t.Fatalf("Strip() output does not parse as JSON: %v", err)
	}
	if m["body"] != "see https://example.com/x" {
		t.Errorf("body = %q, want unchanged URL", m["body"])
	}
}

func TestStrip_BlockCommentMarkerInsideString(t *testing.T) {
	input := `{"body": "a /* not a comment */ b"}`
	got := string(Strip([]byte(input)))
	if got != input {
		t.Errorf("Strip() = %q, want %q (unchanged)", got, input)
	}
}

// TestStrip_TrailingCommaInsideStringValue fails against the pre-TD.1 store
// implementation, which stripped trailing commas in a second pass over
// already-comment-stripped output and so had lost track of string state.
func TestStrip_TrailingCommaInsideStringValue(t *testing.T) {
	input := `{"body": "a, } b"}`
	got := string(Strip([]byte(input)))
	if got != input {
		t.Errorf("Strip() = %q, want %q (unchanged)", got, input)
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(got), &m); err != nil {
		t.Fatalf("Strip() output does not parse as JSON: %v", err)
	}
	if m["body"] != "a, } b" {
		t.Errorf("body = %q, want %q", m["body"], "a, } b")
	}
}

func TestStrip_EscapedQuoteInString(t *testing.T) {
	input := `{"body": "she said \"hi // there\", ok"}`
	got := string(Strip([]byte(input)))
	if got != input {
		t.Errorf("Strip() = %q, want %q (unchanged)", got, input)
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(got), &m); err != nil {
		t.Fatalf("Strip() output does not parse as JSON: %v", err)
	}
	if m["body"] != `she said "hi // there", ok` {
		t.Errorf("body = %q", m["body"])
	}
}

// TestStrip_DoesNotTrimWhitespace guards the deliberate design point: four
// of the seven original implementations did not trim, and json.Unmarshal is
// whitespace-insensitive, so Strip must not add trimming behaviour that
// some callers never had.
func TestStrip_DoesNotTrimWhitespace(t *testing.T) {
	input := "\n\n  {\"key\": \"value\"}  \n\n"
	got := string(Strip([]byte(input)))
	if got != input {
		t.Errorf("Strip() = %q, want %q (unchanged, no trim)", got, input)
	}
}

// TestStrip_TrailingCommaFollowedByLineComment guards a real regression
// found while porting internal/store's fixture tests: a trailing comma
// followed by a same-line "// comment" before the closing brace must still
// be recognised as trailing and dropped, even though a comment (not
// whitespace alone) sits between the comma and the brace.
func TestStrip_TrailingCommaFollowedByLineComment(t *testing.T) {
	input := "{\n\t\"a\": 1, // trailing comma with a comment after it\n}"
	got := string(Strip([]byte(input)))
	var m map[string]int
	if err := json.Unmarshal([]byte(got), &m); err != nil {
		t.Fatalf("Strip() output does not parse as JSON: %v\noutput: %q", err, got)
	}
	if m["a"] != 1 {
		t.Errorf("a = %d, want 1", m["a"])
	}
}

// TestStrip_TrailingCommaFollowedByBlockComment mirrors the line-comment
// case above for a block comment sitting between a trailing comma and the
// closing bracket.
func TestStrip_TrailingCommaFollowedByBlockComment(t *testing.T) {
	input := "{\n\t\"a\": 1, /* note */\n}"
	got := string(Strip([]byte(input)))
	var m map[string]int
	if err := json.Unmarshal([]byte(got), &m); err != nil {
		t.Fatalf("Strip() output does not parse as JSON: %v\noutput: %q", err, got)
	}
	if m["a"] != 1 {
		t.Errorf("a = %d, want 1", m["a"])
	}
}

func TestUnmarshal_StripsAndDecodes(t *testing.T) {
	input := []byte("{\n  // comment\n  \"key\": \"value\",\n}")
	var m map[string]string
	if err := Unmarshal(input, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if m["key"] != "value" {
		t.Errorf("key = %q, want value", m["key"])
	}
}

func TestReadFile_NotExistWrapsErrNotExist(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadFile(filepath.Join(dir, "missing.jsonc"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("errors.Is(err, os.ErrNotExist) = false, want true; err = %v", err)
	}
}

func TestReadFile_StripsComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonc")
	if err := os.WriteFile(path, []byte("{\n // c\n \"a\": 1,\n}"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	data, err := ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var m map[string]int
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal stripped data: %v", err)
	}
	if m["a"] != 1 {
		t.Errorf("a = %d, want 1", m["a"])
	}
}

func TestWriteFile_Atomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.jsonc")

	data := map[string]interface{}{"id": "test-id", "body": "hello"}
	if err := WriteFile(path, data); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(b) == 0 {
		t.Error("expected non-empty file")
	}

	var result map[string]interface{}
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("unmarshal written file: %v", err)
	}
	if result["id"] != "test-id" {
		t.Errorf("id = %v, want test-id", result["id"])
	}

	// No leftover temp files.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".wyrd-") && strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
}

func TestWriteFile_NoHTMLEscaping(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.jsonc")

	data := map[string]interface{}{"body": "a && b < c"}
	if err := WriteFile(path, data); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(b), "b < c") {
		t.Errorf("expected literal '<' (no HTML-escaping) in output: %s", b)
	}
}
