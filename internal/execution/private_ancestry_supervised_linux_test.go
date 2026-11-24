package execution

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestSupervisedStorageAncestryNamespace(t *testing.T) {
	if mode := os.Getenv("OFFGRID_STORAGE_PROOF_CASE"); mode != "" {
		journal := os.Getenv("OFFGRID_STORAGE_PROOF_JOURNAL")
		if mode == "foreign owner" {
			info, err := os.Lstat(filepath.Dir(journal))
			if err != nil {
				t.Fatal(err)
			}
			stat, ok := info.Sys().(*syscall.Stat_t)
			if !ok || stat.Uid == uint32(os.Geteuid()) {
				t.Fatal("foreign owner did not remain unmapped")
			}
			t.Logf("namespace observed foreign UID=%d euid=%d", stat.Uid, os.Geteuid())
		}
		registry, err := SupervisedHostProtectionRegistry(os.Getenv("OFFGRID_STORAGE_PROOF"), journal)
		if mode == "valid" {
			if err != nil || registry.checkAncestry(journal) != nil {
				t.Fatal("valid original-host ancestry rejected", err)
			}
			if _, err := captureSupervisedStorageAncestry(journal); err == nil {
				t.Fatal("remapped UID zero minted new host-root authority")
			}
			if err := registry.checkAncestry(filepath.Dir(journal)); err == nil {
				t.Fatal("proof authorized an extra scope")
			}
		} else if err == nil {
			t.Fatal("invalid ancestry proof accepted", mode)
		}
		return
	}
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "config"))
	journal := filepath.Join(base, "journal")
	environment, err := supervisedProcessEnvironment(journal)
	if err != nil {
		t.Fatal(err)
	}
	var original supervisedRuntime
	if err = json.Unmarshal([]byte(strings.TrimPrefix(environment[0], "OFFGRID_EXECUTOR_PROCESS_RUNTIME=")), &original); err != nil || original.Storage == nil {
		t.Fatal(err)
	}
	cases := map[string]func(*supervisedRuntime){
		"valid":                func(*supervisedRuntime) {},
		"old version":          func(pin *supervisedRuntime) { pin.Version = 1 },
		"missing storage":      func(pin *supervisedRuntime) { pin.Storage = nil },
		"missing roots":        func(pin *supervisedRuntime) { pin.Storage.Roots = nil },
		"unwitnessed ancestor": func(pin *supervisedRuntime) { pin.Storage.Roots = pin.Storage.Roots[1:] },
		"identity changed":     func(pin *supervisedRuntime) { pin.Storage.Roots[0].Identity += "-changed" },
		"mode changed":         func(pin *supervisedRuntime) { pin.Storage.Roots[0].Mode ^= os.ModeSticky },
		"duplicate":            func(pin *supervisedRuntime) { pin.Storage.Roots = append(pin.Storage.Roots, pin.Storage.Roots[0]) },
		"extra ancestor": func(pin *supervisedRuntime) {
			root := pin.Storage.Roots[0]
			root.Path = filepath.Join(base, "unapproved")
			pin.Storage.Roots = append(pin.Storage.Roots, root)
		},
		"foreign journal":   func(pin *supervisedRuntime) { pin.Storage.Journal += "-foreign" },
		"foreign registry":  func(pin *supervisedRuntime) { pin.Storage.Registry += "-foreign" },
		"wrong UID mapping": func(pin *supervisedRuntime) { pin.Storage.HostUID++ },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			if name == "unwitnessed ancestor" && original.Storage.HostUID == 0 {
				t.Skip("unmapped host-root negative requires a distinct non-root caller UID")
			}
			encoded, _ := json.Marshal(original)
			var pin supervisedRuntime
			if err := json.Unmarshal(encoded, &pin); err != nil {
				t.Fatal(err)
			}
			mutate(&pin)
			runStorageProofChild(t, pin, journal, name)
			if _, err := os.Stat(journal); !os.IsNotExist(err) {
				t.Fatal("proof verification created journal", err)
			}
			if _, err := os.Stat(filepath.Join(base, "config")); !os.IsNotExist(err) {
				t.Fatal("proof verification initialized account", err)
			}
		})
	}
	// The same payload is not authority in the original host namespace.
	raw, _ := json.Marshal(original)
	if _, err := SupervisedHostProtectionRegistry(string(raw), journal); err == nil {
		t.Fatal("proof accepted outside its owned namespace")
	}
}

func runStorageProofChild(t *testing.T, pin supervisedRuntime, journal, mode string) {
	t.Helper()
	encoded, err := json.Marshal(pin)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestSupervisedStorageAncestryNamespace$", "-test.v")
	command.Env = append(processEnvironment(), "OFFGRID_STORAGE_PROOF_CASE="+mode, "OFFGRID_STORAGE_PROOF_JOURNAL="+journal, "OFFGRID_STORAGE_PROOF="+string(encoded))
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	stop, wait, release, err := startOwned(command)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	done := make(chan error, 1)
	go func() { done <- wait() }()
	select {
	case err = <-done:
	case <-time.After(5 * time.Second):
		_ = stop()
		err = <-done
		t.Fatal("namespace proof check timed out", err, output.String())
	}
	if err != nil {
		t.Fatal(err, output.String())
	}
	if mode == "foreign owner" {
		t.Log(output.String())
	}
}

// Explicit metadata-only qualification: never list/read directory contents,
// initialize a journal, or change a foreign owner's paths/permissions.
func TestSupervisedStorageForeignAncestor(t *testing.T) {
	foreign := os.Getenv("OFFGRID_TEST_FOREIGN_ANCESTOR")
	if foreign == "" {
		t.Skip("explicit readable foreign-owned directory required for metadata-only qualification")
	}
	info, err := os.Lstat(foreign)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0022 != 0 {
		t.Fatal("foreign fixture must be a non-writable regular directory", err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid == 0 || stat.Uid == uint32(os.Geteuid()) {
		t.Fatal("fixture owner must differ from root and caller")
	}
	t.Logf("metadata-only foreign ancestor UID=%d mode=%o", stat.Uid, info.Mode().Perm())
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "config"))
	journal := filepath.Join(foreign, "offgrid-metadata-only-"+protectionHash(base)[:16])
	if _, err := os.Lstat(journal); !os.IsNotExist(err) {
		t.Fatal("metadata-only probe path must not exist", err)
	}
	if _, err := captureSupervisedStorageAncestry(journal); err == nil {
		t.Fatal("parent minted host-root witness for foreign owner")
	}
	environment, err := supervisedProcessEnvironment(filepath.Join(base, "journal"))
	if err != nil {
		t.Fatal(err)
	}
	var pin supervisedRuntime
	if err := json.Unmarshal([]byte(strings.TrimPrefix(environment[0], "OFFGRID_EXECUTOR_PROCESS_RUNTIME=")), &pin); err != nil {
		t.Fatal(err)
	}
	// Even a changed scope cannot turn a foreign unmapped owner into root.
	pin.Storage.Journal = journal
	runStorageProofChild(t, pin, journal, "foreign owner")
	if _, err := os.Lstat(journal); !os.IsNotExist(err) {
		t.Fatal("metadata-only inspection created foreign state", err)
	}
}
