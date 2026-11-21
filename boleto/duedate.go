package boleto

import (
	"fmt"
	"time"
)

// Due-date factor constants.
const (
	// FactorMin is the first factor of a cycle (2000-07-03, then 2025-02-22).
	FactorMin = 1000
	// FactorMax is the last factor of a cycle (2025-02-21, then 2049-10-13).
	FactorMax = 9999
	// FactorCycle is the number of days in one factor cycle (1000–9999).
	FactorCycle = FactorMax - FactorMin + 1

	// DueDateWindowPast and DueDateWindowFuture bound, in days around the
	// reference date, the window in which [DueDateAt] looks for the due
	// date, following FEBRABAN guidance. The window spans 8500 days,
	// shorter than a 9000-day cycle, so at most one candidate falls inside.
	DueDateWindowPast   = 3000
	DueDateWindowFuture = 5500
)

// factorBase is the base date of the due-date factor, 1997-10-07 (factor 0
// of the first cycle). The second cycle is based 9000 days later, on
// 2022-05-29, so that factor 1000 falls on 2025-02-22.
var factorBase = time.Date(1997, time.October, 7, 0, 0, 0, 0, time.UTC)

const secondsPerDay = 24 * 60 * 60

// civilDays returns the number of calendar days from factorBase to the
// calendar date of t in t's own location.
func civilDays(t time.Time) int64 {
	y, m, d := t.Date()
	return (time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() - factorBase.Unix()) / secondsPerDay
}

// FactorFor returns the due-date factor (1000–9999) that encodes the
// calendar date of date (in date's location). Dates from 2025-02-22 onwards
// use the restarted cycle. Dates before 2000-07-03 return [ErrDueDate].
func FactorFor(date time.Time) (int, error) {
	d := civilDays(date)
	if d < FactorMin {
		return 0, fmt.Errorf("%w: %s is before 2000-07-03", ErrDueDate, date.Format(time.DateOnly))
	}
	return int((d-FactorMin)%FactorCycle) + FactorMin, nil
}

// DueDateAt resolves a due-date factor to a calendar date (midnight UTC)
// using ref to pick the cycle. It reports false for factor 0 ("sem
// vencimento") and for values outside 0–9999.
//
// A factor f in 1000–9999 denotes 1997-10-07 + f + 9000·k days for some
// cycle k ≥ 0. DueDateAt picks the candidate inside the FEBRABAN window
// [ref − 3000 days, ref + 5500 days]; when no candidate falls inside (a
// 500-day gap exists between consecutive windows) it picks the candidate
// closest to ref, preferring the earlier one on ties. Legacy factors 1–999
// predate any rollover and always map to the first cycle.
//
// Only the calendar date of ref, in ref's location, is used.
func DueDateAt(factor int, ref time.Time) (time.Time, bool) {
	if factor <= 0 || factor > FactorMax {
		return time.Time{}, false
	}
	if factor < FactorMin {
		return factorBase.AddDate(0, 0, factor), true
	}
	r := civilDays(ref)
	x := int64(factor)
	if lo := r - DueDateWindowPast - x; lo > 0 {
		x += (lo + FactorCycle - 1) / FactorCycle * FactorCycle
	}
	// x is now the first candidate not before the window start.
	if x > r+DueDateWindowFuture && x-FactorCycle >= int64(factor) {
		if prev := x - FactorCycle; r-prev <= x-r {
			x = prev
		}
	}
	return factorBase.AddDate(0, 0, int(x)), true
}
