package power

import (
	"regexp"
	"strconv"
	"strings"
)

func windowsPowerStatus(ac, flags, percent, saver byte) PowerStatus {
	status := PowerStatus{State: PowerUnknown, BatteryLevel: BatteryUnknown, PowerSaveActive: saver == 1}
	switch ac {
	case 1:
		status.State = PowerAC
	case 0:
		status.State = PowerBattery
	}
	if flags != 255 && flags&128 == 0 {
		status.IsCharging = flags&8 != 0
		if percent <= 100 {
			status.BatteryPercent = int(percent)
			status.BatteryLevel = percentToLevel(int(percent))
		}
	}
	return status
}

var darwinBatteryPercent = regexp.MustCompile(`\b([0-9]{1,3})%;`)

func parseDarwinPower(value string) PowerStatus {
	status := PowerStatus{State: PowerUnknown, BatteryLevel: BatteryUnknown}
	switch {
	case strings.Contains(value, "Now drawing from 'AC Power'"):
		status.State = PowerAC
	case strings.Contains(value, "Now drawing from 'Battery Power'"):
		status.State = PowerBattery
	}
	if match := darwinBatteryPercent.FindStringSubmatch(value); len(match) == 2 {
		percent, _ := strconv.Atoi(match[1])
		if percent <= 100 {
			status.BatteryPercent = percent
			status.BatteryLevel = percentToLevel(percent)
		}
	}
	status.IsCharging = strings.Contains(value, "; charging;")
	return status
}
