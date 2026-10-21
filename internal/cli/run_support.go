package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

func legacyGet(ctx context.Context, client *http.Client, endpoint string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func legacyPost(ctx context.Context, client *http.Client, endpoint, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return client.Do(req)
}

func syncInteractiveOutput(env Environment) {
	if env.Interactive.SyncOutput != nil {
		_ = env.Interactive.SyncOutput()
	}
}

func printRunHeader(env Environment, modelName string, ragEnabled bool, sessionName string) {
	s := env.Style
	fmt.Fprintln(env.Stdout)
	fmt.Fprintf(env.Stdout, "  %s%sOffGrid Chat%s\n", s.Primary, s.Bold, s.Reset)
	fmt.Fprintln(env.Stdout)
	fmt.Fprintf(env.Stdout, "  %sModel:%s   %s\n", s.Muted, s.Reset, modelName)
	if sessionName != "" {
		fmt.Fprintf(env.Stdout, "  %sSession:%s %s\n", s.Muted, s.Reset, sessionName)
	}
	if ragEnabled {
		fmt.Fprintf(env.Stdout, "  %sRAG:%s     Enabled\n", s.Muted, s.Reset)
	}
}

func printRunCommands(env Environment) {
	s := env.Style
	fmt.Fprintf(env.Stdout, "  %sCommands:%s /help /status /clear /rag /exit\n", s.Muted, s.Reset)
}
