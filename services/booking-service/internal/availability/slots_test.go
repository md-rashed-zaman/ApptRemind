package availability

import (
	"testing"
	"time"
)

func TestAvailableSlots_Basic(t *testing.T) {
	loc := time.UTC
	day := time.Date(2026, 1, 28, 0, 0, 0, 0, loc)
	windowStart := time.Date(2026, 1, 28, 9, 0, 0, 0, loc)
	windowEnd := time.Date(2026, 1, 28, 10, 0, 0, 0, loc)

	busy := []Interval{
		{Start: day.Add(9*time.Hour + 15*time.Minute), End: day.Add(9*time.Hour + 45*time.Minute)},
	}

	slots := AvailableSlots(windowStart, windowEnd, 15*time.Minute, 15*time.Minute, busy, day)
	if len(slots) != 2 {
		t.Fatalf("expected 2 slots, got %d", len(slots))
	}
	if !slots[0].Equal(day.Add(9 * time.Hour)) {
		t.Fatalf("expected first slot 09:00, got %s", slots[0].Format(time.RFC3339))
	}
	if !slots[1].Equal(day.Add(9*time.Hour + 45*time.Minute)) {
		t.Fatalf("expected second slot 09:45, got %s", slots[1].Format(time.RFC3339))
	}
}

func TestAvailableSlots_SkipsPast(t *testing.T) {
	loc := time.UTC
	day := time.Date(2026, 1, 28, 0, 0, 0, 0, loc)
	windowStart := day.Add(9 * time.Hour)
	windowEnd := day.Add(10 * time.Hour)

	now := day.Add(9*time.Hour + 31*time.Minute)
	slots := AvailableSlots(windowStart, windowEnd, 15*time.Minute, 15*time.Minute, nil, now)
	// 09:00, 09:15, 09:30 are in the past (start < now). 09:45 is future.
	if len(slots) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(slots))
	}
	if !slots[0].Equal(day.Add(9*time.Hour + 45*time.Minute)) {
		t.Fatalf("expected slot 09:45, got %s", slots[0].Format(time.RFC3339))
	}
}
