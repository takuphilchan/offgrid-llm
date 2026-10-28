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
	return execution.ServeNativeHost(ctx, os.Stdin, os.Stdout, filepath.Dir(resolved), state, Version)
}
