//go:build !windows && !linux && !darwin

package main

import "fmt"

func newAgentInputPoller() (func() ([]byte, error), error) {
	return nil, fmt.Errorf("live input is unavailable on this platform")
}
