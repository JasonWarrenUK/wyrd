package query

import (
	"fmt"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// resolveBuiltin resolves a BuiltinVariable to a concrete time.Time using the
// supplied clock.  Date arithmetic offsets (e.g. $today + 7d) are applied after
// the base value is established.
func resolveBuiltin(v *BuiltinVariable, clock types.Clock) (time.Time, error) {
	now := clock.Now()
	var base time.Time

	switch v.Name {
	case "now":
		base = now

	case "today":
		base = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	case "week_start":
		// ISO week starts on Monday.
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7 // Sunday → 7 so Monday = 1
		}
		daysToMonday := weekday - 1
		d := now.AddDate(0, 0, -daysToMonday)
		base = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())

	case "month_start":
		base = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	default:
		return time.Time{}, fmt.Errorf("unknown built-in variable $%s", v.Name)
	}

	if v.Offset == nil {
		return base, nil
	}

	return applyDateOffset(base, v.Offset)
}

// applyDateOffset adds or subtracts the specified duration from base.
func applyDateOffset(base time.Time, offset *DateOffset) (time.Time, error) {
	sign := 1
	if offset.Sign == "-" {
		sign = -1
	}
	amount := sign * offset.Amount

	switch offset.Unit {
	case "d":
		return base.AddDate(0, 0, amount), nil
	case "w":
		return base.AddDate(0, 0, amount*7), nil
	case "m":
		return base.AddDate(0, amount, 0), nil
	case "y":
		return base.AddDate(amount, 0, 0), nil
	default:
		return time.Time{}, fmt.Errorf("unknown date offset unit %q; expected d, w, m, or y", offset.Unit)
	}
}
