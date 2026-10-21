package main

import (
	"context"
	"os"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
)

func runModelList(ctx context.Context, args []string) error {
	return cli.RunList(ctx, args, legacyPresenter(), commandServerURL(), os.Getenv("OFFGRID_API_KEY"), httpClientLong)
}

func runModelDownload(ctx context.Context, args []string) error {
	env := localAdminEnvironment()
	env.Service.Client = newCommandClient
	env.ServerURL = commandServerURL()
	return cli.RunModelDownloadCommand(ctx, args, env)
}
