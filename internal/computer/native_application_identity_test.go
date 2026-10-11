package computer

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type packagedFixtureAdapter struct {
	*desktopFixtureAdapter
	application processApplicationIdentity
}

func (d *packagedFixtureAdapter) TargetApplication(NativeTarget) (processApplicationIdentity, error) {
	return d.application, nil
}

func fixturePackage(c *DesktopController, d *desktopFixtureAdapter) (*packagedFixtureAdapter, packagedApplication) {
	pkg := packagedApplication{Family: "Fixture_123", Entry: "Editor", AUMID: "Fixture_123!Editor", FullName: "Fixture_1.0_x64_123", Root: "fixture-root", ManifestRevision: strings.Repeat("a", 64)}
	app := c.apps["editor"]
	app.kind, app.packageIdentity = applicationPackaged, &pkg
	app.validateRegistration = func(current packagedApplication) error {
		if current != pkg {
			return ErrControlScope
		}
		return nil
	}
	c.apps["editor"] = app
	driver := &packagedFixtureAdapter{desktopFixtureAdapter: d, application: processApplicationIdentity{AUMID: pkg.AUMID, FullName: pkg.FullName}}
	c.driver, c.provider = driver, driver
	return driver, pkg
}

func TestPackagedApplicationRequiresExactEntryRevisionAndProvider(t *testing.T) {
	c, d := desktopControllerFixture(t)
	driver, pkg := fixturePackage(c, d)
	target := d.targets[1]
	if got := c.applicationFor(target); got != "editor" {
		t.Fatal(got)
	}
	for _, identity := range []processApplicationIdentity{
		{AUMID: "Fixture_123!OtherEntry", FullName: pkg.FullName},
		{AUMID: pkg.AUMID, FullName: "Fixture_2.0_x64_123"},
		{AUMID: pkg.Family, FullName: pkg.FullName}, {},
	} {
		driver.application = identity
		if got := c.applicationFor(target); got != "" {
			t.Fatalf("forged package matched: %q", got)
		}
	}
	driver.application = processApplicationIdentity{AUMID: pkg.AUMID, FullName: pkg.FullName}
	c.provider = d
	if got := c.applicationFor(target); got != "" {
		t.Fatal("executable-only provider matched package")
	}
	c.provider = driver
	target.Identity.OSSession = "foreign-session"
	if got := c.applicationFor(target); got != "" {
		t.Fatal("foreign session matched package")
	}
}

func TestPackagedRegistrationChangePreventsDispatch(t *testing.T) {
	c, d := desktopControllerFixture(t)
	_, pkg := fixturePackage(c, d)
	launches := 0
	c.launch = func(context.Context, installedApplication) error { launches++; return nil }
	a, err := c.Prepare(Operation{Kind: "open_application", Application: "editor"})
	if err != nil {
		t.Fatal(err)
	}
	app := c.apps["editor"]
	app.validateRegistration = func(packagedApplication) error { return ErrControlScope }
	c.apps["editor"] = app
	if _, err := c.Dispatch(context.Background(), a); !errors.Is(err, ErrControlScope) || launches != 0 {
		t.Fatal("changed registration dispatched", err)
	}
	app.validateRegistration = nil
	if checkInstalledApplication(app) == nil {
		t.Fatal("forged unvalidated package admitted")
	}
	app.validateRegistration = func(packagedApplication) error { return nil }
	app.kind = applicationExecutable
	if checkInstalledApplication(app) == nil {
		t.Fatal("package downgraded to executable")
	}
	app.kind = applicationPackaged
	pkg.Entry = "OtherEntry"
	app.packageIdentity = &pkg
	if checkInstalledApplication(app) == nil {
		t.Fatal("inconsistent AUMID admitted")
	}
}

func TestPackagedActivationUncertaintyIsNeverJournaledAsNoEffect(t *testing.T) {
	c, d := desktopControllerFixture(t)
	fixturePackage(c, d)
	launches := 0
	c.launch = func(context.Context, installedApplication) error { launches++; return ErrUncertain }
	a, err := c.Prepare(Operation{Kind: "open_application", Application: "editor"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := c.Dispatch(context.Background(), a)
	if !errors.Is(err, ErrUncertain) || result.Outcome != "uncertain" || !result.Uncertain {
		t.Fatal(result, err)
	}
	if _, err = c.Dispatch(context.Background(), a); err == nil || launches != 1 {
		t.Fatal("uncertain activation replayed")
	}
}

func TestExecutableRegistrationRemovalInvalidatesPreparation(t *testing.T) {
	c, _ := desktopControllerFixture(t)
	app := c.apps["editor"]
	registered := true
	app.kind = applicationExecutable
	app.validateExecutableRegistration = func() error {
		if !registered {
			return ErrControlScope
		}
		return nil
	}
	c.apps["editor"] = app
	launches := 0
	c.launch = func(context.Context, installedApplication) error { launches++; return nil }
	a, err := c.Prepare(Operation{Kind: "open_application", Application: "editor"})
	if err != nil {
		t.Fatal(err)
	}
	registered = false
	if _, err = c.Dispatch(context.Background(), a); !errors.Is(err, ErrControlScope) || launches != 0 {
		t.Fatal("removed registration dispatched", err)
	}
}

func TestPackagedCatalogDoesNotMergeNamesOrEntries(t *testing.T) {
	c, d := desktopControllerFixture(t)
	_, pkg := fixturePackage(c, d)
	a := c.apps["editor"]
	b := a
	b.ID = "second"
	b.Name = a.Name // duplicate display names cannot merge distinct app entries
	other := pkg
	other.AUMID, other.Entry = "Fixture_123!OtherEntry", "OtherEntry"
	b.packageIdentity = &other
	if sameInstalledApplication(a, b) {
		t.Fatal("shared family/executable merged")
	}
	b = a
	b.ID = "refreshed"
	if !sameInstalledApplication(a, b) {
		t.Fatal("unchanged registration not retained")
	}
	other = pkg
	other.ManifestRevision = strings.Repeat("b", 64)
	b.packageIdentity = &other
	if sameInstalledApplication(a, b) {
		t.Fatal("revised registration retained")
	}
	b = a
	b.kind = "future-kind"
	if checkInstalledApplication(b) == nil {
		t.Fatal("open-ended identity kind")
	}
}

func TestPackagedReadinessSupportsExistingProcessWithoutRelaunch(t *testing.T) {
	c, d := desktopControllerFixture(t)
	fixturePackage(c, d)
	launches := 0
	c.launch = func(context.Context, installedApplication) error { launches++; return nil }
	a, err := c.Prepare(Operation{Kind: "open_application", Application: "editor"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Dispatch(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		result, err := c.VerifyApplication(context.Background(), "editor")
		if err != nil || !result.Verified || result.Target == nil || result.Target.ProcessGeneration != "editor-process" {
			t.Fatal(result, err)
		}
	}
	if launches != 1 || d.focus != 0 {
		t.Fatal("readiness relaunched or focused")
	}
}
