package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateStateDarwinPendingCaseAliasRemainsFenced(t *testing.T) {
	parent, _, _ := darwinCaseAliasFixture(t)
	requested := filepath.Join(parent, "Future", "browser")
	pending, err := locateHostProtection(requested)
	if err != nil {
		t.Fatal(err)
	}
	store := &hostProtectionTransaction{records: map[string]hostProtectionRecord{
		"pending": {Kind: "browser", State: "pending", Root: pending},
	}}
	if err := store.checkLiveLocations(context.Background()); err != nil {
		t.Fatalf("unchanged pending root rejected: %v", err)
	}
	if err := os.Mkdir(filepath.Join(parent, "future"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := store.checkLiveLocations(context.Background()); !errors.Is(err, ErrHostProtectionUncertain) {
		t.Fatalf("newly resolved case alias escaped pending admission fence: %v", err)
	}
}

func darwinCaseAliasFixture(t *testing.T) (string, string, string) {
	t.Helper()
	parent := t.TempDir()
	original := filepath.Join(parent, "BrowserState")
	alias := filepath.Join(parent, "browserstate")
	if err := os.Mkdir(original, 0700); err != nil {
		t.Fatal(err)
	}
	actual, err := os.Stat(original)
	if err != nil {
		t.Fatal(err)
	}
	alternative, err := os.Stat(alias)
	if os.IsNotExist(err) {
		t.Skip("fixture filesystem is case-sensitive; case-alias qualification requires a case-insensitive volume")
	}
	if err != nil || !os.SameFile(actual, alternative) {
		t.Fatalf("case alias did not identify the created directory: %v", err)
	}
	return parent, original, alias
}

func TestPrivateStateDarwinCaseAliases(t *testing.T) {
	parent, original, alias := darwinCaseAliasFixture(t)
	child := filepath.Join(original, "child")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct{ name, root, protected string }{
		{"equal", alias, original},
		{"equal reverse", original, alias},
		{"ancestor", alias, child},
		{"descendant", filepath.Join(alias, "child"), original},
		{"missing protected tail", alias, filepath.Join(original, "missing", "uploads")},
		{"missing aliased tail", original, filepath.Join(alias, "missing", "uploads")},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			if err := PrivateStateDisjointFromRoot(fixture.root, fixture.protected); !errors.Is(err, ErrAuthority) {
				t.Fatalf("overlapping case alias accepted: %v", err)
			}
		})
	}
	disjoint := filepath.Join(parent, "other")
	if err := os.Mkdir(disjoint, 0700); err != nil {
		t.Fatal(err)
	}
	if err := PrivateStateDisjointFromRoot(disjoint, alias); err != nil {
		t.Fatalf("disjoint sibling rejected: %v", err)
	}
	first, err := locateHostProtection(original)
	if err != nil {
		t.Fatal(err)
	}
	second, err := locateHostProtection(alias)
	if err != nil || first != second {
		t.Fatalf("registry aliases have different canonical identities: %v", err)
	}
	missing, err := locateHostProtection(filepath.Join(alias, "missing", "uploads"))
	if err != nil || missing.Anchor != first.Anchor || missing.Identity != first.Identity || !protectionOverlap(first.Path, missing.Path) {
		t.Fatalf("missing tail lost canonical ancestor: %v", err)
	}
}

func TestPrivateStateDarwinNativePathValidation(t *testing.T) {
	root := t.TempDir()
	canonical, err := canonicalPrivateLocation(root)
	if err != nil {
		t.Fatal(err)
	}
	want, err := folderIdentity(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := folderIdentity(canonical)
	if err != nil || got != want {
		t.Fatalf("native path changed directory identity: %v", err)
	}
	if _, err := canonicalPrivateLocation(filepath.Join(root, "absent")); !os.IsNotExist(err) {
		t.Fatalf("missing path no longer distinguishes reservation: %v", err)
	}
	file := filepath.Join(root, "regular")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := canonicalPrivateLocation(file); err == nil {
		t.Fatal("regular file accepted as private directory")
	}
	link := filepath.Join(root, "alias")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	if resolved, err := canonicalPrivateLocation(link); err != nil || resolved != canonical {
		t.Fatalf("symlink did not resolve to opened directory: %v", err)
	}
}

func TestPrivateStateDarwinDistinctCaseSensitiveDirectories(t *testing.T) {
	parent := t.TempDir()
	first, second := filepath.Join(parent, "Case"), filepath.Join(parent, "case")
	if err := os.Mkdir(first, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(second); err == nil {
		t.Skip("fixture filesystem is case-insensitive; distinct case spellings require a case-sensitive volume")
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.Mkdir(second, 0700); err != nil {
		t.Fatal(err)
	}
	if err := PrivateStateDisjointFromRoot(first, second); err != nil {
		t.Fatalf("distinct case-sensitive directories conflated: %v", err)
	}
}
