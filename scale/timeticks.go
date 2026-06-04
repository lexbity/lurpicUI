package scale

import (
	"math"
	"time"
)

// timeInterval describes a calendar-aware tick interval.
type timeInterval struct {
	name      string
	duration  time.Duration
	roundDown func(time.Time) time.Time
	advance   func(time.Time) time.Time
}

// timeIntervals is the ordered table of candidate intervals, coarsest first.
// The first interval whose approximate tick count fits within the requested
// count is selected.
var timeIntervals = []timeInterval{
	{"1y", 366 * 24 * time.Hour, roundDownYear, advanceYear},
	{"3mo", 92 * 24 * time.Hour, roundDownQuarter, advanceQuarter},
	{"1mo", 31 * 24 * time.Hour, roundDownMonth, advanceMonth},
	{"1w", 7 * 24 * time.Hour, roundDownWeek, advanceWeek},
	{"2d", 2 * 24 * time.Hour, roundDown2Day, advance2Day},
	{"1d", 24 * time.Hour, roundDownDay, advanceDay},
	{"12h", 12 * time.Hour, durationRoundDown(12 * time.Hour), durationAdvance(12 * time.Hour)},
	{"6h", 6 * time.Hour, durationRoundDown(6 * time.Hour), durationAdvance(6 * time.Hour)},
	{"3h", 3 * time.Hour, durationRoundDown(3 * time.Hour), durationAdvance(3 * time.Hour)},
	{"1h", time.Hour, durationRoundDown(time.Hour), durationAdvance(time.Hour)},
	{"30m", 30 * time.Minute, durationRoundDown(30 * time.Minute), durationAdvance(30 * time.Minute)},
	{"15m", 15 * time.Minute, durationRoundDown(15 * time.Minute), durationAdvance(15 * time.Minute)},
	{"5m", 5 * time.Minute, durationRoundDown(5 * time.Minute), durationAdvance(5 * time.Minute)},
	{"1m", time.Minute, durationRoundDown(time.Minute), durationAdvance(time.Minute)},
	{"30s", 30 * time.Second, durationRoundDown(30 * time.Second), durationAdvance(30 * time.Second)},
	{"15s", 15 * time.Second, durationRoundDown(15 * time.Second), durationAdvance(15 * time.Second)},
	{"5s", 5 * time.Second, durationRoundDown(5 * time.Second), durationAdvance(5 * time.Second)},
	{"1s", time.Second, durationRoundDown(time.Second), durationAdvance(time.Second)},
}

func roundDownYear(t time.Time) time.Time {
	return time.Date(t.Year(), 1, 1, 0, 0, 0, 0, t.Location())
}

func advanceYear(t time.Time) time.Time {
	return t.AddDate(1, 0, 0)
}

func roundDownQuarter(t time.Time) time.Time {
	q := (t.Month()-1)/3 + 1
	return time.Date(t.Year(), (q-1)*3+1, 1, 0, 0, 0, 0, t.Location())
}

func advanceQuarter(t time.Time) time.Time {
	return t.AddDate(0, 3, 0)
}

func roundDownMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

func advanceMonth(t time.Time) time.Time {
	return t.AddDate(0, 1, 0)
}

func roundDownWeek(t time.Time) time.Time {
	weekday := t.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	daysToSubtract := int(weekday) - int(time.Monday)
	return time.Date(t.Year(), t.Month(), t.Day()-daysToSubtract, 0, 0, 0, 0, t.Location())
}

func advanceWeek(t time.Time) time.Time {
	return t.AddDate(0, 0, 7)
}

func roundDown2Day(t time.Time) time.Time {
	days := t.UnixMilli() / (86400 * 1000)
	if days%2 != 0 {
		days--
	}
	return time.UnixMilli(days * 86400 * 1000).In(t.Location())
}

func advance2Day(t time.Time) time.Time {
	return t.AddDate(0, 0, 2)
}

func roundDownDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func advanceDay(t time.Time) time.Time {
	return t.AddDate(0, 0, 1)
}

func durationRoundDown(d time.Duration) func(time.Time) time.Time {
	return func(t time.Time) time.Time {
		return t.Truncate(d)
	}
}

func durationAdvance(d time.Duration) func(time.Time) time.Time {
	return func(t time.Time) time.Time {
		return t.Add(d)
	}
}

// chooseInterval selects the interval whose approximate tick count is
// closest to the requested count across the span [loMs, hiMs] in the
// given location.
func chooseInterval(loMs, hiMs float64, count int, loc *time.Location) timeInterval {
	if count <= 0 {
		return timeInterval{}
	}
	span := hiMs - loMs
	if span <= 0 {
		return timeInterval{}
	}
	fcount := float64(count)
	best := timeIntervals[len(timeIntervals)-1]
	bestDiff := math.MaxFloat64
	for _, iv := range timeIntervals {
		approx := span / float64(iv.duration.Milliseconds())
		diff := math.Abs(approx - fcount)
		if diff < bestDiff {
			bestDiff = diff
			best = iv
		}
	}
	return best
}

// timeTicks generates calendar-aligned tick values across [loMs, hiMs]
// using the coarsest interval that fits within count. Returns the tick
// values (in Unix ms) and the interval that was selected.
func timeTicks(loMs, hiMs float64, count int, loc *time.Location) ([]float64, timeInterval) {
	iv := chooseInterval(loMs, hiMs, count, loc)
	if iv.roundDown == nil {
		return nil, iv
	}
	tLo := time.UnixMilli(int64(loMs)).In(loc)
	tHi := time.UnixMilli(int64(hiMs)).In(loc)

	start := iv.roundDown(tLo)

	var vals []float64
	for t := start; !t.After(tHi); t = iv.advance(t) {
		ms := float64(t.UnixMilli())
		if ms >= loMs-0.5 && ms <= hiMs+0.5 {
			if len(vals) == 0 || ms != vals[len(vals)-1] {
				vals = append(vals, ms)
			}
		}
	}
	return vals, iv
}
