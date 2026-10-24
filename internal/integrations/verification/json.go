package verification

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

// Reject duplicate keys as well as unknown fields and trailing values; ordinary
// encoding/json accepts duplicates, which is unsuitable for an explicit contract.
func decodeStrict(r io.Reader, limit int64, out any) error {
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil || int64(len(b)) > limit || !utf8.Valid(b) {
		return errors.New("JSON limit exceeded")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	if err = uniqueValue(d, 0); err != nil {
		return err
	}
	if _, err = d.Token(); err != io.EOF {
		return errors.New("trailing JSON")
	}
	d = json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	return d.Decode(out)
}

func uniqueValue(d *json.Decoder, depth int) error {
	if depth > 8 {
		return errors.New("JSON nesting limit exceeded")
	}
	t, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	if delim != '{' && delim != '[' {
		return errors.New("invalid JSON")
	}
	keys := map[string]bool{}
	for d.More() {
		if delim == '{' {
			k, err := d.Token()
			if err != nil {
				return err
			}
			key, ok := k.(string)
			if !ok || key != strings.ToLower(key) || keys[key] {
				return errors.New("duplicate JSON key")
			}
			keys[key] = true
		}
		if err := uniqueValue(d, depth+1); err != nil {
			return err
		}
	}
	_, err = d.Token()
	return err
}
