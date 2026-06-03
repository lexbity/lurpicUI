package scale

import "time"

// formatTimeTick formats a time tick with redundant-field elision relative
// to the previous tick. If prev is the zero value, no elision is performed.
// The format is determined by the interval name.
func formatTimeTick(t time.Time, prev time.Time, iv timeInterval) string {
	switch iv.name {
	case "1y":
		if !prev.IsZero() && prev.Year() == t.Year() {
			return ""
		}
		return t.Format("2006")
	case "3mo":
		return formatElided(t, prev, "2006", "Jan")
	case "1mo":
		return formatElided(t, prev, "2006", "Jan")
	case "1w":
		return formatElidedDate(t, prev)
	case "2d":
		return formatElidedDate(t, prev)
	case "1d":
		return formatElidedDate(t, prev)
	case "12h", "6h", "3h", "1h":
		return formatElidedHour(t, prev)
	case "30m", "15m", "5m", "1m":
		return formatElidedMinute(t, prev)
	case "30s", "15s", "5s", "1s":
		return formatElidedSecond(t, prev)
	default:
		return t.Format(time.RFC3339)
	}
}

// formatElided shows the date when the day changes, then the time.
func formatElidedDate(t, prev time.Time) string {
	if prev.IsZero() || t.Year() != prev.Year() || t.YearDay() != prev.YearDay() {
		if prev.IsZero() || t.Year() != prev.Year() {
			return t.Format("Jan _2 2006")
		}
		return t.Format("Jan _2")
	}
	return t.Format("Mon")
}

// formatElidedHour shows the date if different, then HH:00.
func formatElidedHour(t, prev time.Time) string {
	if prev.IsZero() || t.Year() != prev.Year() || t.YearDay() != prev.YearDay() {
		if prev.IsZero() || t.Year() != prev.Year() {
			return t.Format("Jan _2 2006 15:04")
		}
		return t.Format("Jan _2 15:04")
	}
	return t.Format("15:04")
}

// formatElidedMinute shows date if different, then HH:mm.
func formatElidedMinute(t, prev time.Time) string {
	if prev.IsZero() || t.Year() != prev.Year() || t.YearDay() != prev.YearDay() {
		if prev.IsZero() || t.Year() != prev.Year() {
			return t.Format("Jan _2 2006 15:04")
		}
		return t.Format("Jan _2 15:04")
	}
	return t.Format("15:04")
}

// formatElidedSecond shows date if different, then HH:mm:ss.
func formatElidedSecond(t, prev time.Time) string {
	if prev.IsZero() || t.Year() != prev.Year() || t.YearDay() != prev.YearDay() {
		if prev.IsZero() || t.Year() != prev.Year() {
			return t.Format("Jan _2 2006 15:04:05")
		}
		return t.Format("Jan _2 15:04:05")
	}
	return t.Format("15:04:05")
}

// formatElided shows a primary format if the year changes and a secondary
// format otherwise, with the secondary also elided if the value hasn't changed.
func formatElided(t, prev time.Time, fullFmt, elidedFmt string) string {
	if prev.IsZero() || t.Year() != prev.Year() {
		return t.Format(fullFmt + " " + elidedFmt)
	}
	return t.Format(elidedFmt)
}
