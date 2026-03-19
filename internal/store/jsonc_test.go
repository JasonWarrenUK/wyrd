package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStripComments(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no comments",
			input: `{"key": "value"}`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "line comment",
			input: "{\n  // this is a comment\n  \"key\": \"value\"\n}",
			want:  "{\n  \n  \"key\": \"value\"\n}",
		},
		{
			name:  "block comment",
			input: `{"key": /* comment */ "value"}`,
			want:  `{"key":  "value"}`,
		},
		{
			name:  "trailing comma",
			input: `{"key": "value",}`,
			want:  `{"key": "value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(stripComments([]byte(tt.input)))
			if got != tt.want {
				t.Errorf("stripComments() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWriteJSONCRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonc")

	data := map[string]interface{}{
		"id":   "test-id",
		"body": "hello",
	}

	if err := writeJSONC(path, data); err != nil {
		t.Fatalf("writeJSONC: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(b) == 0 {
		t.Error("expected non-empty file")
	}

	// Verify it parses back.
	parsed, err := readJSONC(path)
	if err != nil {
		t.Fatalf("readJSONC: %v", err)
	}

	var result map[string]interface{}
	if err := unmarshalJSON(parsed, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["id"] != "test-id" {
		t.Errorf("id = %v, want test-id", result["id"])
	}
}
