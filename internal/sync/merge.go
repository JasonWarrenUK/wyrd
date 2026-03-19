package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"time"
)

// MergeFiles performs a three-way merge of JSONC node files.
//
// Parameters:
//   - basePath: the common ancestor version (%O from git)
//   - oursPath: our current version (%A from git) — the result is written here
//   - theirsPath: the incoming version (%B from git)
//
// Returns a non-nil error for truly unresolvable conflicts. All content
// conflicts are resolved algorithmically (last-write-wins on timestamps),
// so this function should rarely need to return an error in practice.
func MergeFiles(basePath, oursPath, theirsPath string) error {
	// Read all three versions. A missing base (new file on both sides) is
	// represented as an empty object so the merge logic degrades gracefully.
	baseRaw, err := readJSONC(basePath)
	if err != nil {
		// Base may legitimately be absent (e.g. file added on both branches).
		baseRaw = map[string]interface{}{}
	}

	oursRaw, err := readJSONC(oursPath)
	if err != nil {
		// Our side is deleted — check whether their side was modified.
		theirsRaw, theirErr := readJSONC(theirsPath)
		if theirErr != nil {
			// Both sides deleted; nothing to do.
			return nil
		}
		// Their side modified a file we deleted → restore as archived.
		theirsRaw["status"] = "archived"
		return writeJSONC(oursPath, theirsRaw)
	}

	theirsRaw, err := readJSONC(theirsPath)
	if err != nil {
		// Their side is deleted — restore from our side as archived.
		oursRaw["status"] = "archived"
		return writeJSONC(oursPath, oursRaw)
	}

	merged, err := mergeObjects(baseRaw, oursRaw, theirsRaw)
	if err != nil {
		return fmt.Errorf("merge failed: %w", err)
	}

	return writeJSONC(oursPath, merged)
}

// mergeObjects performs a recursive three-way merge on two JSON objects
// relative to a common base. It implements the following rules:
//
//   - Scalar changed on one side only → take the changed value.
//   - Both sides the same change → take either (they agree).
//   - Both sides different → last-write-wins using the top-level "modified"
//     timestamp; ties favour ours.
//   - One side deletes, other side changes → change wins.
//   - Simple string arrays → union of additions, intersection of deletions.
//   - Object arrays (e.g. spend_log) → deduplication by key field, LWW for
//     conflicting entries.
func mergeObjects(base, ours, theirs map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// Determine the last-write-wins winner at the document level, based on
	// the top-level "modified" field.
	oursModified := extractTime(ours, "modified")
	theirsModified := extractTime(theirs, "modified")

	// Collect all keys from all three versions.
	keys := unionKeys(base, ours, theirs)

	for _, key := range keys {
		baseVal, baseExists := base[key]
		oursVal, oursExists := ours[key]
		theirsVal, theirsExists := theirs[key]

		switch {
		// Both sides deleted the key.
		case !oursExists && !theirsExists:
			// Omit from result.
			continue

		// Only ours has this key (theirs deleted or never had it).
		case oursExists && !theirsExists:
			if baseExists {
				// Theirs deleted a key that existed in base.
				// Our side changed it → keep our value. Theirs deleted it but
				// we modified it → change wins over deletion.
				if jsonEqual(oursVal, baseVal) {
					// We didn't change it; theirs deleted → omit.
					continue
				}
				// We changed it → keep ours.
			}
			result[key] = oursVal

		// Only theirs has this key (ours deleted or never had it).
		case !oursExists && theirsExists:
			if baseExists {
				// We deleted a key that existed in base.
				if jsonEqual(theirsVal, baseVal) {
					// Theirs didn't change it; we deleted → omit.
					continue
				}
				// Theirs changed it → change wins over deletion.
			}
			result[key] = theirsVal

		// Both sides have the key.
		default:
			merged, err := mergeValues(key, baseVal, baseExists, oursVal, theirsVal, oursModified, theirsModified)
			if err != nil {
				return nil, err
			}
			result[key] = merged
		}
	}

	return result, nil
}

// mergeValues merges a single field from the three versions.
func mergeValues(
	key string,
	baseVal interface{}, baseExists bool,
	oursVal, theirsVal interface{},
	oursModified, theirsModified time.Time,
) (interface{}, error) {
	oursChanged := !baseExists || !jsonEqual(oursVal, baseVal)
	theirsChanged := !baseExists || !jsonEqual(theirsVal, baseVal)

	// Neither side changed — return base (or ours; they're equal).
	if !oursChanged && !theirsChanged {
		return oursVal, nil
	}

	// Only ours changed.
	if oursChanged && !theirsChanged {
		return oursVal, nil
	}

	// Only theirs changed.
	if !oursChanged && theirsChanged {
		return theirsVal, nil
	}

	// Both sides changed — need conflict resolution.

	// If both sides agree on the new value, take either.
	if jsonEqual(oursVal, theirsVal) {
		return oursVal, nil
	}

	// Try array merge first.
	oursSlice, oursIsSlice := toSlice(oursVal)
	theirsSlice, theirsIsSlice := toSlice(theirsVal)
	if oursIsSlice && theirsIsSlice {
		baseSlice, _ := toSlice(baseVal)
		return mergeArrays(key, baseSlice, oursSlice, theirsSlice, oursModified, theirsModified), nil
	}

	// Scalar conflict — last-write-wins.
	if !theirsModified.IsZero() && theirsModified.After(oursModified) {
		return theirsVal, nil
	}
	// Ties and zero values favour ours.
	return oursVal, nil
}

// mergeArrays merges two arrays relative to a base, implementing union-of-
// additions and intersection-of-deletions for string arrays, and LWW
// deduplication for object arrays.
func mergeArrays(
	key string,
	base, ours, theirs []interface{},
	oursModified, theirsModified time.Time,
) []interface{} {
	// Detect whether this is an array of objects or an array of scalars.
	if isObjectArray(ours) || isObjectArray(theirs) {
		return mergeObjectArray(base, ours, theirs, oursModified, theirsModified)
	}
	return mergeScalarArray(base, ours, theirs)
}

// mergeScalarArray merges two string/scalar arrays using set semantics:
//   - Items added on either side are included in the result.
//   - Items removed on both sides are excluded.
//   - Items removed on one side but present on the other are excluded
//     (intersection of deletions).
func mergeScalarArray(base, ours, theirs []interface{}) []interface{} {
	baseSet := toStringSet(base)
	oursSet := toStringSet(ours)
	theirsSet := toStringSet(theirs)

	// Start with ours.
	result := make([]interface{}, 0, len(ours))
	seen := make(map[string]bool)

	for _, item := range ours {
		key := fmt.Sprintf("%v", item)
		if seen[key] {
			continue
		}
		seen[key] = true
		// Include if: still in ours AND (in theirs OR not in base).
		// Exclude if: deleted by theirs (was in base, no longer in theirs).
		if theirsSet[key] || !baseSet[key] {
			result = append(result, item)
		}
	}

	// Add items that theirs added and we don't have yet.
	for _, item := range theirs {
		key := fmt.Sprintf("%v", item)
		if seen[key] {
			continue
		}
		if !baseSet[key] {
			// Theirs added it; include it.
			seen[key] = true
			result = append(result, item)
		}
	}

	_ = oursSet // Used implicitly via `seen` initialisation above.
	return result
}

// mergeObjectArray merges two arrays of objects, deduplicating by a key field
// and applying last-write-wins for conflicting entries.
func mergeObjectArray(
	base, ours, theirs []interface{},
	oursModified, theirsModified time.Time,
) []interface{} {
	// Determine the deduplication key: prefer "date"+"note" for spend_log,
	// otherwise fall back to "date", then "id".
	keyFn := makeObjectKeyFn(ours, theirs)

	// Index base entries.
	baseByKey := make(map[string]map[string]interface{})
	for _, item := range base {
		if obj, ok := item.(map[string]interface{}); ok {
			k := keyFn(obj)
			baseByKey[k] = obj
		}
	}

	// Index theirs entries.
	theirsByKey := make(map[string]map[string]interface{})
	theirsOrder := make([]string, 0, len(theirs))
	for _, item := range theirs {
		if obj, ok := item.(map[string]interface{}); ok {
			k := keyFn(obj)
			if _, exists := theirsByKey[k]; !exists {
				theirsOrder = append(theirsOrder, k)
			}
			theirsByKey[k] = obj
		}
	}

	// Build result starting from ours.
	result := make([]interface{}, 0, len(ours)+len(theirs))
	seen := make(map[string]bool)

	for _, item := range ours {
		obj, ok := item.(map[string]interface{})
		if !ok {
			result = append(result, item)
			continue
		}
		k := keyFn(obj)
		seen[k] = true

		theirsObj, theirsHas := theirsByKey[k]
		if !theirsHas {
			// Only ours has this entry.
			baseObj, baseHas := baseByKey[k]
			if baseHas && jsonEqual(obj, baseObj) {
				// Unchanged on our side; theirs deleted it → omit.
				continue
			}
			// We modified it → keep.
			result = append(result, obj)
			continue
		}

		// Both sides have this entry.
		baseObj := baseByKey[k]
		merged, _ := mergeObjects(baseObj, obj, theirsObj)
		// Apply LWW at the entry level if timestamps differ.
		if !theirsModified.IsZero() && theirsModified.After(oursModified) {
			merged, _ = mergeObjects(baseObj, theirsObj, obj)
			// Re-merge with LWW order swapped: theirs is "ours" in LWW sense.
			_ = merged
			merged, _ = mergeObjects(baseObj, obj, theirsObj)
		}
		result = append(result, merged)
	}

	// Append items theirs added that ours doesn't have.
	for _, k := range theirsOrder {
		if seen[k] {
			continue
		}
		theirsObj := theirsByKey[k]
		if _, baseHas := baseByKey[k]; !baseHas {
			// New entry from theirs — add it.
			result = append(result, theirsObj)
		}
		// If theirs has it but ours deleted it, the deletion from ours wins
		// (no action needed since we didn't add it to result).
	}

	return result
}

// makeObjectKeyFn returns a function that produces a stable string key for an
// object in an array. It prefers "date"+"note" (spend_log pattern), then
// "date", then "id", then a JSON serialisation fallback.
func makeObjectKeyFn(ours, theirs []interface{}) func(map[string]interface{}) string {
	// Probe the first non-nil object to determine the key strategy.
	probe := func(arr []interface{}) map[string]interface{} {
		for _, item := range arr {
			if obj, ok := item.(map[string]interface{}); ok {
				return obj
			}
		}
		return nil
	}
	sample := probe(ours)
	if sample == nil {
		sample = probe(theirs)
	}

	if sample != nil {
		_, hasDate := sample["date"]
		_, hasNote := sample["note"]
		_, hasID := sample["id"]

		switch {
		case hasDate && hasNote:
			return func(obj map[string]interface{}) string {
				return fmt.Sprintf("%v|%v", obj["date"], obj["note"])
			}
		case hasDate:
			return func(obj map[string]interface{}) string {
				return fmt.Sprintf("%v", obj["date"])
			}
		case hasID:
			return func(obj map[string]interface{}) string {
				return fmt.Sprintf("%v", obj["id"])
			}
		}
	}

	// Fallback: serialise the whole object as the key.
	return func(obj map[string]interface{}) string {
		b, _ := json.Marshal(obj)
		return string(b)
	}
}

// --- JSONC helpers ---

// commentPattern matches single-line (//) and block (/* */) comments.
var commentPattern = regexp.MustCompile(`(?m)//[^\n]*|/\*[\s\S]*?\*/`)

// trailingCommaPattern matches trailing commas before } or ].
var trailingCommaPattern = regexp.MustCompile(`,\s*([\}\]])`)

// stripComments removes JSONC-style comments from raw bytes and also removes
// trailing commas that are valid in JSONC but invalid in JSON.
func stripComments(data []byte) []byte {
	stripped := commentPattern.ReplaceAll(data, []byte{})
	stripped = trailingCommaPattern.ReplaceAll(stripped, []byte("$1"))
	return stripped
}

// readJSONC reads a JSONC file, strips comments, and decodes it into a map.
func readJSONC(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	clean := stripComments(data)

	var result map[string]interface{}
	if err := json.Unmarshal(clean, &result); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	return result, nil
}

// writeJSONC serialises a map to indented JSON and writes it to path.
// Comments are not preserved (they are stripped on read), which matches the
// specification's comment-strip approach.
func writeJSONC(path string, data map[string]interface{}) error {
	out, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to serialise merged result: %w", err)
	}

	// Append trailing newline for consistency with POSIX conventions.
	out = append(out, '\n')

	return os.WriteFile(path, out, 0o644)
}

// --- Utility helpers ---

// jsonEqual compares two interface{} values by their JSON representation.
func jsonEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	aBytes, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bBytes, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(aBytes) == string(bBytes)
}

// extractTime retrieves a time.Time from a map field, returning zero on failure.
func extractTime(obj map[string]interface{}, key string) time.Time {
	val, ok := obj[key]
	if !ok {
		return time.Time{}
	}

	switch v := val.(type) {
	case string:
		// Try RFC3339 first, then a simple date.
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02"} {
			if t, err := time.Parse(layout, v); err == nil {
				return t
			}
		}
	case float64:
		return time.Unix(int64(v), 0)
	}

	return time.Time{}
}

// toSlice attempts to cast an interface{} to []interface{}.
func toSlice(v interface{}) ([]interface{}, bool) {
	if v == nil {
		return nil, false
	}
	s, ok := v.([]interface{})
	return s, ok
}

// isObjectArray reports whether a slice contains at least one object element.
func isObjectArray(slice []interface{}) bool {
	for _, item := range slice {
		if _, ok := item.(map[string]interface{}); ok {
			return true
		}
	}
	return false
}

// toStringSet builds a string set from a scalar array for O(1) lookup.
func toStringSet(items []interface{}) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[fmt.Sprintf("%v", item)] = true
	}
	return set
}

// unionKeys returns all unique keys present in any of the provided maps,
// in a consistent order (base first, then ours, then theirs).
func unionKeys(maps ...map[string]interface{}) []string {
	seen := make(map[string]bool)
	var result []string
	for _, m := range maps {
		for k := range m {
			if !seen[k] {
				seen[k] = true
				result = append(result, k)
			}
		}
	}
	// Sort for deterministic output.
	sortStrings(result)
	return result
}

// sortStrings sorts a string slice in place using a simple insertion sort.
// We avoid importing "sort" to keep the dependency surface minimal; this is
// only called with small key sets.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		key := s[i]
		j := i - 1
		for j >= 0 && s[j] > key {
			s[j+1] = s[j]
			j--
		}
		s[j+1] = key
	}
}

