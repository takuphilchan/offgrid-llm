// Package automation defines calendar data and occurrence identity, not another
// agent executor. All due work still uses the application-owned durable runner.
package automation

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"sort"
	"strconv"
	"time"
	_ "time/tzdata" // Offline fallback; never fetch timezone data at runtime.
)

var ErrCalendar = errors.New("invalid calendar: use an IANA timezone, hour 0–23, minute 0–59 and distinct weekdays 0 (Sunday) through 6")
var zoneName = regexp.MustCompile(`^[A-Za-z0-9_+\-]+(/[A-Za-z0-9_+\-]+)*$`)

type Calendar struct {
	Zone     string `json:"zone"`
	Hour     int    `json:"hour"`
	Minute   int    `json:"minute"`
	Weekdays []int  `json:"weekdays"`
}

func (c Calendar) Canonical() (Calendar, error) {
	if c.Zone == "Local" || len(c.Zone) > 128 || !zoneName.MatchString(c.Zone) || c.Hour < 0 || c.Hour > 23 || c.Minute < 0 || c.Minute > 59 || len(c.Weekdays) == 0 || len(c.Weekdays) > 7 {
		return Calendar{}, ErrCalendar
	}
	if _, err := time.LoadLocation(c.Zone); err != nil {
		return Calendar{}, ErrCalendar
	}
	copy := c
	copy.Weekdays = append([]int(nil), c.Weekdays...)
	sort.Ints(copy.Weekdays)
	for i, day := range copy.Weekdays {
		if day < 0 || day > 6 || i > 0 && day == copy.Weekdays[i-1] {
			return Calendar{}, ErrCalendar
		}
	}
	return copy, nil
}

// Next selects the first real instant of a wall-clock label, never the second
// fold. If that first instant has passed, this date is done even if its repeated
// wall time is still ahead. Missing labels have no occurrence.
func (c Calendar) Next(after time.Time) (time.Time, error) {
	c, err := c.Canonical()
	if err != nil || after.IsZero() {
		return time.Time{}, ErrCalendar
	}
	location, err := time.LoadLocation(c.Zone)
	if err != nil {
		return time.Time{}, ErrCalendar
	}
	local := after.In(location)
	date := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	for day := 0; day <= 15; day++ {
		d := date.AddDate(0, 0, day)
		weekday := int(d.Weekday())
		selected := false
		for _, allowed := range c.Weekdays {
			if allowed == weekday {
				selected = true
				break
			}
		}
		if !selected {
			continue
		}
		first := firstWallInstant(d, c.Hour, c.Minute, location)
		if !first.IsZero() && first.After(after) {
			return first.UTC(), nil
		}
	}
	return time.Time{}, ErrCalendar
}

func firstWallInstant(date time.Time, hour, minute int, location *time.Location) time.Time {
	wall := time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, time.UTC)
	// Discover actual nearby offsets instead of assuming one-hour DST or relying
	// on time.Date's intentionally unspecified choice in an ambiguous interval.
	// Minute sampling also retains offsets with historical non-minute seconds;
	// each candidate is independently round-tripped before it can be selected.
	offsets := map[int]bool{}
	start := wall.Add(-26 * time.Hour)
	for i := 0; i <= 52*60; i++ {
		_, offset := start.Add(time.Duration(i) * time.Minute).In(location).Zone()
		offsets[offset] = true
	}
	var first time.Time
	for offset := range offsets {
		candidate := wall.Add(-time.Duration(offset) * time.Second)
		actual := candidate.In(location)
		if actual.Year() == date.Year() && actual.Month() == date.Month() && actual.Day() == date.Day() && actual.Hour() == hour && actual.Minute() == minute && actual.Second() == 0 && (first.IsZero() || candidate.Before(first)) {
			first = candidate
		}
	}
	return first
}

func OccurrenceID(schedule string, revision int, due time.Time) string {
	value := schedule + "\x00" + strconv.Itoa(revision) + "\x00" + due.UTC().Format(time.RFC3339Nano)
	digest := sha256.Sum256([]byte(value))
	return "occurrence-" + hex.EncodeToString(digest[:])
}
