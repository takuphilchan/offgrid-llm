package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func connectNativeHost(state string) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	identityDirectory, err := execution.ProjectIdentityDirectory()
	if err != nil {
		return err
	}
	return execution.ServeNativeHostWithProjectIdentity(ctx, os.Stdin, os.Stdout, filepath.Dir(resolved), state, Version, identityDirectory)
}
