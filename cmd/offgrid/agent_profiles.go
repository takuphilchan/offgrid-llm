package main

import (
	"context"
	"io"
	"os"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/output"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func validProfileID(id string) bool { return cli.ValidProfileID(id) }

func readProfileSettings(name string, stdin io.Reader) (agents.ProfileSettings, error) {
	return cli.ReadProfileSettings(name, stdin)
}

func profileCommand(ctx context.Context, client *serviceclient.Client, args []string, stdin io.Reader, stdout io.Writer, jsonMode bool) error {
	return cli.RunProfileService(ctx, client, args, stdin, stdout, jsonMode)
}

func runAgentProfiles(ctx context.Context, args []string) error {
	return cli.RunAgentCommand(ctx, append([]string{"profiles"}, args...), cli.Environment{
		Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
		Options: cli.InvocationOptions{JSON: output.JSONMode},
		Service: cli.ServiceDependencies{
			Client:          newCommandClient,
			OpenProfileFile: func(path string) (io.ReadCloser, error) { return os.Open(path) },
		},
	})
}
