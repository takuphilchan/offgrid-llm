// Package jsonwire validates identities and exact text before encoding/json can
// replace malformed Unicode or collapse repeated object keys. It does not
// normalize text, prohibit literal replacement characters, or interpret schemas.
package jsonwire

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"
)

var ErrAmbiguous = errors.New("JSON contains malformed Unicode or repeated object keys")

func hex4(raw []byte) uint16 {
	var n uint16
	for _, c := range raw {
		n <<= 4
		switch {
		case c >= '0' && c <= '9':
			n |= uint16(c - '0')
		case c >= 'a' && c <= 'f':
			n |= uint16(c - 'a' + 10)
		case c >= 'A' && c <= 'F':
			n |= uint16(c - 'A' + 10)
		}
	}
	return n
}

func Validate(raw []byte) error {
	if !utf8.Valid(raw) || !json.Valid(raw) {
		return ErrAmbiguous
	}
	// JSON syntax is already valid. Inspect escapes inside strings only; a
	// literal backslash followed by "uD800" is not a surrogate escape.
	quoted := false
	for i := 0; i < len(raw); i++ {
		if raw[i] == '"' {
			quoted = !quoted
			continue
		}
		if !quoted || raw[i] != '\\' {
			continue
		}
		i++
		if raw[i] != 'u' {
			continue
		}
		u := hex4(raw[i+1 : i+5])
		i += 4
		if u >= 0xdc00 && u <= 0xdfff {
			return ErrAmbiguous
		}
		if u < 0xd800 || u > 0xdbff {
			continue
		}
		if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
			return ErrAmbiguous
		}
		low := hex4(raw[i+3 : i+7])
		if low < 0xdc00 || low > 0xdfff {
			return ErrAmbiguous
		}
		i += 6
	}
	// An iterative walk also checks nested objects without adding a recursive
	// parser or losing exact large-number spelling to float64 conversion.
	type frame struct {
		keys map[string]bool
		key  bool
	}
	stack := []frame{}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	for {
		token, err := d.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return ErrAmbiguous
		}
		if delim, ok := token.(json.Delim); ok && (delim == '}' || delim == ']') {
			stack = stack[:len(stack)-1]
			continue
		}
		if len(stack) > 0 {
			f := &stack[len(stack)-1]
			if f.keys != nil && f.key {
				key, ok := token.(string)
				if !ok || f.keys[key] {
					return ErrAmbiguous
				}
				f.keys[key], f.key = true, false
				continue
			}
			if f.keys != nil {
				f.key = true
			}
		}
		if delim, ok := token.(json.Delim); ok {
			f := frame{}
			if delim == '{' {
				f.keys, f.key = map[string]bool{}, true
			}
			stack = append(stack, f)
		}
	}
}
