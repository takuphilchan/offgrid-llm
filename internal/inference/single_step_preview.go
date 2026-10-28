//go:build !llama

package inference

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// Recognize only the owned envelope's first, literal "message" string. Never
// search inside arguments, metadata, reasoning, or arbitrary model prose. An
// invalid/truncated envelope may have a visible draft, but cannot emit a final
// decision: singleStepStream.emit still validates the entire original response.
type singleStepPreview struct {
	position int
	prefix   int
	started  bool
	stopped  bool
}

func (p *singleStepPreview) update(raw string, emit func(string) error) error {
	if emit == nil || p.stopped {
		return nil
	}
	const prefix = `{"message":"`
	for !p.started && p.position < len(raw) {
		b := raw[p.position]
		// Whitespace is allowed between JSON tokens, never inside the key.
		if (p.prefix == 0 || p.prefix == 1 || p.prefix == 10 || p.prefix == 11) && strings.ContainsRune(" \t\r\n", rune(b)) {
			p.position++
			continue
		}
		if b != prefix[p.prefix] {
			p.stopped = true
			return nil
		}
		p.position++
		p.prefix++
		p.started = p.prefix == len(prefix)
	}
	var text strings.Builder
	for p.started && p.position < len(raw) {
		rest := raw[p.position:]
		if rest[0] == '"' {
			p.stopped = true
			break
		}
		if rest[0] == '\\' {
			if len(rest) < 2 {
				break
			}
			n := 2
			if rest[1] == 'u' {
				n = 6
				if len(rest) < n {
					break
				}
				// Buffer a high surrogate until the complete low surrogate is
				// available. json.Unmarshal validates the paired escape below.
				if rest[2] == 'd' || rest[2] == 'D' {
					if strings.ContainsRune("89aAbB", rune(rest[3])) {
						n = 12
					}
				}
			}
			if len(rest) < n {
				break
			}
			var decoded string
			if json.Unmarshal([]byte(`"`+rest[:n]+`"`), &decoded) != nil || strings.ContainsRune(decoded, utf8.RuneError) {
				p.stopped = true
				break
			}
			text.WriteString(decoded)
			p.position += n
			continue
		}
		if rest[0] < 0x20 {
			p.stopped = true
			break
		}
		if !utf8.FullRuneInString(rest) {
			break
		}
		r, size := utf8.DecodeRuneInString(rest)
		if r == utf8.RuneError && size == 1 {
			p.stopped = true
			break
		}
		text.WriteString(rest[:size])
		p.position += size
	}
	if text.Len() > 0 {
		return emit(text.String())
	}
	return nil
}
