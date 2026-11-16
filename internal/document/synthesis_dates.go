package document

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Recognize only unambiguous calendar forms. This is a source-value comparison,
// not a general date parser or an assertion that the model assigned a date to
// the correct obligation. Ambiguous slash dates and inferred years stay subject
// to the conservative numeric check and human review.
var sourceNamedDay = regexp.MustCompile(`(?i)\b([0-9]{1,2})\s+(January|February|March|April|May|June|July|August|September|October|November|December)(?:,?\s+([0-9]{4}))?\b`)
var sourceNamedMonth = regexp.MustCompile(`(?i)\b(January|February|March|April|May|June|July|August|September|October|November|December)\s+([0-9]{1,2})(?:,?\s+([0-9]{4}))?\b`)
var numericISODate = regexp.MustCompile(`\b([0-9]{4})-([0-9]{2})-([0-9]{2})\b`)
var numericCJKDate = regexp.MustCompile(`(?:([0-9]{4})[年년]\s*)?([0-9]{1,2})[月월]\s*([0-9]{1,2})[日일]`)

type synthesisDate struct{ year, month, day int }

func dateValue(year, month, day string) synthesisDate {
	y, _ := strconv.Atoi(year)
	m, _ := strconv.Atoi(month)
	d, _ := strconv.Atoi(day)
	return synthesisDate{y, m, d}
}
func (d synthesisDate) valid() bool {
	if d.month < 1 || d.month > 12 || d.day < 1 || d.year < 0 || d.year > 9999 {
		return false
	}
	days := []int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	if d.year == 0 || d.year%400 == 0 || d.year%4 == 0 && d.year%100 != 0 {
		days[1] = 29
	}
	return d.day <= days[d.month-1]
}
func namedMonth(name string) int {
	for i, month := range []string{"january", "february", "march", "april", "may", "june", "july", "august", "september", "october", "november", "december"} {
		if strings.EqualFold(name, month) {
			return i + 1
		}
	}
	return 0
}
func sourceCalendarDates(text string) []synthesisDate {
	var dates []synthesisDate
	add := func(d synthesisDate) {
		if d.valid() {
			dates = append(dates, d)
		}
	}
	for _, m := range sourceNamedDay.FindAllStringSubmatch(text, -1) {
		d := dateValue(m[3], "", m[1])
		d.month = namedMonth(m[2])
		add(d)
	}
	for _, m := range sourceNamedMonth.FindAllStringSubmatch(text, -1) {
		d := dateValue(m[3], "", m[2])
		d.month = namedMonth(m[1])
		add(d)
	}
	for _, re := range []*regexp.Regexp{numericISODate, numericCJKDate} {
		for _, at := range re.FindAllStringSubmatchIndex(text, -1) {
			if at[0] > 0 {
				previous, _ := utf8.DecodeLastRuneInString(text[:at[0]])
				if unicode.IsDigit(previous) {
					continue
				}
			}
			group := func(index int) string {
				if at[index*2] < 0 {
					return ""
				}
				return text[at[index*2]:at[index*2+1]]
			}
			add(dateValue(group(1), group(2), group(3)))
		}
	}
	return dates
}

// Remove a translated numeric date from numeric-token validation only when the
// entire month/day/(explicit year) tuple occurs in the cited evidence. Never
// add its month number to the global allowed-number set: that would permit an
// unrelated invented quantity elsewhere in the claim.
func withoutSupportedCalendarDates(claim, evidence string) string {
	dates := sourceCalendarDates(evidence)
	if len(dates) == 0 {
		return claim
	}
	for _, re := range []*regexp.Regexp{numericISODate, numericCJKDate} {
		matches := re.FindAllStringSubmatchIndex(claim, -1)
		for i := len(matches) - 1; i >= 0; i-- {
			at := matches[i]
			if at[0] > 0 {
				previous, _ := utf8.DecodeLastRuneInString(claim[:at[0]])
				if unicode.IsDigit(previous) {
					continue
				}
			}
			group := func(index int) string {
				if at[index*2] < 0 {
					return ""
				}
				return claim[at[index*2]:at[index*2+1]]
			}
			d := dateValue(group(1), group(2), group(3))
			if !d.valid() {
				continue
			}
			for _, source := range dates {
				if d.month == source.month && d.day == source.day && (d.year == 0 || d.year == source.year) {
					claim = claim[:at[0]] + " " + claim[at[1]:]
					break
				}
			}
		}
	}
	return claim
}
