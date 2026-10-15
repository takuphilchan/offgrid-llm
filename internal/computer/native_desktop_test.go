package computer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDesktopBindingSeparatesMissingSelectionFromAuthorityFailure(t *testing.T) {
	c, d := desktopControllerFixture(t)
	consent, _ := desktopConsentFixture()
	if _, err := c.Binding(consent.Binding); !errors.Is(err, ErrTargetNotSelected) {
		t.Fatalf("missing selection: %v", err)
	}
	foreign := consent.Binding
	foreign.Target.OSSession = "foreign"
	if _, err := c.Binding(foreign); !errors.Is(err, ErrControlScope) {
		t.Fatalf("foreign scope disclosed selection: %v", err)
	}
	d.identity.OSSession = "locked-or-replaced"
	if _, err := c.Binding(consent.Binding); !errors.Is(err, ErrControlScope) {
		t.Fatalf("lost session treated as missing target: %v", err)
	}
	d.identity = c.identity
	c.selected, c.selectedApp = d.targets[0], "wrong-app"
	if _, err := c.Binding(consent.Binding); !errors.Is(err, ErrControlScope) {
		t.Fatalf("mismatched target: %v", err)
	}
}

type desktopFixtureAdapter struct {
	NativeAdapter
	identity TargetIdentity
	targets  []NativeTarget
	paths    map[string]string
	selected NativeTarget
	focus    int
}

func (d *desktopFixtureAdapter) DesktopIdentity() (TargetIdentity, error) { return d.identity, nil }
func (d *desktopFixtureAdapter) TargetExecutable(t NativeTarget) (string, error) {
	return d.paths[t.Identity.ID], nil
}
func (d *desktopFixtureAdapter) Targets() ([]NativeTarget, error) { return d.targets, nil }
func (d *desktopFixtureAdapter) Select(id string) (NativeTarget, error) {
	for _, t := range d.targets {
		if t.Identity.ID == id {
			d.selected = t
			return t, nil
		}
	}
	return NativeTarget{}, ErrControlScope
}
func (d *desktopFixtureAdapter) FocusSelected() error { d.focus++; return nil }
func (d *desktopFixtureAdapter) Observe(context.Context) (NativeView, error) {
	return NativeView{Observation: ControlObservation{ID: "view", Target: d.selected.Identity, Sequence: 1, CapturedAt: time.Now(), StateDigest: strings.Repeat("a", 64)}}, nil
}

// Production discovery stores canonical paths. Mirror that contract for owned
// fixtures: macOS temp roots are symlinked and Windows runners may use 8.3 names.
func canonicalApplicationFixtureDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func desktopControllerFixture(t *testing.T) (*DesktopController, *desktopFixtureAdapter) {
	t.Helper()
	consent, _ := desktopConsentFixture()
	d := &desktopFixtureAdapter{identity: consent.Binding.Target, paths: map[string]string{}}
	c := &DesktopController{driver: d, provider: d, identity: d.identity, apps: map[string]installedApplication{}, targets: map[string]NativeTarget{}, prepared: map[string]PreparedControlAction{}}
	for _, id := range []string{"chrome", "editor"} {
		path := filepath.Join(canonicalApplicationFixtureDir(t), id)
		if err := os.WriteFile(path, []byte("owned synthetic executable"), 0600); err != nil {
			t.Fatal(err)
		}
		info, _ := os.Stat(path)
		c.apps[id] = installedApplication{NativeApplication: NativeApplication{ID: id, Name: id}, path: path, identity: info}
		identity := d.identity
		identity.ID = id + "-window"
		identity.ProcessGeneration = id + "-process"
		identity.Surface = id + "-surface"
		d.targets = append(d.targets, NativeTarget{Identity: identity, Title: id})
		d.paths[identity.ID] = path
	}
	return c, d
}
func TestDesktopControllerCatalogLaunchReadinessAndSwitch(t *testing.T) {
	c, d := desktopControllerFixture(t)
	launches := 0
	c.launch = func(context.Context, installedApplication) error { launches++; return nil }
	apps, err := c.Applications()
	if err != nil || len(apps) != 2 {
		t.Fatal(apps, err)
	}
	a, err := c.Prepare(Operation{Kind: "open_application", Application: "chrome"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := c.Dispatch(context.Background(), a)
	if err != nil || result.Outcome != "dispatched" || launches != 1 {
		t.Fatal(result, err)
	}
	if _, err = c.Dispatch(context.Background(), a); err == nil || launches != 1 {
		t.Fatal("launch replayed")
	}
	// Reused process/surface is valid; no new PID and no second launch required.
	for i := 0; i < 2; i++ {
		v, err := c.VerifyApplication(context.Background(), "chrome")
		if err != nil || !v.Verified {
			t.Fatal(v, err)
		}
	}
	if launches != 1 || d.focus != 0 {
		t.Fatal("verification changed the desktop")
	}
	_, err = c.Surfaces()
	if err != nil {
		t.Fatal(err)
	}
	a, err = c.Prepare(Operation{Kind: "focus_surface", Target: "editor-window"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Dispatch(context.Background(), a); err != nil || d.focus != 1 {
		t.Fatal(err)
	}
	c.observed = time.Now().Add(-16 * time.Second)
	if _, err = c.Prepare(Operation{Kind: "focus_surface", Target: "chrome-window"}); err == nil {
		t.Fatal("stale target prepared")
	}
	d.identity.OSSession = "another-session"
	if _, err = c.Applications(); err == nil {
		t.Fatal("OS session changed")
	}
}
func TestDesktopControllerRejectsReplacementAndCancelledLaunch(t *testing.T) {
	c, d := desktopControllerFixture(t)
	launches := 0
	c.launch = func(context.Context, installedApplication) error { launches++; return nil }
	if _, err := c.Prepare(Operation{Kind: "open_application", Application: "invented"}); err == nil {
		t.Fatal("unknown executable")
	}
	a, _ := c.Prepare(Operation{Kind: "open_application", Application: "chrome"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Dispatch(ctx, a); err == nil || launches != 0 {
		t.Fatal("cancelled launch ran")
	}
	a, _ = c.Prepare(Operation{Kind: "open_application", Application: "chrome"})
	app := c.apps["chrome"]
	if err := os.Rename(app.path, app.path+".previous"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(app.path, []byte("replaced"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Dispatch(context.Background(), a); err == nil || launches != 0 {
		t.Fatal("replaced binary launched")
	}
	surfaces, err := c.Surfaces()
	if err != nil || len(surfaces) != 1 {
		t.Fatal("replacement surfaced", surfaces, err)
	}
	d.targets = nil
	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	v, err := c.VerifyApplication(ctx, "editor")
	if err != nil || v.Verified || time.Since(start) > time.Second {
		t.Fatal("unbounded or invented readiness", v, err)
	}
}
func TestApplicationChildEnvironmentHasNoOffGridAuthority(t *testing.T) {
	got := cleanApplicationEnvironment([]string{"PATH=system", "OFFGRID_API_KEY=secret", "offgrid_session=secret", "ELECTRON_RUN_AS_NODE=1", "NODE_OPTIONS=payload", "LD_PRELOAD=payload", "PYTHONPATH=payload"})
	if len(got) != 1 || got[0] != "PATH=system" {
		t.Fatal(got)
	}
}

func TestDesktopCatalogRefreshPreservesOnlyUnchangedIdentities(t *testing.T) {
	c, _ := desktopControllerFixture(t)
	chrome := c.apps["chrome"]
	edge := c.apps["editor"]
	edge.ID, edge.Name = "new-edge", "Microsoft Edge"
	delete(c.apps, "editor") // unavailable when the long-lived worker started
	c.discover = func() ([]installedApplication, error) {
		found := chrome
		found.ID = "fresh-chrome"
		return []installedApplication{found, edge}, nil
	}
	apps, err := c.Applications()
	if err != nil || len(apps) != 2 || c.apps["chrome"].path != chrome.path || c.apps["new-edge"].Name != "Microsoft Edge" {
		t.Fatal("stale catalog or unstable identity", apps, err)
	}
	a, err := c.Prepare(Operation{Kind: "open_application", Application: "new-edge"})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(edge.path, edge.path+".previous"); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(edge.path, []byte("updated installed app"), 0600); err != nil {
		t.Fatal(err)
	}
	edge.identity, _ = os.Lstat(edge.path)
	edge.ID = "updated-edge"
	if _, err = c.Applications(); err != nil {
		t.Fatal(err)
	}
	if _, exists := c.apps["new-edge"]; exists || c.IsPrepared(a) {
		t.Fatal("replacement reused an old identity or preparation")
	}
	if _, err = c.Prepare(Operation{Kind: "open_application", Application: "new-edge"}); err == nil {
		t.Fatal("old identity still authorized")
	}
}
