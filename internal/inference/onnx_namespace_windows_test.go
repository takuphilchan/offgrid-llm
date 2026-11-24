package inference

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"testing"
)

func TestONNXRuntimeLeaseRejectsNewImportableFiles(t *testing.T) {
	root := t.TempDir()
	privateFixtureDirectory(t, root)
	site := filepath.Join(root, "Lib", "site-packages")
	if err := os.MkdirAll(site, 0700); err != nil {
		t.Fatal(err)
	}
	pins := map[string]string{}
	for name, data := range map[string][]byte{"python._pth": []byte("Lib\n"), "Lib/site-packages/existing.py": []byte("fixture")} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), data, 0600); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		pins[name] = "sha256:" + hex.EncodeToString(digest[:])
	}
	lease := &managedOllamaFiles{}
	defer lease.close()
	if err := lease.onnxRuntimeTree(context.Background(), root, pins); err != nil {
		if errors.Is(err, errONNXMutableRuntime) {
			return
		} // Unsafe namespace must be rejected before admission.
		t.Fatal(err)
	}
	for _, directory := range []string{root, site} {
		if err := os.WriteFile(filepath.Join(directory, "new_unpinned.py"), []byte("new importable bytes"), 0600); err == nil {
			t.Errorf("active runtime lease allowed new importable file in %s", directory)
		}
	}
}

func TestONNXRuntimeACLPolicyRejectsWritableOwnersAndNamespaces(t *testing.T) {
	for _, test := range []struct {
		sddl string
		okay bool
	}{
		{"O:BAD:P(A;;FA;;;SY)(A;;FA;;;BA)(A;;GRGX;;;BU)", true},
		{"O:BAD:P(A;;FA;;;SY)(A;;FA;;;BA)(A;;0x2;;;BU)", false},
		{"O:BAD:P(A;;FA;;;SY)(A;;FA;;;BA)(A;;WD;;;BU)", false},
		{"O:BUD:P(A;;GRGX;;;BU)", false},
		{"O:BAD:NO_ACCESS_CONTROL", false},
	} {
		sd, err := windows.SecurityDescriptorFromString(test.sddl)
		if err != nil {
			t.Fatal(err)
		}
		err = validateONNXRuntimeSecurityDescriptor(sd, false)
		if (err == nil) != test.okay {
			t.Fatalf("ACL policy mismatch %s: %v", test.sddl, err)
		}
	}
}
