//go:build windows && amd64

package main

import "github.com/takuphilchan/offgrid-llm/internal/computer"

func newNativeDriver() (computer.NativeAdapter, error) { return computer.NewNativeWindows() }
func acquireNativeInputOwner() (func(), error)         { return computer.AcquireNativeInputOwner() }
func nativeConfirm(message string) bool                { return computer.NativeConfirm(message) }
func startNativeStopSurface(revoke func()) (interface{ Close() }, error) {
	return computer.StartNativeStopSurface(revoke)
}
