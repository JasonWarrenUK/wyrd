package ritual_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/tui/ritual"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// writeRitual marshals r to JSONC and writes it to {dir}/rituals/{name}.jsonc.
func writeRitual(t *testing.T, dir string, name string, r *types.Ritual) {
	t.Helper()

	ritualsDir := filepath.Join(dir, "rituals")
	if err := os.MkdirAll(ritualsDir, 0o755); err != nil {
		t.Fatalf("creating rituals dir: %v", err)
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshalling ritual: %v", err)
	}

	path := filepath.Join(ritualsDir, name+".jsonc")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing ritual file: %v", err)
	}
}

func TestLoadRituals_Empty(t *testing.T) {
	dir := t.TempDir()
	rituals, err := ritual.LoadRituals(dir)
	if err != nil {
		t.Fatalf("expected no error for missing rituals dir, got: %v", err)
	}
	if len(rituals) != 0 {
		t.Errorf("expected 0 rituals, got %d", len(rituals))
	}
}

func TestLoadRituals_SingleRitual(t *testing.T) {
	dir := t.TempDir()

	r := &types.Ritual{
		Name:     "morning",
		Friction: types.FrictionGate,
		Schedule: types.RitualSchedule{
			Days: []string{"mon", "tue", "wed", "thu", "fri"},
			Time: "09:00",
		},
		Steps: []types.RitualStep{
			{
				Type:     types.StepQuerySummary,
				Label:    "Overview",
				Query:    "MATCH (n) RETURN count(n) AS total",
				Template: "You have {{total}} nodes.",
			},
		},
	}
	writeRitual(t, dir, "morning", r)

	loaded, err := ritual.LoadRituals(dir)
	if err != nil {
		t.Fatalf("LoadRituals: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 ritual, got %d", len(loaded))
	}
	if loaded[0].Name != "morning" {
		t.Errorf("expected name %q, got %q", "morning", loaded[0].Name)
	}
	if loaded[0].Friction != types.FrictionGate {
		t.Errorf("expected friction gate, got %q", loaded[0].Friction)
	}
	if len(loaded[0].Steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(loaded[0].Steps))
	}
}

func TestLoadRituals_WithJSONCComment(t *testing.T) {
	dir := t.TempDir()

	ritualsDir := filepath.Join(dir, "rituals")
	if err := os.MkdirAll(ritualsDir, 0o755); err != nil {
		t.Fatalf("creating rituals dir: %v", err)
	}

	jsoncContent := `{
		// Morning ritual
		"name": "morning",
		"friction": "nudge",
		"schedule": {
			/* runs on weekdays */
			"days": ["mon", "tue"],
			"time": "08:00"
		},
		"steps": []
	}`

	if err := os.WriteFile(filepath.Join(ritualsDir, "morning.jsonc"), []byte(jsoncContent), 0o644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	loaded, err := ritual.LoadRituals(dir)
	if err != nil {
		t.Fatalf("LoadRituals with JSONC comments: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 ritual, got %d", len(loaded))
	}
	if loaded[0].Friction != types.FrictionNudge {
		t.Errorf("expected nudge friction, got %q", loaded[0].Friction)
	}
}

// ---- Schedule matching tests ------------------------------------------------

// monday09h is a convenient fixed weekday Monday at 09:00.
var monday09h = time.Date(2025, 3, 17, 9, 0, 0, 0, time.UTC) // Monday

func stubClock(t time.Time) types.Clock {
	return types.StubClock{Fixed: t}
}

func TestDueRituals_WeekdayMatch(t *testing.T) {
	rituals := []*types.Ritual{
		{
			Name: "morning",
			Schedule: types.RitualSchedule{
				Days: []string{"mon", "tue", "wed", "thu", "fri"},
				Time: "09:00",
			},
		},
	}

	// Monday at 09:00 — should trigger.
	due := ritual.DueRituals(rituals, stubClock(monday09h))
	if len(due) != 1 {
		t.Errorf("expected 1 due ritual on Monday 09:00, got %d", len(due))
	}
}

func TestDueRituals_TooEarly(t *testing.T) {
	rituals := []*types.Ritual{
		{
			Name: "morning",
			Schedule: types.RitualSchedule{
				Days: []string{"mon"},
				Time: "09:00",
			},
		},
	}

	// Monday at 08:59 — not yet due.
	early := monday09h.Add(-1 * time.Minute)
	due := ritual.DueRituals(rituals, stubClock(early))
	if len(due) != 0 {
		t.Errorf("expected 0 due rituals before schedule time, got %d", len(due))
	}
}

func TestDueRituals_WrongDay(t *testing.T) {
	rituals := []*types.Ritual{
		{
			Name: "weekly-review",
			Schedule: types.RitualSchedule{
				Days: []string{"sun"},
				Time: "09:00",
			},
		},
	}

	// Monday — wrong day.
	due := ritual.DueRituals(rituals, stubClock(monday09h))
	if len(due) != 0 {
		t.Errorf("expected 0 due rituals on Monday for sunday schedule, got %d", len(due))
	}
}

func TestDueRituals_SingleDayField(t *testing.T) {
	// Tests the Day (singular) schedule format.
	sunday09h := time.Date(2025, 3, 16, 9, 0, 0, 0, time.UTC) // Sunday

	rituals := []*types.Ritual{
		{
			Name: "weekly-review",
			Schedule: types.RitualSchedule{
				Day:  "sun",
				Time: "09:00",
			},
		},
	}

	due := ritual.DueRituals(rituals, stubClock(sunday09h))
	if len(due) != 1 {
		t.Errorf("expected 1 due ritual for Day=sun on Sunday, got %d", len(due))
	}
}

func TestDueRituals_Daily(t *testing.T) {
	allDays := []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

	rituals := []*types.Ritual{
		{
			Name: "evening",
			Schedule: types.RitualSchedule{
				Days: allDays,
				Time: "21:00",
			},
		},
	}

	// Wednesday 21:05 — daily ritual should be due.
	wednesday21h := time.Date(2025, 3, 19, 21, 5, 0, 0, time.UTC)
	due := ritual.DueRituals(rituals, stubClock(wednesday21h))
	if len(due) != 1 {
		t.Errorf("expected 1 due ritual for daily evening on Wed 21:05, got %d", len(due))
	}
}

func TestDueRituals_MultipleRituals(t *testing.T) {
	rituals := []*types.Ritual{
		{
			Name:     "morning",
			Schedule: types.RitualSchedule{Days: []string{"mon"}, Time: "09:00"},
		},
		{
			Name:     "evening",
			Schedule: types.RitualSchedule{Days: []string{"mon"}, Time: "21:00"},
		},
	}

	// Monday at 10:00 — only morning should be due.
	monday10h := monday09h.Add(time.Hour)
	due := ritual.DueRituals(rituals, stubClock(monday10h))
	if len(due) != 1 {
		t.Fatalf("expected 1 due ritual, got %d", len(due))
	}
	if due[0].Name != "morning" {
		t.Errorf("expected morning ritual, got %q", due[0].Name)
	}
}
