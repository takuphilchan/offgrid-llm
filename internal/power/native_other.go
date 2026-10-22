//go:build !windows && !darwin

package power

func readNativePower() (PowerStatus, bool) { return PowerStatus{}, false }
