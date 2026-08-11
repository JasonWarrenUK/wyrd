package stage_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/stage"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// hashKindLikeProvenance reimplements the hashing algorithm inline (zero
// ShadowOf, marshal, sha256, hex, truncate to 16, prefix "sha256:") so
// TestDefaultKindHashDiffersOnFieldChange fails loudly if the production
// algorithm ever silently changes shape rather than passing vacuously.
func hashKindLikeProvenance(k types.Kind) string {
	k.ShadowOf = ""
	data, err := json.Marshal(k)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	full := hex.EncodeToString(sum[:])
	return "sha256:" + full[:16]
}

// TestDefaultKindHashStable verifies repeated calls for the same name return
// the same non-empty, correctly prefixed value.
func TestDefaultKindHashStable(t *testing.T) {
	h1 := stage.DefaultKindHash("Task")
	h2 := stage.DefaultKindHash("Task")

	if h1 == "" {
		t.Fatal("expected a non-empty hash for the built-in Task kind")
	}
	if h1 != h2 {
		t.Errorf("hash not stable across calls: %q != %q", h1, h2)
	}
	if !strings.HasPrefix(h1, "sha256:") {
		t.Errorf("hash %q missing sha256: prefix", h1)
	}
}

// TestDefaultKindHashDiffersOnFieldChange verifies the hash reflects the
// entry's content: mutating a field (via the reimplemented algorithm above)
// produces a different value than the production function's result for the
// unmutated default.
func TestDefaultKindHashDiffersOnFieldChange(t *testing.T) {
	defaults, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds: %v", err)
	}
	var task types.Kind
	for _, k := range defaults {
		if k.Name == "Task" {
			task = k
			break
		}
	}
	if task.Name == "" {
		t.Fatal("precondition failed: no built-in Task kind found")
	}

	unmutated := hashKindLikeProvenance(task)
	if unmutated != stage.DefaultKindHash("Task") {
		t.Fatalf("reimplemented hash %q does not match production hash %q for the unmutated default — algorithm mismatch", unmutated, stage.DefaultKindHash("Task"))
	}

	mutated := task
	mutated.Colour = mutated.Colour + "ff" // any field change
	mutatedHash := hashKindLikeProvenance(mutated)

	if mutatedHash == unmutated {
		t.Error("hash did not change after mutating a field")
	}
}

// TestDefaultKindHashIgnoresShadowOf verifies a copy of the default carrying
// a ShadowOf value hashes identically to the pristine default — locks in
// that ShadowOf is zeroed before hashing.
func TestDefaultKindHashIgnoresShadowOf(t *testing.T) {
	defaults, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds: %v", err)
	}
	var task types.Kind
	for _, k := range defaults {
		if k.Name == "Task" {
			task = k
			break
		}
	}
	if task.Name == "" {
		t.Fatal("precondition failed: no built-in Task kind found")
	}

	withShadow := task
	withShadow.ShadowOf = "sha256:doesnotmatterxx"

	if got, want := hashKindLikeProvenance(withShadow), stage.DefaultKindHash("Task"); got != want {
		t.Errorf("hash with ShadowOf set = %q, want %q (ShadowOf must be zeroed before hashing)", got, want)
	}
}

// TestDefaultKindHashUnknownNameEmpty verifies a name with no matching
// built-in default returns "".
func TestDefaultKindHashUnknownNameEmpty(t *testing.T) {
	if got := stage.DefaultKindHash("NoSuchKind"); got != "" {
		t.Errorf("DefaultKindHash(unknown) = %q, want empty", got)
	}
}

// TestDefaultKindHashDiffersBetweenKinds guards against a hash function that
// ignores its input and returns a constant.
func TestDefaultKindHashDiffersBetweenKinds(t *testing.T) {
	task := stage.DefaultKindHash("Task")
	goblin := stage.DefaultKindHash("Goblin")

	if task == "" || goblin == "" {
		t.Fatal("precondition failed: both Task and Goblin should be built-in kinds")
	}
	if task == goblin {
		t.Error("Task and Goblin hashed to the same value")
	}
}

// ---------------------------------------------------------------------------
// DefaultStageGroupHash — mirrors the Kind tests above.
// ---------------------------------------------------------------------------

func hashStageGroupLikeProvenance(g types.StageGroup) string {
	g.ShadowOf = ""
	data, err := json.Marshal(g)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	full := hex.EncodeToString(sum[:])
	return "sha256:" + full[:16]
}

func TestDefaultStageGroupHashStable(t *testing.T) {
	h1 := stage.DefaultStageGroupHash("task-flow")
	h2 := stage.DefaultStageGroupHash("task-flow")

	if h1 == "" {
		t.Fatal("expected a non-empty hash for the built-in task-flow group")
	}
	if h1 != h2 {
		t.Errorf("hash not stable across calls: %q != %q", h1, h2)
	}
	if !strings.HasPrefix(h1, "sha256:") {
		t.Errorf("hash %q missing sha256: prefix", h1)
	}
}

func TestDefaultStageGroupHashDiffersOnFieldChange(t *testing.T) {
	defaults, err := stage.DefaultStageGroups()
	if err != nil {
		t.Fatalf("DefaultStageGroups: %v", err)
	}
	var taskFlow types.StageGroup
	for _, g := range defaults {
		if g.Name == "task-flow" {
			taskFlow = g
			break
		}
	}
	if taskFlow.Name == "" {
		t.Fatal("precondition failed: no built-in task-flow group found")
	}

	unmutated := hashStageGroupLikeProvenance(taskFlow)
	if unmutated != stage.DefaultStageGroupHash("task-flow") {
		t.Fatalf("reimplemented hash %q does not match production hash %q for the unmutated default — algorithm mismatch", unmutated, stage.DefaultStageGroupHash("task-flow"))
	}

	mutated := taskFlow
	mutated.Stages = append(append([]string{}, taskFlow.Stages...), "Extra")
	mutatedHash := hashStageGroupLikeProvenance(mutated)

	if mutatedHash == unmutated {
		t.Error("hash did not change after mutating Stages")
	}
}

func TestDefaultStageGroupHashIgnoresShadowOf(t *testing.T) {
	defaults, err := stage.DefaultStageGroups()
	if err != nil {
		t.Fatalf("DefaultStageGroups: %v", err)
	}
	var taskFlow types.StageGroup
	for _, g := range defaults {
		if g.Name == "task-flow" {
			taskFlow = g
			break
		}
	}
	if taskFlow.Name == "" {
		t.Fatal("precondition failed: no built-in task-flow group found")
	}

	withShadow := taskFlow
	withShadow.ShadowOf = "sha256:doesnotmatterxx"

	if got, want := hashStageGroupLikeProvenance(withShadow), stage.DefaultStageGroupHash("task-flow"); got != want {
		t.Errorf("hash with ShadowOf set = %q, want %q (ShadowOf must be zeroed before hashing)", got, want)
	}
}

func TestDefaultStageGroupHashUnknownNameEmpty(t *testing.T) {
	if got := stage.DefaultStageGroupHash("no-such-flow"); got != "" {
		t.Errorf("DefaultStageGroupHash(unknown) = %q, want empty", got)
	}
}

func TestDefaultStageGroupHashDiffersBetweenGroups(t *testing.T) {
	taskFlow := stage.DefaultStageGroupHash("task-flow")
	eventFlow := stage.DefaultStageGroupHash("event-flow")

	if taskFlow == "" || eventFlow == "" {
		t.Fatal("precondition failed: both task-flow and event-flow should be built-in groups")
	}
	if taskFlow == eventFlow {
		t.Error("task-flow and event-flow hashed to the same value")
	}
}
