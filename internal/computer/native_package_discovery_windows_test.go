//go:build windows && amd64

package computer

import (
	"os"
	"runtime"
	"strings"
	"testing"

	ole "github.com/go-ole/go-ole"
)

// Read-only OS registration check, deliberately separate from native activation
// and input tests. The explicit AUMID must already be installed; nothing is
// launched, selected, downloaded or modified by this test.
func TestRegisteredPackageDiscoveryReadOnly(t *testing.T) {
	aumid := os.Getenv("OFFGRID_TEST_DISCOVERY_AUMID")
	if aumid == "" {
		t.Skip("requires an explicitly selected installed registration; reads only")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED)
	if err != nil {
		if e, ok := err.(*ole.OleError); !ok || e.Code() != 1 {
			t.Fatal(err)
		}
	}
	defer ole.CoUninitialize()
	family, entry, ok := strings.Cut(aumid, "!")
	if !ok {
		t.Fatal("expected a packaged AUMID")
	}
	names, err := packageFullNames(family)
	if err != nil || len(names) == 0 {
		t.Fatalf("package registrations: %d, %v", len(names), err)
	}
	for _, name := range names {
		root, err := packageInstallPath(name)
		if err != nil {
			t.Fatalf("registered install path: %v", err)
		}
		path, _, err := packageEntry(root, entry)
		if err != nil {
			t.Fatalf("manifest entry: %v", err)
		}
		if !ordinaryGUIExecutable(path, "") {
			t.Fatal("registered entry not admitted as a GUI executable")
		}
	}
	app, err := resolvePackagedApplication(aumid, "")
	if err != nil {
		t.Fatalf("resolve registration: %v", err)
	}
	app.validateRegistration = validatePackagedRegistration
	if err := checkInstalledApplication(app); err != nil {
		t.Fatalf("validate registration: %v", err)
	}
	apps, err := installedNativeApplications()
	if err != nil {
		t.Fatalf("installed catalog enumeration: %v", err)
	}
	for _, candidate := range apps {
		if candidate.packageIdentity != nil && candidate.packageIdentity.AUMID == aumid {
			t.Logf("validated package is discoverable among %d installed entries; no activation or input", len(apps))
			return
		}
	}
	t.Fatalf("valid registered app omitted from %d installed catalog entries", len(apps))
}
