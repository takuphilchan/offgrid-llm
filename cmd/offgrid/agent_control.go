package main

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/output"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

type agentRunSnapshot = cli.AgentRunSnapshot

func terminalSafe(value string) string { return cli.TerminalSafe(value) }

func agentRequest(client *http.Client, method, endpoint string, body io.Reader) (*http.Response, error) {
	return agentRequestContext(context.Background(), client, method, endpoint, body)
}

func agentRequestContext(ctx context.Context, transport *http.Client, method, endpoint string, body io.Reader) (*http.Response, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.User != nil || u.Fragment != "" {
		return nil, usage("Invalid agent service URL")
	}
	client, err := serviceclient.New(u.Scheme+"://"+u.Host, os.Getenv("OFFGRID_API_KEY"), transport)
	if err != nil {
		return nil, err
	}
	return client.Do(ctx, method, u.RequestURI(), "application/json", body)
}

func agentControl(base string, args []string) (*agentRunSnapshot, error) {
	return agentControlContext(context.Background(), base, args)
}

func agentControlContext(ctx context.Context, base string, args []string) (*agentRunSnapshot, error) {
	client, err := serviceclient.New(base, os.Getenv("OFFGRID_API_KEY"), httpClient)
	if err != nil {
		return nil, err
	}
	return cli.AgentControlService(ctx, client, args)
}

func validAgentStatus(status string) bool { return cli.ValidAgentStatus(status) }

func isAgentControl(action string) bool {
	switch action {
	case "status", "approve", "deny", "cancel", "resume", "reconcile", "pause", "takeover", "steer", "reconnect", "export", "artifact":
		return true
	}
	return false
}

func runAgentControl(ctx context.Context, args []string) error {
	return cli.RunAgentCommand(ctx, args, cli.Environment{
		Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
		Options: cli.InvocationOptions{JSON: output.JSONMode},
		Service: cli.ServiceDependencies{Client: newCommandClient, ControlClient: func() (*serviceclient.Client, error) {
			return serviceclient.New(commandServerURL(), os.Getenv("OFFGRID_API_KEY"), httpClient)
		}, CreateArtifactFile: func(path string) (cli.ArtifactFile, error) {
			return os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		}},
	})
}

func handleAgentControl(ctx context.Context, args []string) int {
	return executeCommand(ctx, func(ctx context.Context) error { return runAgentControl(ctx, args) })
}

func renderAgentSnapshot(w io.Writer, snapshot agentRunSnapshot) {
	cli.RenderAgentSnapshot(w, snapshot)
}
