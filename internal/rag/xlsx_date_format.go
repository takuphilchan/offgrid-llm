package rag

import (
	"fmt"
	"math/big"
	"strings"
	"time"
)

func xlsxDisplayDate(number *big.Rat, tokens []xlsxFormatToken, date1904 bool) (string, string) {
	if number.Sign() < 0 || number.Cmp(big.NewRat(2958466, 1)) >= 0 {
		return "", "unrendered"
	}
	day := new(big.Int).Quo(number.Num(), number.Denom()).Int64()
	fraction := new(big.Rat).Sub(number, new(big.Rat).SetInt64(day))
	fraction.Mul(fraction, big.NewRat(86400, 1))
	seconds := new(big.Int).Quo(fraction.Num(), fraction.Denom()).Int64()
	wholeSeconds := fraction.IsInt()
	base := time.Date(1899, 12, 31, 0, 0, 0, 0, time.UTC)
	adjusted := day
	state := "format_derived"
	if date1904 {
		base = time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)
	} else if day > 60 {
		adjusted--
	}
	date := base.AddDate(0, 0, int(adjusted))
	year, month, d := date.Date()
	if year > 9999 {
		return "", "unrendered"
	}
	if !date1904 && day == 60 {
		year, month, d = 1900, time.February, 29
		state = "excel_1900_leap_day"
	}
	hour, minute, second := int(seconds/3600), int(seconds/60%60), int(seconds%60)
	var code strings.Builder
	for _, t := range tokens {
		if t.literal {
			code.WriteByte(' ')
		} else {
			code.WriteString(t.text)
		}
	}
	ampm := strings.Contains(strings.ToUpper(code.String()), "AM/PM")
	hasHour, hasMinute, hasSecond := false, false, false
	var out strings.Builder
	for i := 0; i < len(tokens); {
		t := tokens[i]
		if t.literal {
			out.WriteString(t.text)
			i++
			continue
		}
		if i+5 <= len(tokens) {
			var candidate strings.Builder
			for _, part := range tokens[i : i+5] {
				if part.literal {
					break
				}
				candidate.WriteString(part.text)
			}
			if strings.EqualFold(candidate.String(), "AM/PM") {
				if hour < 12 {
					out.WriteString("AM")
				} else {
					out.WriteString("PM")
				}
				i += 5
				continue
			}
		}
		letter := strings.ToLower(t.text)
		if !strings.ContainsAny(letter, "ymdhs") {
			if strings.ContainsAny(t.text, " /-:.,()") {
				out.WriteString(t.text)
				i++
				continue
			}
			return "", "unrendered"
		}
		end := i + 1
		for end < len(tokens) && !tokens[end].literal && strings.EqualFold(tokens[end].text, t.text) {
			end++
		}
		width := end - i
		value := 0
		switch letter {
		case "y":
			if width != 2 && width != 4 {
				return "", "unrendered"
			}
			value = year
			if width == 2 {
				value %= 100
			}
		case "d":
			if width > 2 || (!date1904 && day == 0) {
				return "", "unrendered"
			}
			value = d
		case "m":
			if width > 2 {
				return "", "unrendered"
			}
			// A minute follows an hour or precedes seconds. Separators and
			// quoted units do not turn a calendar month into a minute.
			previous, next := "", ""
			for j := i - 1; j >= 0; j-- {
				if !tokens[j].literal && strings.ContainsAny(strings.ToLower(tokens[j].text), "ymdhs") {
					previous = strings.ToLower(tokens[j].text)
					break
				}
			}
			for j := end; j < len(tokens); j++ {
				if !tokens[j].literal && strings.ContainsAny(strings.ToLower(tokens[j].text), "ymdhs") {
					next = strings.ToLower(tokens[j].text)
					break
				}
			}
			value = int(month)
			if previous == "h" || next == "s" {
				value = minute
				hasMinute = true
			}
		case "h":
			hasHour = true
			if width > 2 {
				return "", "unrendered"
			}
			value = hour
			if ampm {
				value %= 12
				if value == 0 {
					value = 12
				}
			}
		case "s":
			hasSecond = true
			if width > 2 {
				return "", "unrendered"
			}
			value = second
		}
		if width == 1 {
			fmt.Fprint(&out, value)
		} else {
			fmt.Fprintf(&out, "%0*d", width, value)
		}
		i = end
	}
	// Subsecond and omitted-field rounding depends on Excel's display rules.
	// Do not present a truncated clock as an exact reconstruction.
	if (hasHour || hasMinute || hasSecond) && (!wholeSeconds || (!hasSecond && second != 0) || (hasHour && !hasMinute && minute != 0)) {
		return "", "unrendered"
	}
	return out.String(), state
}
