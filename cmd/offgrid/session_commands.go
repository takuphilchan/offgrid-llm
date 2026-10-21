package main

import (
	"context"
	"io"
	"os"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/output"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

func sessionEnvironment() cli.Environment {
	return cli.Environment{
		Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
		Options: cli.InvocationOptions{JSON: output.JSONMode},
		Service: cli.ServiceDependencies{
			Client: newCommandClient, RenderStream: renderAgentStreamTo,
			CreateSessionExportFile: func(path string) (cli.SessionExportFile, error) {
				return os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			},
			RemoveSessionExportFile: os.Remove,
			Chat: func(ctx context.Context, client *serviceclient.Client, args []string, stdin io.Reader, stdout io.Writer, jsonMode bool) error {
				return cli.RunConversationChat(ctx, client, args, stdin, stdout, jsonMode, renderAgentStreamTo)
			},
		},
	}
}

func runSessionCommand(ctx context.Context, args []string) error {
	return cli.RunSessionCommand(ctx, args, sessionEnvironment())
}

func runSessionExportCommand(ctx context.Context, args []string) error {
	return cli.RunSessionExportCommand(ctx, args, sessionEnvironment())
}

func loadServiceSession(ctx context.Context, client *serviceclient.Client, name string) (*sessions.Session, error) {
	return cli.LoadServiceSession(ctx, client, name)
}

func sessionCommand(ctx context.Context, client *serviceclient.Client, args []string, stdout io.Writer, jsonMode bool) error {
	return cli.RunSessionService(ctx, client, args, stdout, jsonMode, renderAgentStreamTo)
}

func exportServiceSession(ctx context.Context, client *serviceclient.Client, name, format, filename string, stdout io.Writer, jsonMode bool) error {
	return cli.ExportServiceSession(ctx, client, name, format, filename, stdout, jsonMode)
}
