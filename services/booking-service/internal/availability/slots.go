package availability

import "time"

type Interval struct {
	Start time.Time
	End   time.Time
}

// AvailableSlots returns slot start times within [windowStart, windowEnd) where a booking of
// length duration would not overlap any of the busy intervals.
//
// All times are expected to be in the same location (timezone).
func AvailableSlots(windowStart, windowEnd time.Time, duration, step time.Duration, busy []Interval, now time.Time) []time.Time {
	if duration <= 0 || step <= 0 {
		return nil
	}
	if !windowEnd.After(windowStart) {
		return nil
	}
	if windowStart.Add(duration).After(windowEnd) {
		return nil
	}

	var slots []time.Time
	for t := windowStart; !t.Add(duration).After(windowEnd); t = t.Add(step) {
		if t.Before(now) {
			continue
		}
		if !overlapsAny(t, t.Add(duration), busy) {
			slots = append(slots, t)
		}
	}
	return slots
}

func overlapsAny(start, end time.Time, busy []Interval) bool {
	for _, b := range busy {
		// Half-open intervals: [start,end) overlaps [b.Start,b.End) iff start < b.End && b.Start < end.
		if start.Before(b.End) && b.Start.Before(end) {
			return true
		}
	}
	return false
}
