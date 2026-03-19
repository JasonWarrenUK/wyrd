package types

import "time"

// Clock is an injectable time source used throughout Wyrd.
// Using an interface rather than calling time.Now() directly allows
// tests to control the current time without patching globals.
type Clock interface {
	// Now returns the current time.
	Now() time.Time
}

// RealClock returns actual wall-clock time. Used in production.
type RealClock struct{}

// Now returns time.Now().
func (RealClock) Now() time.Time {
	return time.Now()
}

// StubClock returns a fixed time. Used in tests.
type StubClock struct {
	Fixed time.Time
}

// Now returns the fixed time.
func (s StubClock) Now() time.Time {
	return s.Fixed
}
