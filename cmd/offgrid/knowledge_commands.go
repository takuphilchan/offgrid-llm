package main

import (
	"context"
	"io"
	"os"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/output"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func runKnowledgeCommand(ctx context.Context, args []string) error {
	return cli.RunKnowledgeCommand(ctx, args, cli.Environment{
		Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
		Options: cli.InvocationOptions{JSON: output.JSONMode},
		Service: cli.ServiceDependencies{Client: newCommandClient},
	})
}

func knowledgeCommand(ctx context.Context, client *serviceclient.Client, args []string, input io.Reader, progress io.Writer) (any, error) {
	return cli.RunKnowledgeService(ctx, client, args, input, progress, output.JSONMode)
}

func validateKnowledgeResponse(action string, result map[string]any) error {
	return cli.ValidateKnowledgeResponse(action, result)
}
