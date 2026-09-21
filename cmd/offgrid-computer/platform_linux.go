//go:build linux && cgo && offgrid_native

package main

import (
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/internal/storage"
	"os"
	"path/filepath"
	"syscall"
)

var localConsent *computer.NativeConsentProcess

func newNativeDriver() (computer.NativeAdapter, error) { return computer.NewNativeLinux() }
func acquireNativeInputOwner() (func(), error) {
	root := os.Getenv("XDG_RUNTIME_DIR")
	if root == "" {
		return nil, computer.ErrControlScope
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, computer.ErrControlScope
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Getuid() {
		return nil, computer.ErrControlScope
	}
	owner, err := storage.AcquireOwnership(filepath.Join(root, "offgrid-computer-input"))
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
	ui, err := computer.StartNativeConsentProcess(filepath.Join(filepath.Dir(worker), "offgrid-computer-ui"), revoke)
	if err == nil {
		localConsent = ui
	}
	return ui, err
}
func nativeConfirm(text string) bool { return localConsent != nil && localConsent.Confirm(text) }
