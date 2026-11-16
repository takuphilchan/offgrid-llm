package document

import "unicode/utf8"

// Keep option lists bounded in stored graphs as well as worker receipts.
func ValidateFormOptions(label string, options []FormOption, choice bool) error {
	if len(label) > 128<<10 || !utf8.ValidString(label) || len(options) > 4096 || !choice && len(options) > 0 {
		return invalidExtraction("form options")
	}
	total := 0
	for i, option := range options {
		if option.Index != i || !utf8.ValidString(option.Label) {
			return invalidExtraction("form option")
		}
		total += len(option.Label) + 32
		if total > 128<<10 {
			return invalidExtraction("form option limit")
		}
	}
	return nil
}
