package automation

import (
	"testing"
	"time"
)

func instant(t *testing.T, value string) time.Time {
	t.Helper()
	result, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func daily(zone string, hour, minute int) Calendar {
	return Calendar{Zone: zone, Hour: hour, Minute: minute, Weekdays: []int{0, 1, 2, 3, 4, 5, 6}}
}

func TestCalendarZonesGapsAndFirstFold(t *testing.T) {
	for _, test := range []struct {
		name        string
		calendar    Calendar
		after, want string
	}{
		{"utc", daily("UTC", 9, 0), "2026-10-25T08:59:59Z", "2026-10-25T09:00:00Z"},
		{"strictly_after", daily("UTC", 9, 0), "2026-10-25T09:00:00Z", "2026-10-26T09:00:00Z"},
		{"harare", daily("Africa/Harare", 9, 30), "2026-10-25T00:00:00Z", "2026-10-25T07:30:00Z"},
		{"new_york_gap", daily("America/New_York", 2, 30), "2026-03-08T00:00:00Z", "2026-03-09T06:30:00Z"},
		{"new_york_first_fold", daily("America/New_York", 1, 30), "2026-11-01T00:00:00Z", "2026-11-01T05:30:00Z"},
		{"new_york_no_second_fold", daily("America/New_York", 1, 30), "2026-11-01T05:45:00Z", "2026-11-02T06:30:00Z"},
		{"lord_howe_half_hour_gap", daily("Australia/Lord_Howe", 2, 15), "2026-10-03T00:00:00Z", "2026-10-04T15:15:00Z"},
		{"lord_howe_half_hour_fold", daily("Australia/Lord_Howe", 1, 45), "2026-04-04T00:00:00Z", "2026-04-04T14:45:00Z"},
		{"lord_howe_no_second_fold", daily("Australia/Lord_Howe", 1, 45), "2026-04-04T14:50:00Z", "2026-04-05T15:15:00Z"},
		{"weekday", Calendar{Zone: "UTC", Hour: 9, Weekdays: []int{1}}, "2026-10-26T10:00:00Z", "2026-11-02T09:00:00Z"},
	} {
		t.Run(test.name, func(t *testing.T) {
			actual, err := test.calendar.Next(instant(t, test.after))
			if err != nil || !actual.Equal(instant(t, test.want)) {
				t.Fatal(actual, err, "want", test.want)
			}
		})
	}
}

func TestCalendarInvalidAndCanonical(t *testing.T) {
	for _, calendar := range []Calendar{{Zone: "Local", Weekdays: []int{0}}, {Zone: "../UTC", Weekdays: []int{0}}, {Zone: "Unknown/Nowhere", Weekdays: []int{0}}, {Zone: "UTC", Hour: 24, Weekdays: []int{0}}, {Zone: "UTC", Minute: 60, Weekdays: []int{0}}, {Zone: "UTC", Weekdays: []int{0, 0}}, {Zone: "UTC", Weekdays: []int{7}}, {Zone: "UTC"}} {
		if _, err := calendar.Canonical(); err == nil {
			t.Fatal("invalid accepted", calendar)
		}
	}
	calendar := Calendar{Zone: "UTC", Hour: 9, Weekdays: []int{5, 1, 3}}
	copy, err := calendar.Canonical()
	if err != nil || copy.Weekdays[0] != 1 {
		t.Fatal(copy, err)
	}
	copy.Weekdays[0] = 0
	if calendar.Weekdays[0] != 5 {
		t.Fatal("mutated caller")
	}
}

func TestOccurrenceIdentityAndClockDirection(t *testing.T) {
	calendar := daily("UTC", 9, 0)
	after := instant(t, "2026-10-25T08:00:00Z")
	due, err := calendar.Next(after)
	if err != nil {
		t.Fatal(err)
	}
	backward, err := calendar.Next(after.Add(-time.Hour))
	if err != nil || !due.Equal(backward) {
		t.Fatal(backward, err)
	}
	forward, err := calendar.Next(after.Add(72 * time.Hour))
	if err != nil || !forward.After(due) {
		t.Fatal(forward, err)
	}
	id := OccurrenceID("schedule", 1, due)
	if id != OccurrenceID("schedule", 1, due.In(time.FixedZone("other", 7200))) || id == OccurrenceID("schedule", 2, due) || id == OccurrenceID("other", 1, due) {
		t.Fatal("unstable or aliased identity")
	}
}
