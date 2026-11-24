package execution

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestProtectedHostStateRejectsExtendedWindowsAliases(t *testing.T) {
	parent := t.TempDir()
	private := filepath.Join(parent, "computer")
	child := filepath.Join(private, "uploads")
	if err := os.MkdirAll(child, 0700); err != nil {
		t.Fatal(err)
	}
	extended := func(path string) string { return `\\?\` + path }
	for name, pair := range map[string][2]string{
		"equal-root":            {extended(private), private},
		"equal-protected":       {private, extended(private)},
		"ancestor":              {extended(parent), private},
		"mixed-separator-root":  {extended(filepath.ToSlash(parent)), private},
		"mixed-protected":       {parent, extended(filepath.ToSlash(private))},
		"descendant":            {extended(child), private},
		"missing-tail":          {extended(parent), filepath.Join(parent, "future", "computer")},
		"missing-extended-tail": {parent, extended(filepath.Join(parent, "future", "computer"))},
	} {
		t.Run(name, func(t *testing.T) {
			if err := PrivateStateDisjointFromRoot(pair[0], pair[1]); !errors.Is(err, ErrAuthority) {
				t.Fatalf("same physical workspace admitted through namespace alias: %v", err)
			}
		})
	}
	if err := PrivateStateDisjointFromRoot(extended(t.TempDir()), private); err != nil {
		t.Fatalf("disjoint extended workspace rejected: %v", err)
	}
	if err := PrivateStateDisjointFromRoot(extended(filepath.ToSlash(t.TempDir())), private); err != nil {
		t.Fatalf("disjoint mixed-separator extended workspace rejected: %v", err)
	}
}

func TestProtectedHostStateRejectsVolumeGUIDAliases(t *testing.T) {
	parent := t.TempDir()
	private := filepath.Join(parent, "computer")
	child := filepath.Join(private, "uploads")
	if err := os.MkdirAll(child, 0700); err != nil {
		t.Fatal(err)
	}
	canonical, err := canonicalPrivateLocation(private)
	if err != nil {
		t.Fatal(err)
	}
	ordinary, err := os.Stat(private)
	if err != nil {
		t.Fatal(err)
	}
	aliased, err := os.Stat(canonical)
	if err != nil || !os.SameFile(ordinary, aliased) {
		t.Fatalf("canonical fixture does not address the same directory: %v", err)
	}
	for name, pair := range map[string][2]string{
		"equal-root":      {canonical, private},
		"equal-protected": {private, canonical},
		"ancestor":        {filepath.Dir(canonical), private},
		"descendant":      {filepath.Join(canonical, "uploads"), private},
		"missing-tail":    {parent, filepath.Join(filepath.Dir(canonical), "future", "computer")},
	} {
		t.Run(name, func(t *testing.T) {
			if err := PrivateStateDisjointFromRoot(pair[0], pair[1]); !errors.Is(err, ErrAuthority) {
				t.Fatalf("same physical workspace admitted through volume alias: %v", err)
			}
		})
	}
	if err := PrivateStateDisjointFromRoot(t.TempDir(), canonical); err != nil {
		t.Fatalf("disjoint volume-GUID private root rejected: %v", err)
	}
}
