package main

import (
	"context"
	"os"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/output"
)

func downloadJobArtifact(ctx context.Context, args []string) error {
	return cli.RunArtifactDownload(ctx, newCommandClient, args, os.Stdout, output.JSONMode, func(path string) (cli.ArtifactFile, error) {
		return os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	})
}
