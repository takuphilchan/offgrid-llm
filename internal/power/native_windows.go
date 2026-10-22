package power

import (
	"golang.org/x/sys/windows"
	"unsafe"
)

func readNativePower() (PowerStatus, bool) {
	var raw struct {
		AC, Flags, Percent, Saver byte
		Remaining, Full           uint32
	}
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetSystemPowerStatus")
	ok, _, _ := proc.Call(uintptr(unsafe.Pointer(&raw)))
	if ok == 0 {
		return PowerStatus{State: PowerUnknown, BatteryLevel: BatteryUnknown}, true
	}
	return windowsPowerStatus(raw.AC, raw.Flags, raw.Percent, raw.Saver), true
}
