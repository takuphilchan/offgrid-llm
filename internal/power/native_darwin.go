package power

import (
	"context"
	"os/exec"
	"time"
)

func readNativePower() (PowerStatus, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	data, err := exec.CommandContext(ctx, "/usr/bin/pmset", "-g", "batt").Output()
	if err != nil {
		return PowerStatus{State: PowerUnknown, BatteryLevel: BatteryUnknown}, true
	}
	return parseDarwinPower(string(data)), true
}
