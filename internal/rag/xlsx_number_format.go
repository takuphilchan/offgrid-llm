package rag

import (
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

var xlsxNumber = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE]([+-]?[0-9]+))?$`)
var xlsxDecimalPattern = regexp.MustCompile(`^([#0,]+)(?:\.([0#]+))?(?:[eE]([+-])([0]+))?$`)

func xlsxRational(raw string) (*big.Rat, bool) {
	v := strings.TrimSpace(raw)
	if len(v) > 1024 {
		return nil, false
	}
	match := xlsxNumber.FindStringSubmatch(v)
	if match == nil {
		return nil, false
	}
	if match[1] != "" {
		exponent, err := strconv.Atoi(match[1])
		if err != nil || exponent < -1000 || exponent > 1000 {
			return nil, false
		}
	}
	r, ok := new(big.Rat).SetString(v)
	return r, ok
}

type xlsxFormatToken struct {
	text    string
	literal bool
}

// Quoted/escaped literals never become date or numeric-format instructions.
func xlsxFormatTokens(code string) ([]xlsxFormatToken, bool) {
	var result []xlsxFormatToken
	for i := 0; i < len(code); {
		switch code[i] {
		case '"':
			end := strings.IndexByte(code[i+1:], '"')
			if end < 0 {
				return nil, false
			}
			end += i + 1
			result = append(result, xlsxFormatToken{code[i+1 : end], true})
			i = end + 1
		case '\\':
			if i+1 >= len(code) {
				return nil, false
			}
			result = append(result, xlsxFormatToken{code[i+1 : i+2], true})
			i += 2
		case '_', '*':
			// Width-dependent alignment cannot be reconstructed from a cell value.
			return nil, false
		case '[':
			end := strings.IndexByte(code[i:], ']')
			if end < 0 {
				return nil, false
			}
			end += i
			value := strings.ToLower(code[i+1 : end])
			switch value {
			case "black", "blue", "cyan", "green", "magenta", "red", "white", "yellow":
			default:
				return nil, false
			}
			i = end + 1
		default:
			result = append(result, xlsxFormatToken{code[i : i+1], false})
			i++
		}
	}
	return result, true
}

func xlsxFormatSections(code string) ([]string, bool) {
	var parts []string
	start := 0
	quoted, escaped := false, false
	for i, c := range code {
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == '"' {
			quoted = !quoted
		}
		if c == ';' && !quoted {
			parts = append(parts, code[start:i])
			start = i + 1
		}
	}
	parts = append(parts, code[start:])
	return parts, !quoted && !escaped && len(parts) <= 4
}

func xlsxDisplayNumber(raw, code string, date1904 bool) (string, string) {
	if code == "General" || code == "@" {
		return raw, "stored"
	}
	number, ok := xlsxRational(raw)
	if !ok {
		return raw, "unrendered"
	}
	sections, ok := xlsxFormatSections(code)
	if !ok {
		return raw, "unrendered"
	}
	selected := sections[0]
	explicitNegative := false
	if number.Sign() < 0 && len(sections) > 1 {
		selected = sections[1]
		explicitNegative = true
	}
	if number.Sign() == 0 && len(sections) > 2 {
		selected = sections[2]
	}
	if selected == "" {
		return "", "format_hidden"
	}
	tokens, ok := xlsxFormatTokens(selected)
	if !ok {
		return raw, "unrendered"
	}
	for _, token := range tokens {
		if !token.literal && strings.ContainsAny(token.text, "ymdhsYMDHS") {
			value, state := xlsxDisplayDate(number, tokens, date1904)
			if state == "unrendered" {
				value = raw
			}
			return value, state
		}
	}
	var prefix, suffix, pattern strings.Builder
	started, finished := false, false
	percent, scale := 0, 0
	for _, token := range tokens {
		if !token.literal && !strings.ContainsAny(token.text, "0#.,Ee+- %$()") {
			// Fractions, text substitution and numeric slash masks are not a
			// supported numeric projection. Date separators are handled above.
			return raw, "unrendered"
		}
		if !token.literal && strings.ContainsAny(token.text, "0#.,Ee+-") && !(token.text == "-" && !started) && !(token.text == "+" && !started) {
			if finished {
				return raw, "unrendered"
			}
			started = true
			pattern.WriteString(token.text)
			continue
		}
		if started {
			finished = true
		}
		if !token.literal && token.text == "%" {
			percent++
		}
		if started {
			suffix.WriteString(token.text)
		} else {
			prefix.WriteString(token.text)
		}
	}
	if !started {
		return prefix.String(), "format_derived"
	}
	p := pattern.String()
	for strings.HasSuffix(p, ",") {
		scale++
		p = strings.TrimSuffix(p, ",")
	}
	match := xlsxDecimalPattern.FindStringSubmatch(p)
	if match == nil || len(match[2]) > 30 || percent > 2 || scale > 4 || len(match[1]) > 40 || len(match[4]) > 4 {
		return raw, "unrendered"
	}
	n := new(big.Rat).Abs(number)
	if percent > 0 {
		n.Mul(n, new(big.Rat).SetInt(pow10(2*percent)))
	}
	if scale > 0 {
		n.Quo(n, new(big.Rat).SetInt(pow10(3*scale)))
	}
	exponent := 0
	if match[3] != "" {
		if match[1] != "0" {
			return raw, "unrendered"
		}
		if n.Sign() != 0 {
			exponent = len(n.Num().String()) - len(n.Denom().String())
			power := new(big.Rat).SetInt(pow10(absInt(exponent)))
			if exponent < 0 {
				n.Mul(n, power)
			} else {
				n.Quo(n, power)
			}
			if n.Cmp(big.NewRat(1, 1)) < 0 {
				n.Mul(n, big.NewRat(10, 1))
				exponent--
			}
		}
	}
	digits := roundedXLSXDecimal(n, len(match[2]))
	if match[3] != "" && strings.HasPrefix(digits, "10") {
		n.Quo(n, big.NewRat(10, 1))
		exponent++
		digits = roundedXLSXDecimal(n, len(match[2]))
	}
	pieces := strings.SplitN(digits, ".", 2)
	integer := pieces[0]
	minInteger := strings.Count(match[1], "0")
	if integer == "0" && minInteger == 0 {
		integer = ""
	}
	if len(integer) < minInteger {
		integer = strings.Repeat("0", minInteger-len(integer)) + integer
	}
	if strings.Contains(match[1], ",") {
		for at := len(integer) - 3; at > 0; at -= 3 {
			integer = integer[:at] + "," + integer[at:]
		}
	}
	decimal := ""
	if len(pieces) > 1 {
		decimal = pieces[1]
		minimum := strings.LastIndex(match[2], "0") + 1
		for len(decimal) > minimum && strings.HasSuffix(decimal, "0") {
			decimal = strings.TrimSuffix(decimal, "0")
		}
	}
	display := integer
	if decimal != "" {
		display += "." + decimal
	}
	if match[3] != "" {
		sign := "+"
		if exponent < 0 {
			sign = "-"
		} else if match[3] == "-" {
			sign = ""
		}
		exp := strconv.Itoa(absInt(exponent))
		if len(exp) < len(match[4]) {
			exp = strings.Repeat("0", len(match[4])-len(exp)) + exp
		}
		display += "E" + sign + exp
	}
	if number.Sign() < 0 && !explicitNegative {
		display = "-" + display
	}
	return prefix.String() + display + suffix.String(), "format_derived"
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
func pow10(n int) *big.Int { return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil) }
func roundedXLSXDecimal(n *big.Rat, places int) string {
	numerator := new(big.Int).Mul(n.Num(), pow10(places))
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(numerator, n.Denom(), r)
	if new(big.Int).Mul(r, big.NewInt(2)).Cmp(n.Denom()) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	s := q.String()
	if places == 0 {
		return s
	}
	if len(s) <= places {
		s = strings.Repeat("0", places-len(s)+1) + s
	}
	return s[:len(s)-places] + "." + s[len(s)-places:]
}
