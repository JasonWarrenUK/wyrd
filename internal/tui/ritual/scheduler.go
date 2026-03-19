package ritual

import (
	"sync"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// SchedulerState tracks which rituals have already been presented during the
// current calendar day so they are not triggered more than once per day.
type SchedulerState struct {
	mu         sync.Mutex
	dismissed  map[string]string // ritual name → date string "YYYY-MM-DD"
}

// NewSchedulerState returns an initialised SchedulerState.
func NewSchedulerState() *SchedulerState {
	return &SchedulerState{
		dismissed: make(map[string]string),
	}
}

// Dismiss records that the named ritual was handled on the given date so it
// will not be triggered again until the following day.
func (s *SchedulerState) Dismiss(name string, date time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dismissed[name] = date.Format("2006-01-02")
}

// IsDismissed reports whether the ritual has already been dismissed today.
func (s *SchedulerState) IsDismissed(name string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.dismissed[name]
	if !ok {
		return false
	}
	return d == now.Format("2006-01-02")
}

// PendingRituals returns the rituals that are due now but have not yet been
// dismissed for the current day. It combines schedule matching (DueRituals)
// with per-day deduplication from state.
func PendingRituals(
	rituals []*types.Ritual,
	state *SchedulerState,
	clock types.Clock,
) []*types.Ritual {
	due := DueRituals(rituals, clock)
	now := clock.Now()

	var pending []*types.Ritual
	for _, r := range due {
		if !state.IsDismissed(r.Name, now) {
			pending = append(pending, r)
		}
	}
	return pending
}

// Scheduler polls for pending rituals on startup and at a regular interval.
// It calls onTrigger for each newly due ritual. Callers are responsible for
// calling Dismiss once the ritual has been presented.
type Scheduler struct {
	rituals  []*types.Ritual
	state    *SchedulerState
	clock    types.Clock
	interval time.Duration

	// onTrigger is called (possibly multiple times per tick) for each due
	// ritual. It is invoked from the scheduler's internal goroutine.
	onTrigger func(ritual *types.Ritual)

	stopCh chan struct{}
}

// NewScheduler constructs a Scheduler. interval is typically one minute.
func NewScheduler(
	rituals []*types.Ritual,
	state *SchedulerState,
	clock types.Clock,
	interval time.Duration,
	onTrigger func(*types.Ritual),
) *Scheduler {
	return &Scheduler{
		rituals:   rituals,
		state:     state,
		clock:     clock,
		interval:  interval,
		onTrigger: onTrigger,
		stopCh:    make(chan struct{}),
	}
}

// Start begins polling. It fires immediately at startup, then on each interval
// tick. Call Stop to halt the goroutine.
func (s *Scheduler) Start() {
	go func() {
		s.check()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.check()
			case <-s.stopCh:
				return
			}
		}
	}()
}

// Stop halts the scheduler goroutine.
func (s *Scheduler) Stop() {
	close(s.stopCh)
}

// check evaluates pending rituals and fires onTrigger for each.
func (s *Scheduler) check() {
	pending := PendingRituals(s.rituals, s.state, s.clock)
	for _, r := range pending {
		s.onTrigger(r)
	}
}
