// Package textsearch provides bounded, literal multilingual retrieval terms.
// Normalization is for indexes only; original evidence and its offsets never change.
package textsearch

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const MaxQueryTerms = 64
const MaxQueryBytes = 16 << 10

func boundedQuery(text string) string {
	if len(text) <= MaxQueryBytes {
		return text
	}
	end := MaxQueryBytes
	for end > 0 && !utf8.RuneStart(text[end]) {
		end--
	}
	return text[:end]
}

func unspaced(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Thai, unicode.Lao, unicode.Khmer, unicode.Myanmar)
}

// Tokens preserves short identifiers and numbers. Unspaced scripts emit adjacent
// bigrams as well as individual letters; combining marks remain with their base.
func Tokens(text string) []string {
	text = norm.NFC.String(strings.ToLower(text))
	var result []string
	var word []rune
	var units []string
	flushUnits := func() {
		for i, u := range units {
			result = append(result, u)
			if i+1 < len(units) {
				result = append(result, u+units[i+1])
			}
		}
		units = nil
	}
	flushWord := func() {
		if len(word) > 0 {
			result = append(result, string(word))
			word = nil
		}
	}
	for _, r := range text {
		switch {
		case unicode.IsMark(r):
			if len(units) > 0 {
				units[len(units)-1] += string(r)
			} else if len(word) > 0 {
				word = append(word, r)
			}
		case unspaced(r):
			flushWord()
			units = append(units, string(r))
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			flushUnits()
			word = append(word, r)
			if len(word) == 128 {
				flushWord()
			}
		default:
			flushWord()
			flushUnits()
		}
	}
	flushWord()
	flushUnits()
	return result
}

func Frequencies(text string) map[string]int {
	result := map[string]int{}
	for _, term := range Tokens(text) {
		result[term]++
	}
	return result
}

func QueryTerms(text string) []string {
	seen := map[string]bool{}
	var terms []string
	for _, term := range Tokens(boundedQuery(text)) {
		if !seen[term] {
			seen[term] = true
			terms = append(terms, term)
		}
		if len(terms) == MaxQueryTerms {
			break
		}
	}
	return terms
}

// IndexText is a deterministic token projection, never displayed as source text.
func IndexText(text string) string { return strings.Join(Tokens(text), " ") }

// FTSQuery never exposes the FTS expression language to user input.
func FTSQuery(text string) string {
	text = boundedQuery(text)
	// A quoted phrase remains one literal phrase in the token projection. It
	// contributes a match alongside ordinary terms, rather than forcing every
	// conversational word to match. No operators from the input are executed.
	terms := QueryTerms(text)
	for i, t := range terms {
		terms[i] = "\"" + strings.ReplaceAll(t, "\"", "\"\"") + "\""
	}
	quoted := strings.Split(text, "\"")
	for i := 1; i+1 < len(quoted) && i < 16; i += 2 {
		phrase := Tokens(quoted[i])
		if len(phrase) > 1 && len(phrase) <= 32 {
			terms = append(terms, "\""+strings.Join(phrase, " ")+"\"")
		}
	}
	return strings.Join(terms, " OR ")
}
