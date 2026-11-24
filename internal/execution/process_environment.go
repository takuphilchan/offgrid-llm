package execution

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// EnvironmentDescriptor identifies the inherited template without publishing
// its values. It is evidence, never a source of credentials or process authority.
type EnvironmentDescriptor struct {
	Version  int      `json:"version"`
	Template string   `json:"template"`
	Digest   string   `json:"digest"`
	Names    []string `json:"names"`
}

type processEnvironmentSnapshot struct{ values []string }

func captureProcessEnvironment() processEnvironmentSnapshot {
	return processEnvironmentSnapshot{values: processEnvironment()}
}

func (e processEnvironmentSnapshot) clone() processEnvironmentSnapshot {
	return processEnvironmentSnapshot{values: append([]string{}, e.values...)}
}

func (e processEnvironmentSnapshot) descriptor() EnvironmentDescriptor {
	names := make([]string, 0, len(e.values))
	for _, value := range e.values {
		name, _, _ := strings.Cut(value, "=")
		names = append(names, name)
	}
	encoded, _ := json.Marshal(e.values)
	return EnvironmentDescriptor{Version: 1, Template: "minimal-host-v1", Digest: digest(encoded), Names: names}
}

func (e processEnvironmentSnapshot) lookup(name string) string {
	for _, value := range e.values {
		key, content, _ := strings.Cut(value, "=")
		if key == name || runtime.GOOS == "windows" && strings.EqualFold(key, name) {
			return content
		}
	}
	return ""
}

// Resolve using the approved session's PATH/PATHEXT, not exec.LookPath's ambient
// process environment. Empty/relative PATH entries never select project code.
// Explicit relative paths remain exact approved invocations relative to cwd.
func (e processEnvironmentSnapshot) program(name, directory string) (string, error) {
	find := func(path string) string {
		candidates := []string{path}
		if runtime.GOOS == "windows" && filepath.Ext(path) == "" {
			candidates = nil
			for _, extension := range strings.Split(e.lookup("PATHEXT"), ";") {
				if len(extension) > 1 && extension[0] == '.' && !strings.ContainsAny(extension, `/\\:`) {
					candidates = append(candidates, path+extension)
				}
			}
		}
		for _, candidate := range candidates {
			info, err := os.Stat(candidate)
			if err == nil && info.Mode().IsRegular() && (runtime.GOOS == "windows" || info.Mode().Perm()&0111 != 0) {
				return candidate
			}
		}
		return ""
	}
	if filepath.IsAbs(name) || strings.ContainsAny(name, `/\\`) {
		path := name
		if !filepath.IsAbs(path) {
			if filepath.VolumeName(path) != "" {
				return "", ErrPath
			}
			path = filepath.Join(directory, path)
		}
		if path = find(path); path != "" {
			return path, nil
		}
	} else if filepath.VolumeName(name) == "" {
		for _, path := range filepath.SplitList(e.lookup("PATH")) {
			if !filepath.IsAbs(path) {
				continue
			}
			if candidate := find(filepath.Join(path, name)); candidate != "" {
				return candidate, nil
			}
		}
	}
	return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
}
