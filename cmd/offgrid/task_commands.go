package main

import (
	"context"
	"os"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/output"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

type taskSubmission = cli.TaskSubmission

func parseTaskSubmission(args []string) (taskSubmission, bool, error) {
	return cli.ParseTaskSubmission(args)
}

func submitTask(ctx context.Context, client *serviceclient.Client, req taskSubmission) (*agentRunSnapshot, error) {
	return cli.SubmitTask(ctx, client, req)
}

func runTaskSubmit(ctx context.Context, args []string) error {
	return cli.RunTaskSubmitCommand(ctx, args, cli.Environment{
		Stdout: os.Stdout, Stderr: os.Stderr,
		Options: cli.InvocationOptions{JSON: output.JSONMode},
		Service: cli.ServiceDependencies{Client: newCommandClient, RenderStream: renderAgentStreamTo},
	})
}
