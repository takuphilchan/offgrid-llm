package execution

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"
)

func protectionPathValid(path string) bool {
	return len(path) > 0 && len(path) <= 8192 && filepath.IsAbs(path) && utf8.ValidString(path) && !strings.ContainsRune(path, 0)
}

// Pin the nearest existing directory as well as every missing component. A
// missing leaf is a reservation, not permission to adopt a replacement later.
func locateHostProtection(path string) (hostProtectionLocation, error) {
	return locateHostProtectionWith(path, canonicalPrivateLocation)
}

func locateHostProtectionWith(path string, canonicalize func(string) (string, error)) (hostProtectionLocation, error) {
	if !protectionPathValid(path) {
		return hostProtectionLocation{}, ErrPath
	}
	cursor := filepath.Clean(path)
	var suffix []string
	appeared := 0
	for len(suffix) <= 64 {
		canonical, err := canonicalize(cursor)
		if err == nil {
			info, err := os.Stat(canonical)
			if err != nil || !info.IsDir() {
				return hostProtectionLocation{}, ErrPath
			}
			identity, err := folderIdentity(canonical)
			if err != nil {
				return hostProtectionLocation{}, err
			}
			for left, right := 0, len(suffix)-1; left < right; left, right = left+1, right-1 {
				suffix[left], suffix[right] = suffix[right], suffix[left]
			}
			tail := filepath.Join(suffix...)
			if len(suffix) == 0 {
				tail = ""
			}
			resolved := canonical
			if tail != "" {
				resolved = filepath.Join(canonical, tail)
			}
			if !protectionPathValid(resolved) {
				return hostProtectionLocation{}, ErrPath
			}
			return hostProtectionLocation{Path: resolved, Anchor: canonical, Identity: identity, Suffix: tail}, nil
		}
		if !os.IsNotExist(err) {
			return hostProtectionLocation{}, ErrPath
		}
		if info, err := os.Lstat(cursor); err == nil {
			// Another supervisor can create the registry after our missing-path
			// probe and before this check. Re-open a real directory to establish
			// its physical identity; never accept a link or adopt lexical identity.
			// Bound repeated inconsistencies. Admission revalidates this identity
			// again under the account lock before publishing any reservation.
			if info.IsDir() && info.Mode()&os.ModeSymlink == 0 && appeared < 2 {
				appeared++
				continue
			}
			return hostProtectionLocation{}, ErrPath
		} else if !os.IsNotExist(err) {
			return hostProtectionLocation{}, ErrPath
		}
		parent := filepath.Dir(cursor)
		if parent == cursor {
			return hostProtectionLocation{}, ErrPath
		}
		suffix = append(suffix, filepath.Base(cursor))
		cursor = parent
	}
	return hostProtectionLocation{}, ErrHostProtectionLimit
}

func protectionOverlap(first, second string) bool {
	return containsPrivateLocation(first, second) || containsPrivateLocation(second, first)
}
func protectionSamePath(first, second string) bool {
	return containsPrivateLocation(first, second) && containsPrivateLocation(second, first)
}
func protectionPathKey(path string) string {
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
	}
	return protectionHash(path)
}

func protectionLocationValid(location hostProtectionLocation) bool {
	if !protectionPathValid(location.Path) || !protectionPathValid(location.Anchor) || !validIdentity(location.Identity) {
		return false
	}
	if location.Suffix == "" {
		return location.Path == location.Anchor
	}
	if filepath.IsAbs(location.Suffix) || filepath.Clean(location.Suffix) != location.Suffix || location.Suffix == "." || location.Suffix == ".." || strings.HasPrefix(location.Suffix, ".."+string(filepath.Separator)) {
		return false
	}
	return filepath.Join(location.Anchor, location.Suffix) == location.Path
}

// A renamed live workspace can otherwise escape path-overlap comparisons.
// Its saved identity does not reveal its new pathname; fence admission until
// the original owner settles it instead of treating a missing path as release.
func (s *hostProtectionTransaction) checkLiveLocations(ctx context.Context) error {
	checked := map[string]string{}
	for _, row := range s.records {
		if err := ctx.Err(); err != nil {
			return err
		}
		if row.State == "released" {
			continue
		}
		locations := []hostProtectionLocation{row.Root}
		if row.Journal != nil {
			locations = append(locations, *row.Journal)
		}
		for _, location := range locations {
			if location.Suffix != "" {
				// Missing components can appear as case aliases or redirected
				// directories without changing the pinned ancestor. Re-resolve
				// every pending path even when that ancestor was already checked;
				// never adopt the newer anchor or identity during admission.
				current, err := locateHostProtection(location.Path)
				if err != nil || !protectionSamePath(current.Path, location.Path) {
					return ErrHostProtectionUncertain
				}
			}
			key := protectionPathKey(location.Anchor)
			if identity, ok := checked[key]; ok {
				if identity != location.Identity {
					return ErrHostProtectionUncertain
				}
				continue
			}
			identity, err := folderIdentity(location.Anchor)
			if err != nil || identity != location.Identity {
				return ErrHostProtectionUncertain
			}
			canonical, err := canonicalPrivateLocation(location.Anchor)
			if err != nil || !protectionSamePath(canonical, location.Anchor) {
				return ErrHostProtectionUncertain
			}
			checked[key] = identity
		}
	}
	return nil
}
