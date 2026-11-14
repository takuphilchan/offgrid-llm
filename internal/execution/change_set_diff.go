package execution

import (
	"fmt"
	"runtime"
	"sort"
)

func workingCopyPrivateMode(entry WorkingCopyEntry) uint32 {
	if runtime.GOOS == "windows" {
		if entry.Kind == "directory" {
			return 0777
		}
		return 0666
	}
	if entry.Kind == "directory" {
		return 0700
	}
	mode := uint32(0600)
	if entry.Mode&0111 != 0 {
		mode |= 0100
	}
	return mode
}

// The copy's private permissions must not accidentally become proposed chmods
// to the original. V1 preserves source modes on existing entries and rejects
// explicit mode edits; new entries retain their observed mode.
func diffWorkingCopy(base, result *WorkingCopyManifest) ([]FileChange, int64, error) {
	before, after := map[string]WorkingCopyEntry{}, map[string]WorkingCopyEntry{}
	names := map[string]bool{}
	for _, entry := range base.Entries {
		before[entry.Path] = entry
		names[entry.Path] = true
	}
	for _, entry := range result.Entries {
		after[entry.Path] = entry
		names[entry.Path] = true
	}
	paths := make([]string, 0, len(names))
	for name := range names {
		paths = append(paths, name)
	}
	sort.Strings(paths)
	changes := []FileChange{}
	blobs := map[string]bool{}
	var bytes int64
	for _, name := range paths {
		old, had := before[name]
		next, has := after[name]
		if had && has && old.Kind == next.Kind {
			if next.Mode != workingCopyPrivateMode(old) {
				return nil, 0, fmt.Errorf("mode changes require review outside the v1 change-set backend (%s): %w", name, ErrUnsupported)
			}
			next.Mode = old.Mode
			if old == next {
				continue
			}
		}
		change := FileChange{Path: name, Kind: "update"}
		if had {
			change.Base = &old
		} else {
			change.Kind = "create"
		}
		if has {
			change.Result = &next
			if next.Kind == "file" && !blobs[next.SHA256] {
				bytes += next.Bytes
				blobs[next.SHA256] = true
			}
		} else {
			change.Kind = "delete"
		}
		changes = append(changes, change)
	}
	return changes, bytes, nil
}
