package integrations

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

// SetEnvironment preserves unrelated dotenv entries. It never includes values
// in errors, logs or process arguments. Only previously snapshotted files may
// be changed; callers retain the recovery copy on any subsequent failure.
func (t *Transaction) SetEnvironment(path string, values map[string]string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	allowed := false
	for _, target := range t.files {
		if target == abs {
			allowed = true
		}
	}
	if !allowed {
		return fmt.Errorf("environment target was not snapshotted")
	}
	if err = t.VerifyUnchanged(); err != nil {
		return err
	}
	_, data, err := fileSnapshot(abs)
	if err != nil {
		return err
	}
	for key, value := range values {
		if key == "" || strings.ContainsAny(key, "= \t\r\n") || strings.ContainsAny(value, "\r\n\x00") || strings.Contains(value, "${") {
			return fmt.Errorf("environment value cannot be represented safely")
		}
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	kept := make([]string, 0, len(lines)+len(values))
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		key, _, found := strings.Cut(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "export ")), "=")
		end, err := dotenvRecordEnd(lines, i)
		if err != nil {
			return err
		}
		if _, replace := values[strings.TrimSpace(key)]; replace && found {
			i = end
			continue
		}
		kept = append(kept, lines[i:end+1]...)
		i = end
	}
	var keys []string
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := strings.ReplaceAll(strings.ReplaceAll(values[key], "\\", "\\\\"), "'", "\\'")
		kept = append(kept, key+"='"+value+"'")
	}
	if err = t.VerifyUnchanged(); err != nil {
		return err
	}
	if err = storage.WriteBytes(abs, []byte(strings.TrimRight(strings.Join(kept, "\n"), "\n")+"\n")); err != nil {
		return err
	}
	return t.Checkpoint()
}

// Quoted dotenv values may span lines. Never mistake a line within one for an
// assignment to the key being updated. Unclosed values require user repair.
func dotenvRecordEnd(lines []string, start int) (int, error) {
	line := strings.TrimSpace(lines[start])
	if strings.HasPrefix(line, "#") {
		return start, nil
	}
	_, raw, found := strings.Cut(line, "=")
	raw = strings.TrimSpace(raw)
	if !found || raw == "" || raw[0] != '\'' && raw[0] != '"' {
		return start, nil
	}
	quote, escaped := raw[0], false
	for i := start; i < len(lines); i++ {
		value := lines[i]
		if i == start {
			value = raw[1:]
		}
		for j := 0; j < len(value); j++ {
			if escaped {
				escaped = false
				continue
			}
			if value[j] == '\\' {
				escaped = true
				continue
			}
			if value[j] == quote {
				return i, nil
			}
		}
		escaped = false
	}
	return start, fmt.Errorf("environment file contains an unclosed quoted value; no update was applied")
}
