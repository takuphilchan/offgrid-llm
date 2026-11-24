package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestHostProtectionPendingChangedSuffixFencesAdmission(t *testing.T) {
	for _, kind := range []string{"file", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			registry := protectionTestRegistry(t)
			parent := protectionTestRoot(t)
			component := filepath.Join(parent, "future")
			if _, err := registry.RegisterBrowserRoot(ctx, filepath.Join(component, "browser")); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "file":
				if err := os.WriteFile(component, []byte("replacement"), 0600); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink(protectionTestRoot(t), component); err != nil {
					t.Skipf("fixture symlink unavailable: %v", err)
				}
			}
			before := protectionRows(t, registry)
			if _, err := registry.RegisterBrowserRoot(ctx, protectionTestRoot(t)); !errors.Is(err, ErrHostProtectionUncertain) {
				t.Fatalf("changed pending suffix did not fence admission: %v", err)
			}
			if len(protectionRows(t, registry)) != len(before) {
				t.Fatal("uncertain admission published a row")
			}
		})
	}
}

func TestHostProtectionPendingExpectedCreationRetainsOriginalClaim(t *testing.T) {
	ctx := context.Background()
	registry := protectionTestRegistry(t)
	parent := protectionTestRoot(t)
	component := filepath.Join(parent, "future")
	root := filepath.Join(component, "browser")
	claim, err := registry.RegisterBrowserRoot(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{component, root} {
		if err = EnsurePrivateDirectory(directory); err != nil {
			t.Fatal(err)
		}
		if _, err = registry.RegisterBrowserRoot(ctx, protectionTestRoot(t)); err != nil {
			t.Fatalf("expected missing suffix creation fenced unrelated admission: %v", err)
		}
		row := protectionRows(t, registry)[claim.ID()]
		if row.State != "pending" || row.Root != claim.record.Root {
			t.Fatal("admission adopted a newly created pending root")
		}
	}
	if _, err = registry.RegisterBrowserRoot(ctx, root); !errors.Is(err, ErrHostProtectionUncertain) {
		t.Fatalf("reopening adopted pending root: %v", err)
	}
	if err = claim.Finalize(ctx); err != nil {
		t.Fatalf("original capability could not finalize expected root: %v", err)
	}
}
