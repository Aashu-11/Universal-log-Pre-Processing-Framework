package normalize

import (
	"fmt"
	"strconv"
	"time"
)

// resolveTimestamp turns one raw field value into a UTC epoch-nanosecond
// timestamp, per spec's TimestampSpec. It never fails silently: an error
// here means the caller falls back to event.ingested_at with a quality
// penalty, never a fabricated time.
func resolveTimestamp(raw string, spec TimestampSpec, now time.Time) (epochNs int64, err error) {
	if spec.EpochUnit != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("timestamp: parse epoch %q: %w", raw, err)
		}
		switch spec.EpochUnit {
		case "seconds":
			return n * int64(time.Second), nil
		case "milliseconds":
			return n * int64(time.Millisecond), nil
		case "nanoseconds":
			return n, nil
		default:
			return 0, fmt.Errorf("timestamp: unknown epoch_unit %q", spec.EpochUnit)
		}
	}

	loc := time.UTC
	if spec.Timezone != "" {
		l, err := time.LoadLocation(spec.Timezone)
		if err != nil {
			return 0, fmt.Errorf("timestamp: load timezone %q: %w", spec.Timezone, err)
		}
		loc = l
	}

	if len(spec.Formats) == 0 {
		return 0, fmt.Errorf("timestamp: no formats configured")
	}

	var lastErr error
	for _, layout := range spec.Formats {
		t, err := time.ParseInLocation(layout, raw, loc)
		if err != nil {
			lastErr = err
			continue
		}
		if t.Year() == 0 && spec.InferMissingYear {
			t = resolveMissingYear(t, now)
		}
		return t.UnixNano(), nil
	}
	return 0, fmt.Errorf("timestamp: no format matched %q: %w", raw, lastErr)
}

// resolveMissingYear handles RFC3164-style timestamps that carry no year
// (Go parses them with year 0000). It tries the collector's current year
// and the previous year, and picks whichever produces a timestamp closer to
// now — this is the standard December/January rollover heuristic every
// syslog collector needs: an event timestamped "Dec 31 23:59:59" received
// on "Jan 1" of the *next* calendar year must resolve to December of the
// *previous* year, not the current one.
func resolveMissingYear(t, now time.Time) time.Time {
	candidateThisYear := withYear(t, now.Year())
	candidateLastYear := withYear(t, now.Year()-1)
	candidateNextYear := withYear(t, now.Year()+1)

	best := candidateThisYear
	bestDelta := absDuration(now.Sub(candidateThisYear))

	for _, c := range []time.Time{candidateLastYear, candidateNextYear} {
		if d := absDuration(now.Sub(c)); d < bestDelta {
			best = c
			bestDelta = d
		}
	}
	return best
}

func withYear(t time.Time, year int) time.Time {
	return time.Date(year, t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
