//go:build windows && amd64

package computer

import (
	"context"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Opt-in real OS activation, deliberately separate from read-only discovery and
// owned fixture tests. Never run against the signed-in user's personal desktop.
func TestNativeWindowsRegisteredPackageActivation(t *testing.T) {
	if os.Getenv("OFFGRID_TEST_NATIVE_WINDOWS") != "1" || os.Getenv("OFFGRID_TEST_NATIVE_PACKAGE_APPLICATION") == "" {
		t.Skip("requires explicitly selected package in an isolated account")
	}
	current, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	account := current.Username
	if i := strings.LastIndex(account, `\`); i >= 0 {
		account = account[i+1:]
	}
	if os.Getenv("OFFGRID_NATIVE_ISOLATED_DESKTOP") != "1" || !strings.HasPrefix(strings.ToLower(account), "offgridtest-") {
		t.Fatal("refusing native package activation outside a disposable OffGridTest-* account")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	driver, err := NewNativeWindows()
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	c, err := NewDesktopController(driver)
	if err != nil {
		t.Fatal(err)
	}
	selected := ""
	for id, app := range c.apps {
		if app.packageIdentity != nil && app.packageIdentity.AUMID == os.Getenv("OFFGRID_TEST_NATIVE_PACKAGE_APPLICATION") {
			selected = id
			break
		}
	}
	if selected == "" {
		t.Fatal("requested package is not a supported registered GUI application")
	}
	a, err := c.Prepare(Operation{Kind: "open_application", Application: selected})
	if err != nil {
		t.Fatal(err)
	}
	journal, err := OpenNativeJournal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer journal.Close()
	consent, step := desktopConsentFixture()
	consent.Binding.Target = c.Identity()
	consent.Desktop.Binding = consent.Binding
	step.Binding = consent.Binding
	step.Actions = []PreparedControlAction{a}
	supervisor := NewNativeSupervisor(journal, consent, c.Dispatch)
	defer supervisor.Stop()
	grant, err := supervisor.Approve(step, func(BoundedStep) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	result, err := supervisor.Execute(step, grant.ID)
	if err != nil || len(result) != 1 || result[0].Outcome != "dispatched" {
		t.Fatal(result, err)
	}
	for i := 0; i < 2; i++ {
		view, err := c.VerifyApplication(context.Background(), selected)
		if err != nil || !view.Verified || view.Target == nil || view.View == nil {
			t.Fatal("registered package did not become independently observable", view.Readiness, err)
		}
		identity, err := driver.TargetApplication(NativeTarget{Identity: *view.Target})
		if err != nil || identity.AUMID != c.apps[selected].packageIdentity.AUMID || identity.FullName != c.apps[selected].packageIdentity.FullName {
			t.Fatal("process registration mismatch", err)
		}
	}
	// Readiness must leave the activation journal intact; lost delivery returns
	// the prior dispatch result and cannot activate another app/window.
	grant, err = supervisor.Approve(step, func(BoundedStep) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	if _, err = supervisor.Execute(step, grant.ID); err != nil {
		t.Fatal(err)
	}
	t.Log("registered package activated once and independently observed twice; no document input, saves, or process termination")
}

func TestPackageEntryRejectsUnboundedOrExecutableMetadata(t *testing.T) {
	for _, test := range []struct {
		name, content string
		valid         bool
	}{
		{"valid", `<Package><Applications><Application Id="Editor" Executable="App\Editor.exe"/></Applications></Package>`, true},
		{"missing-executable", `<Package><Applications><Application Id="Editor" Executable="App\Missing.exe"/></Applications></Package>`, false},
		{"no-entry", `<Package><Applications><Application Id="Other" Executable="Editor.exe"/></Applications></Package>`, false},
		{"duplicate", `<Package><Applications><Application Id="Editor" Executable="A.exe"/><Application Id="Editor" Executable="B.exe"/></Applications></Package>`, false},
		{"traversal", `<Package><Applications><Application Id="Editor" Executable="..\outside.exe"/></Applications></Package>`, false},
		{"absolute", `<Package><Applications><Application Id="Editor" Executable="C:\outside.exe"/></Applications></Package>`, false},
		{"arguments", `<Package><Applications><Application Id="Editor" Executable="Editor.exe --run"/></Applications></Package>`, false},
		{"dll", `<Package><Applications><Application Id="Editor" Executable="Editor.dll"/></Applications></Package>`, false},
		{"network", `<Package><Applications><Application Id="Editor" Executable="\\host\share\app.exe"/></Applications></Package>`, false},
		{"oversized", strings.Repeat(" ", 1024*1024+1), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := canonicalApplicationFixtureDir(t)
			if err := os.Mkdir(filepath.Join(root, "App"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "App", "Editor.exe"), []byte("metadata fixture, not launched"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "AppxManifest.xml"), []byte(test.content), 0600); err != nil {
				t.Fatal(err)
			}
			path, revision, err := packageEntry(root, "Editor")
			if (err == nil) != test.valid {
				t.Fatalf("path=%q revision=%q err=%v", path, revision, err)
			}
			if test.valid && (path != filepath.Join(root, "App", "Editor.exe") || len(revision) != 64) {
				t.Fatal(path, revision)
			}
		})
	}
}

func TestPackagedRegistrationNeverAcceptsModelStrings(t *testing.T) {
	for _, value := range []string{"", "notepad.exe", "Family", "Family!", "!Editor", "Family!Editor!Other", "Family!Editor --command", "Family!../Editor", "Family!Editor\x00"} {
		if _, err := resolvePackagedApplication(value, "test"); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
}
