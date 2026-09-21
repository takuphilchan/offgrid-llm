//go:build darwin && cgo

package main

import (
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/internal/storage"
	"os"
	"path/filepath"
)

var localConsent *computer.NativeConsentProcess

func newNativeDriver() (computer.NativeAdapter, error) { return computer.NewNativeDarwin() }
func acquireNativeInputOwner() (func(), error) {
	root, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	owner, err := storage.AcquireOwnership(filepath.Join(root, "offgrid-computer", "input-owner"))
	if err != nil {
		return nil, err
	}
	return func() { owner.Close() }, nil
}
func startNativeStopSurface(revoke func()) (interface{ Close() }, error) {
	worker, err := os.Executable()
	if err != nil {
		return nil, err
	}
	ui, err := computer.StartNativeConsentProcess(filepath.Join(filepath.Dir(worker), "OffGrid Computer Controls.app", "Contents", "MacOS", "offgrid-computer-ui"), revoke)
	if err == nil {
		localConsent = ui
	}
	return ui, err
}
func nativeConfirm(text string) bool { return localConsent != nil && localConsent.Confirm(text) }
