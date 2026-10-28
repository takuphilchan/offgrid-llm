package execution

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRememberedFolderTrustBindsEveryAuthorityDimension(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "workspace")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	store := FolderTrustStore{Directory: filepath.Join(base, "private")}
	a := LocalApproval{Root: root, Service: "http://127.0.0.1:11611", Workspace: "service-1", Actor: "alice", Host: "host", Backend: "local", Permissions: Permissions{Read: true}}
	if err := store.Remember(a); err != nil {
		t.Fatal(err)
	}
	if ok, err := store.Lookup(a); err != nil || !ok {
		t.Fatal("explicit trust not reusable", err)
	}
	for _, dimension := range []string{"actor", "host", "service", "workspace", "permission", "subfolder", "replacement", "backend"} {
		t.Run(dimension, func(t *testing.T) {
			changed := a
			switch dimension {
			case "actor":
				changed.Actor = "bob"
			case "host":
				changed.Host = "other"
			case "service":
				changed.Service = "http://127.0.0.1:11612"
			case "workspace":
				changed.Workspace = "new-installation"
			case "permission":
				changed.Permissions.Write = true
			case "backend":
				changed.Backend = "isolated_linux"
			case "subfolder":
				changed.Root = filepath.Join(root, "sub")
				if err := os.Mkdir(changed.Root, 0700); err != nil {
					t.Fatal(err)
				}
			case "replacement":
				old := root + "-old"
				if err := os.Rename(root, old); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(root, 0700); err != nil {
					t.Fatal(err)
				}
				defer func() { _ = os.Remove(root); _ = os.Rename(old, root) }()
			}
			if ok, _ := store.Lookup(changed); ok {
				t.Fatal("trust widened", dimension)
			}
		})
	}
	list, err := store.List()
	if err != nil || len(list) != 1 {
		t.Fatal(list, err)
	}
	if err = store.Revoke(list[0].ID); err != nil {
		t.Fatal(err)
	}
	if ok, err := store.Lookup(a); err != nil || ok {
		t.Fatal("revoked trust survived", err)
	}
}

func TestRememberedTrustRejectsCorruptAndWorkspaceOwnedState(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "workspace")
	_ = os.Mkdir(root, 0700)
	a := LocalApproval{Root: root, Service: "http://127.0.0.1:11611", Workspace: "workspace", Actor: "alice", Host: "host", Backend: "local", Permissions: Permissions{Read: true}}
	inside := FolderTrustStore{Directory: filepath.Join(root, "trust")}
	if err := inside.Remember(a); err == nil {
		t.Fatal("model-writable trust store")
	}
	store := FolderTrustStore{Directory: filepath.Join(base, "private")}
	if err := store.Remember(a); err != nil {
		t.Fatal(err)
	}
	list, _ := store.List()
	file := filepath.Join(store.Directory, list[0].ID+".json")
	if err := os.WriteFile(file, []byte(`{"version":99}`), 0600); err != nil {
		t.Fatal(err)
	}
	if ok, err := store.Lookup(a); ok || err == nil {
		t.Fatal("corrupt trust authorized")
	}
	if err := store.Revoke("../outside"); err == nil {
		t.Fatal("unsafe revoke path")
	}
}
