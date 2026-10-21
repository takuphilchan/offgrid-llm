package main

import (
	"context"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
)

func handleDownload(ctx context.Context, args []string) int {
	return executeCommand(ctx, func(ctx context.Context) error { return runModelDownload(ctx, args) })
}

func handleImport(ctx context.Context, args []string) int {
	return executeCommand(ctx, func(ctx context.Context) error {
		return cli.RunImportModelCommand(ctx, args, modelTransferEnvironment())
	})
}

func handleRemove(ctx context.Context, args []string) int {
	return executeCommand(ctx, func(ctx context.Context) error {
		return cli.RunRemoveModelCommand(ctx, args, modelTransferEnvironment())
	})
}

func handleExport(ctx context.Context, args []string) int {
	return executeCommand(ctx, func(ctx context.Context) error {
		return cli.RunExportModelCommand(ctx, args, modelTransferEnvironment())
	})
}
