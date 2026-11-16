package document

import (
	"strings"
	"unicode"
)

// Conservative transcription check only: this neither verifies arithmetic nor
// proves that a number belongs to the claimed subject. Supported calendar dates
// are compared as whole tuples; other reformattings remain literal and may need
// review rather than silently normalizing critical source values.
func synthesisNumbersSupported(claim, evidence string) bool {
	claim = withoutSupportedCalendarDates(claim, evidence)
	return synthesisLiteralNumbersSupported(claim, evidence)
}

func synthesisLiteralNumbersSupported(claim, evidence string) bool {
	available := map[string]bool{}
	for _, number := range synthesisNumbers(evidence) {
		available[number] = true
	}
	for _, number := range synthesisNumbers(claim) {
		if !available[number] {
			return false
		}
	}
	return true
}
func synthesisNumbers(text string) []string {
	runes := []rune(text)
	var values []string
	for i := 0; i < len(runes); i++ {
		start := i
		if strings.ContainsRune("+-−", runes[i]) && i+1 < len(runes) && unicode.IsDigit(runes[i+1]) {
			i++
		}
		if !unicode.IsDigit(runes[i]) {
			continue
		}
		for i+1 < len(runes) {
			if unicode.IsDigit(runes[i+1]) {
				i++
				continue
			}
			if strings.ContainsRune(".,:/٫٬", runes[i+1]) && i+2 < len(runes) && unicode.IsDigit(runes[i+2]) {
				i += 2
				continue
			}
			break
		}
		if i+1 < len(runes) && strings.ContainsRune("%٪", runes[i+1]) {
			i++
		}
		values = append(values, string(runes[start:i+1]))
	}
	return values
}
