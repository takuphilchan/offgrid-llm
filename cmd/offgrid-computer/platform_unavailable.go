//go:build (!windows || !amd64) && (!darwin || !cgo) && (!linux || !cgo || !offgrid_native)

package main

import (
	"errors"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

// No successful no-op adapter: unsupported hosts must never acquire consent or
// advertise availability just because the portable supervisor compiles.
var errNativeUnavailable = errors.New("computer_driver_unavailable")

func newNativeDriver() (computer.NativeAdapter, error)            { return nil, errNativeUnavailable }
func acquireNativeInputOwner() (func(), error)                    { return nil, errNativeUnavailable }
func nativeConfirm(string) bool                                   { return false }
func startNativeStopSurface(func()) (interface{ Close() }, error) { return nil, errNativeUnavailable }
