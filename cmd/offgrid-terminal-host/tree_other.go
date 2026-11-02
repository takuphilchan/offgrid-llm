//go:build !windows

package main

import (
	"errors"
	"os/exec"
)

type ownedTree struct{}

func (*ownedTree) release()    {}
func (*ownedTree) stop() error { return errors.New("terminal platform not qualified") }
func startTree(*exec.Cmd) (*ownedTree, error) {
	return nil, errors.New("terminal platform not qualified")
}
