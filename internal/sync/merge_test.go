package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- helpers ---

// writeTempJSONC writes a map as a JSONC file in a temp directory.
func writeTempJSONC(t *testing.T, dir, name string, data map[string]interface{}) string {
	t.Helper()
	path := filepath.Join(dir, name)
	b, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		t.Fatalf("marshal %s: %v", name, err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// readResultJSONC reads the merged output file and returns the parsed map.
func readResultJSONC(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(stripComments(data), &m); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	return m
}

// iso returns an RFC3339 timestamp string offset from a reference time.
func iso(base time.Time, delta time.Duration) string {
	return base.Add(delta).UTC().Format(time.RFC3339Nano)
}

// --- stripComments tests ---

func TestStripComments_LineComment(t *testing.T) {
	input := []byte(`{
	// This is a comment
	"key": "value"
}`)
	result := stripComments(input)
	var m map[string]interface{}
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatalf("unmarshal after strip: %v", err)
	}
	if m["key"] != "value" {
		t.Errorf("expected key=value, got %v", m["key"])
	}
}

func TestStripComments_BlockComment(t *testing.T) {
	input := []byte(`{
	/* block comment */
	"key": "value"
}`)
	result := stripComments(input)
	var m map[string]interface{}
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatalf("unmarshal after strip: %v", err)
	}
	if m["key"] != "value" {
		t.Errorf("expected key=value, got %v", m["key"])
	}
}

func TestStripComments_TrailingComma(t *testing.T) {
	input := []byte(`{
	"a": 1,
	"b": 2,
}`)
	result := stripComments(input)
	var m map[string]interface{}
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatalf("unmarshal after strip trailing comma: %v", err)
	}
	if m["b"] == nil {
		t.Errorf("expected b field present")
	}
}

// --- mergeObjects scalar tests ---

func TestMergeObjects_OnlyOursChanged(t *testing.T) {
	base := map[string]interface{}{"title": "original", "status": "active"}
	ours := map[string]interface{}{"title": "updated by us", "status": "active"}
	theirs := map[string]interface{}{"title": "original", "status": "active"}

	result, err := mergeObjects(base, ours, theirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["title"] != "updated by us" {
		t.Errorf("expected 'updated by us', got %v", result["title"])
	}
}

func TestMergeObjects_OnlyTheirsChanged(t *testing.T) {
	base := map[string]interface{}{"title": "original"}
	ours := map[string]interface{}{"title": "original"}
	theirs := map[string]interface{}{"title": "updated by them"}

	result, err := mergeObjects(base, ours, theirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["title"] != "updated by them" {
		t.Errorf("expected 'updated by them', got %v", result["title"])
	}
}

func TestMergeObjects_BothChangedSameValue(t *testing.T) {
	base := map[string]interface{}{"status": "active"}
	ours := map[string]interface{}{"status": "done"}
	theirs := map[string]interface{}{"status": "done"}

	result, err := mergeObjects(base, ours, theirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["status"] != "done" {
		t.Errorf("expected 'done', got %v", result["status"])
	}
}

func TestMergeObjects_BothChangedDifferent_LWWFavoursTheirs(t *testing.T) {
	now := time.Now().UTC()
	base := map[string]interface{}{
		"title":    "original",
		"modified": iso(now, -2*time.Hour),
	}
	ours := map[string]interface{}{
		"title":    "our change",
		"modified": iso(now, -1*time.Hour), // older
	}
	theirs := map[string]interface{}{
		"title":    "their change",
		"modified": iso(now, 0), // newer
	}

	result, err := mergeObjects(base, ours, theirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["title"] != "their change" {
		t.Errorf("LWW should favour theirs (newer modified); got %v", result["title"])
	}
}

func TestMergeObjects_BothChangedDifferent_LWWFavoursOurs(t *testing.T) {
	now := time.Now().UTC()
	base := map[string]interface{}{
		"title":    "original",
		"modified": iso(now, -2*time.Hour),
	}
	ours := map[string]interface{}{
		"title":    "our change",
		"modified": iso(now, 0), // newer
	}
	theirs := map[string]interface{}{
		"title":    "their change",
		"modified": iso(now, -1*time.Hour), // older
	}

	result, err := mergeObjects(base, ours, theirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["title"] != "our change" {
		t.Errorf("LWW should favour ours (newer modified); got %v", result["title"])
	}
}

func TestMergeObjects_LWWTiesFavourOurs(t *testing.T) {
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	base := map[string]interface{}{"title": "original", "modified": ts}
	ours := map[string]interface{}{"title": "our change", "modified": ts}
	theirs := map[string]interface{}{"title": "their change", "modified": ts}

	result, err := mergeObjects(base, ours, theirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["title"] != "our change" {
		t.Errorf("tied timestamps should favour ours; got %v", result["title"])
	}
}

func TestMergeObjects_DeletedOnOursSide_TheirsChanged(t *testing.T) {
	// Theirs changed a field we deleted → change wins.
	base := map[string]interface{}{"tag": "old-tag"}
	ours := map[string]interface{}{}          // deleted "tag"
	theirs := map[string]interface{}{"tag": "new-tag"} // modified "tag"

	result, err := mergeObjects(base, ours, theirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["tag"] != "new-tag" {
		t.Errorf("change should win over deletion; got %v", result["tag"])
	}
}

func TestMergeObjects_DeletedOnTheirsSide_OursChanged(t *testing.T) {
	// We changed a field theirs deleted → change wins.
	base := map[string]interface{}{"tag": "old-tag"}
	ours := map[string]interface{}{"tag": "new-tag"}   // modified "tag"
	theirs := map[string]interface{}{}                  // deleted "tag"

	result, err := mergeObjects(base, ours, theirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["tag"] != "new-tag" {
		t.Errorf("change should win over deletion; got %v", result["tag"])
	}
}

func TestMergeObjects_DeletedOnBothSides(t *testing.T) {
	base := map[string]interface{}{"tag": "old-tag"}
	ours := map[string]interface{}{}
	theirs := map[string]interface{}{}

	result, err := mergeObjects(base, ours, theirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, exists := result["tag"]; exists {
		t.Errorf("key deleted on both sides should not appear in result")
	}
}

// --- mergeScalarArray tests ---

func TestMergeScalarArray_UnionOfAdditions(t *testing.T) {
	base := []interface{}{"a", "b"}
	ours := []interface{}{"a", "b", "c"} // added c
	theirs := []interface{}{"a", "b", "d"} // added d

	result := mergeScalarArray(base, ours, theirs)
	resultSet := toStringSet(result)

	if !resultSet["c"] {
		t.Errorf("expected 'c' in result (ours added it)")
	}
	if !resultSet["d"] {
		t.Errorf("expected 'd' in result (theirs added it)")
	}
	if !resultSet["a"] || !resultSet["b"] {
		t.Errorf("expected original items to be preserved")
	}
}

func TestMergeScalarArray_IntersectionOfDeletions(t *testing.T) {
	base := []interface{}{"a", "b", "c"}
	ours := []interface{}{"a", "b"}   // deleted c
	theirs := []interface{}{"a", "b"} // also deleted c

	result := mergeScalarArray(base, ours, theirs)
	resultSet := toStringSet(result)

	if resultSet["c"] {
		t.Errorf("'c' deleted on both sides should not appear in result")
	}
}

func TestMergeScalarArray_OneSideDeletes(t *testing.T) {
	// One side deletes "b" — deletion wins (intersection: theirs no longer has b).
	base := []interface{}{"a", "b"}
	ours := []interface{}{"a"}        // deleted b
	theirs := []interface{}{"a", "b"} // kept b

	result := mergeScalarArray(base, ours, theirs)
	resultSet := toStringSet(result)

	// When only one side deletes, the deletion wins (intersection logic).
	if resultSet["b"] {
		t.Errorf("'b' deleted by ours should not appear — intersection-of-deletions")
	}
}

// --- mergeObjectArray tests ---

func TestMergeObjectArray_NewEntryFromTheirs(t *testing.T) {
	base := []interface{}{}
	ours := []interface{}{
		map[string]interface{}{"date": "2024-01-01", "amount": 10.0, "note": "lunch"},
	}
	theirs := []interface{}{
		map[string]interface{}{"date": "2024-01-01", "amount": 10.0, "note": "lunch"},
		map[string]interface{}{"date": "2024-01-02", "amount": 5.0, "note": "coffee"},
	}
	now := time.Now()

	result := mergeObjectArray(base, ours, theirs, now, now)
	if len(result) != 2 {
		t.Errorf("expected 2 entries, got %d", len(result))
	}
}

func TestMergeObjectArray_Deduplication(t *testing.T) {
	entry := map[string]interface{}{"date": "2024-01-01", "amount": 10.0, "note": "lunch"}
	base := []interface{}{}
	ours := []interface{}{entry}
	theirs := []interface{}{entry} // same entry on both sides

	now := time.Now()
	result := mergeObjectArray(base, ours, theirs, now, now)
	if len(result) != 1 {
		t.Errorf("duplicate entries should be deduplicated; got %d entries", len(result))
	}
}

// --- MergeFiles integration tests ---

func TestMergeFiles_BothSidesIdentical(t *testing.T) {
	dir := t.TempDir()
	data := map[string]interface{}{"id": "abc", "body": "hello", "status": "active"}

	basePath := writeTempJSONC(t, dir, "base.jsonc", data)
	oursPath := writeTempJSONC(t, dir, "ours.jsonc", data)
	theirsPath := writeTempJSONC(t, dir, "theirs.jsonc", data)

	if err := MergeFiles(basePath, oursPath, theirsPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := readResultJSONC(t, oursPath)
	if result["body"] != "hello" {
		t.Errorf("expected body='hello', got %v", result["body"])
	}
}

func TestMergeFiles_DeletedNodeRestoredAsArchived(t *testing.T) {
	// Simulate: ours deleted the file, theirs modified it.
	// The file should be restored with status="archived".
	dir := t.TempDir()

	baseData := map[string]interface{}{"id": "abc", "body": "original"}
	theirsData := map[string]interface{}{"id": "abc", "body": "updated by them"}

	basePath := writeTempJSONC(t, dir, "base.jsonc", baseData)
	oursPath := filepath.Join(dir, "ours.jsonc") // deliberately absent
	theirsPath := writeTempJSONC(t, dir, "theirs.jsonc", theirsData)

	_ = basePath // used by merge

	if err := MergeFiles(basePath, oursPath, theirsPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := readResultJSONC(t, oursPath)
	if result["status"] != "archived" {
		t.Errorf("expected status='archived', got %v", result["status"])
	}
	if result["body"] != "updated by them" {
		t.Errorf("expected body='updated by them', got %v", result["body"])
	}
}

func TestMergeFiles_TheirsDeletedNodeRestoredAsArchived(t *testing.T) {
	// Simulate: theirs deleted the file, ours modified it.
	// The file should keep ours content with status="archived".
	dir := t.TempDir()

	baseData := map[string]interface{}{"id": "abc", "body": "original"}
	oursData := map[string]interface{}{"id": "abc", "body": "updated by us"}

	basePath := writeTempJSONC(t, dir, "base.jsonc", baseData)
	oursPath := writeTempJSONC(t, dir, "ours.jsonc", oursData)
	theirsPath := filepath.Join(dir, "theirs.jsonc") // deliberately absent

	if err := MergeFiles(basePath, oursPath, theirsPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := readResultJSONC(t, oursPath)
	if result["status"] != "archived" {
		t.Errorf("expected status='archived', got %v", result["status"])
	}
}

func TestMergeFiles_JSONCWithComments(t *testing.T) {
	// Verify JSONC files with comments are parsed correctly.
	dir := t.TempDir()

	writeRaw := func(name, content string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		return path
	}

	base := writeRaw("base.jsonc", `{
	// base comment
	"id": "abc",
	"body": "original",
	"status": "active"
}`)
	ours := writeRaw("ours.jsonc", `{
	// our comment
	"id": "abc",
	"body": "our update", /* inline comment */
	"status": "active"
}`)
	theirs := writeRaw("theirs.jsonc", `{
	"id": "abc",
	"body": "original",
	"status": "active"
}`)

	if err := MergeFiles(base, ours, theirs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := readResultJSONC(t, ours)
	if result["body"] != "our update" {
		t.Errorf("expected 'our update', got %v", result["body"])
	}
}

func TestMergeFiles_SpendLogMerge(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()

	baseData := map[string]interface{}{
		"id":       "budget-1",
		"modified": iso(now, -2*time.Hour),
		"spend_log": []interface{}{
			map[string]interface{}{"date": "2024-01-01", "amount": 10.0, "note": "lunch"},
		},
	}
	oursData := map[string]interface{}{
		"id":       "budget-1",
		"modified": iso(now, -1*time.Hour),
		"spend_log": []interface{}{
			map[string]interface{}{"date": "2024-01-01", "amount": 10.0, "note": "lunch"},
			map[string]interface{}{"date": "2024-01-02", "amount": 5.0, "note": "coffee"},
		},
	}
	theirsData := map[string]interface{}{
		"id":       "budget-1",
		"modified": iso(now, -1*time.Hour),
		"spend_log": []interface{}{
			map[string]interface{}{"date": "2024-01-01", "amount": 10.0, "note": "lunch"},
			map[string]interface{}{"date": "2024-01-03", "amount": 8.0, "note": "snack"},
		},
	}

	basePath := writeTempJSONC(t, dir, "base.jsonc", baseData)
	oursPath := writeTempJSONC(t, dir, "ours.jsonc", oursData)
	theirsPath := writeTempJSONC(t, dir, "theirs.jsonc", theirsData)

	if err := MergeFiles(basePath, oursPath, theirsPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := readResultJSONC(t, oursPath)
	rawLog, ok := result["spend_log"]
	if !ok {
		t.Fatal("spend_log missing from merged result")
	}
	log, ok := rawLog.([]interface{})
	if !ok {
		t.Fatal("spend_log is not an array")
	}

	dates := make(map[string]bool)
	for _, item := range log {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if d, ok := obj["date"].(string); ok {
			dates[d] = true
		}
	}

	// Should contain all three dates.
	for _, expected := range []string{"2024-01-01", "2024-01-02", "2024-01-03"} {
		if !dates[expected] {
			t.Errorf("expected date %s in merged spend_log; got dates: %v", expected, dates)
		}
	}

	// Should not contain duplicates.
	if len(log) != 3 {
		t.Errorf("expected 3 entries (no duplicates), got %d", len(log))
	}
}

// --- pluralise tests ---

func TestPluralise(t *testing.T) {
	tests := []struct {
		word  string
		count int
		want  string
	}{
		{"node", 1, "node"},
		{"node", 2, "nodes"},
		{"node", 0, "nodes"},
		{"edge", 1, "edge"},
		{"edge", 5, "edges"},
	}
	for _, tc := range tests {
		got := pluralise(tc.word, tc.count)
		if got != tc.want {
			t.Errorf("pluralise(%q, %d) = %q; want %q", tc.word, tc.count, got, tc.want)
		}
	}
}

// --- unionKeys tests ---

func TestUnionKeys_Sorted(t *testing.T) {
	a := map[string]interface{}{"z": 1, "a": 2}
	b := map[string]interface{}{"m": 3}
	c := map[string]interface{}{"a": 4, "b": 5}

	keys := unionKeys(a, b, c)
	expected := []string{"a", "b", "m", "z"}
	if len(keys) != len(expected) {
		t.Fatalf("expected %v keys, got %v", expected, keys)
	}
	for i, k := range keys {
		if k != expected[i] {
			t.Errorf("keys[%d] = %q; want %q", i, k, expected[i])
		}
	}
}
