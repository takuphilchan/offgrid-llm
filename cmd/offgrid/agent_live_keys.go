package main

import (
	"bytes"
	"strings"
	"time"
	"unicode/utf8"
)

// An incremental decoder keeps split UTF-8, escape sequences and bracketed
// pastes inside this input phase. Paste can never submit a correction or Stop.
type agentLiveKeys struct {
	data     []byte
	paste    bool
	overflow bool
	pasted   []byte
	escapeAt time.Time
}

func (k *agentLiveKeys) next(now time.Time) (key string, paste bool, ok bool) {
	if k.paste {
		end := []byte("\x1b[201~")
		if index := bytes.Index(k.data, end); index >= 0 {
			k.appendPaste(k.data[:index])
			k.data = k.data[index+len(end):]
			k.paste = false
			if k.overflow {
				key = ""
			} else {
				key = string(k.pasted)
			}
			k.pasted = nil
			return key, true, true
		}
		if len(k.data) > len(end) {
			keep := len(end) - 1
			k.appendPaste(k.data[:len(k.data)-keep])
			k.data = k.data[len(k.data)-keep:]
		}
		return "", false, false
	}
	if len(k.data) == 0 {
		return "", false, false
	}
	if k.data[0] == 27 {
		if k.escapeAt.IsZero() {
			k.escapeAt = now
		}
		if len(k.data) == 1 && now.Sub(k.escapeAt) < 50*time.Millisecond {
			return "", false, false
		}
		n := 1
		if len(k.data) > 1 && (k.data[1] == '[' || k.data[1] == 'O') {
			n = 0
			for i := 2; i < len(k.data) && i < 16; i++ {
				if k.data[i] >= 0x40 && k.data[i] <= 0x7e {
					n = i + 1
					break
				}
			}
			if n == 0 && now.Sub(k.escapeAt) < 50*time.Millisecond {
				return "", false, false
			}
			if n == 0 {
				n = min(16, len(k.data))
			}
		} else if len(k.data) > 1 && (k.data[1] == '\r' || k.data[1] == '\n') {
			n = 2
		}
		key = string(k.data[:n])
		k.data = k.data[n:]
		k.escapeAt = time.Time{}
		if key == "\x1b[200~" {
			k.paste = true
			k.overflow = false
			return k.next(now)
		}
		return key, false, true
	}
	if !utf8.FullRune(k.data) {
		return "", false, false
	}
	r, n := utf8.DecodeRune(k.data)
	k.data = k.data[n:]
	return string(r), false, true
}
func (k *agentLiveKeys) appendPaste(data []byte) {
	if len(k.pasted)+len(data) > 32000 {
		k.overflow = true
		k.pasted = nil
	}
	if !k.overflow {
		k.pasted = append(k.pasted, data...)
	}
}
func livePaste(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
}
