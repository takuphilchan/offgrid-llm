//go:build windows && amd64

package main

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

// Pure authority fixture: no real native driver, dialogs, input or application launch.
type permissionDriver struct {
	computer.NativeAdapter
	identity computer.TargetIdentity
}

func (d *permissionDriver) DesktopIdentity() (computer.TargetIdentity, error)      { return d.identity, nil }
func (d *permissionDriver) TargetExecutable(computer.NativeTarget) (string, error) { return "", nil }

func TestDesktopSessionPermissionReleasesTaskNotConsent(t *testing.T) {
	driver := &permissionDriver{identity: computer.TargetIdentity{ID: "desktop", OSSession: "fixture-os", ProcessGeneration: "desktop", Surface: "desktop", Driver: "windows-uia"}}
	controller, err := computer.NewDesktopController(driver)
	if err != nil {
		t.Fatal(err)
	}
	journal, err := computer.OpenNativeJournal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer journal.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := &desktopWorker{driver: driver, controller: controller, journal: journal, ctx: ctx, active: &atomic.Pointer[computer.NativeSupervisor]{}, permission: &computer.DesktopPermission{Service: "http://127.0.0.1:11611", Workspace: "workspace", Actor: "owner"}}
	defer d.stop()
	authority := &computer.DesktopAuthority{Workspace: "workspace", Binding: computer.ControlBinding{Actor: "owner", Task: "first", Companion: "companion", Session: "session", Target: driver.identity}}
	grant := request{Kind: "grant_desktop", Desktop: authority, Service: d.permission.Service, ApprovalMode: computer.ApprovalFullTask, Prompt: "Owned test"}
	if _, err := d.call(grant); err != nil {
		t.Fatal(err)
	}
	previous := d.supervisor
	d.prepared["old"] = desktopPreparation{}
	if _, err := d.call(request{Kind: "release_desktop"}); err != nil {
		t.Fatal(err)
	}
	if previous.Check() == nil || d.supervisor != nil || d.active.Load() != nil || len(d.prepared) != 0 || d.permission == nil {
		t.Fatal("task authority survived release or root permission was lost")
	}
	if _, err := d.call(request{Kind: "applications"}); err == nil {
		t.Fatal("idle permission dispatched without a task grant")
	}
	authority.Binding.Task = "next"
	for _, field := range []string{"actor", "workspace", "policy", "service"} {
		bad := grant
		a := *authority
		bad.Desktop = &a
		switch field {
		case "actor":
			a.Binding.Actor = "other"
		case "workspace":
			a.Workspace = "other"
		case "policy":
			bad.ApprovalMode = computer.ApprovalAskEveryTime
		case "service":
			bad.Service = "http://127.0.0.1:9"
		}
		if _, err := d.call(bad); err == nil {
			t.Fatal("broadened", field)
		}
	}
	if _, err := d.call(grant); err != nil {
		t.Fatal(err)
	}
	if d.supervisor == previous || d.authority.Binding.Task != "next" {
		t.Fatal("did not create separate authority")
	}
	driver.identity.OSSession = "different"
	if _, err := d.call(request{Kind: "release_desktop"}); err == nil {
		t.Fatal("adopted a different signed-in desktop")
	}
	cancel()
	if _, err := d.call(grant); err == nil {
		t.Fatal("revoked worker reused consent")
	}
}
