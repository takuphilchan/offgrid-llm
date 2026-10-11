package computer

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type readinessAdapter struct {
	*desktopFixtureAdapter
	observations int
	observe      func(context.Context) (NativeView, error)
	surfaces     int
	surface      func() ([]NativeTarget, error)
}

func (d *readinessAdapter) Observe(ctx context.Context) (NativeView, error) {
	d.observations++
	if d.observe != nil {
		return d.observe(ctx)
	}
	return d.desktopFixtureAdapter.Observe(ctx)
}
func (d *readinessAdapter) Targets() ([]NativeTarget, error) {
	d.surfaces++
	if d.surface != nil {
		return d.surface()
	}
	return d.desktopFixtureAdapter.Targets()
}

func TestApplicationReadinessNeverFabricatesZeroTarget(t *testing.T) {
	for _, state := range []string{"surface_unavailable", "observation_unavailable", "unsupported_identity"} {
		t.Run(state, func(t *testing.T) {
			c, fixture := desktopControllerFixture(t)
			d := &readinessAdapter{desktopFixtureAdapter: fixture}
			c.driver, c.provider = d, d
			switch state {
			case "surface_unavailable":
				fixture.targets = nil
			case "observation_unavailable":
				d.observe = func(context.Context) (NativeView, error) { return NativeView{}, errors.New("provider unavailable") }
			case "unsupported_identity":
				app := c.apps["editor"]
				app.kind = "unknown-identity"
				c.apps["editor"] = app
			}
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
			defer cancel()
			result, err := c.VerifyApplication(ctx, "editor")
			if err != nil || result.Verified || result.Readiness != state || result.Target != nil || result.View != nil {
				t.Fatal(result, err)
			}
			data, _ := json.Marshal(result)
			if strings.Contains(string(data), `"target"`) || strings.Contains(string(data), `"view"`) {
				t.Fatal(string(data))
			}
			if fixture.focus != 0 {
				t.Fatal("readiness focused application")
			}
		})
	}
}

func TestApplicationReadinessDelayedSurfaceAndCancellation(t *testing.T) {
	c, fixture := desktopControllerFixture(t)
	d := &readinessAdapter{desktopFixtureAdapter: fixture}
	c.driver, c.provider = d, d
	d.surface = func() ([]NativeTarget, error) {
		if d.surfaces < 2 {
			return nil, nil
		}
		return fixture.targets, nil
	}
	result, err := c.VerifyApplication(context.Background(), "editor")
	if err != nil || !result.Verified || result.Readiness != "ready" || result.Target == nil || result.View == nil || d.surfaces != 2 || fixture.focus != 0 {
		t.Fatal(result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.VerifyApplication(ctx, "editor"); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation lost", err)
	}
	fixture.identity.OSSession = "revoked"
	if _, err := c.VerifyApplication(context.Background(), "editor"); !errors.Is(err, ErrControlScope) {
		t.Fatal("revocation lost", err)
	}
}
