package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/output"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func usage(format string, args ...any) error { return cli.Usage(format, args...) }

// Command functions return errors; only the process boundary chooses an exit
// code. Tests exercise this boundary in a subprocess as well as directly.
func reportCommandError(err error, jsonMode bool, stdout, stderr io.Writer) int {
	return cli.ReportError(err, jsonMode, stdout, stderr)
}

func executeCommand(ctx context.Context, command func(context.Context) error) int {
	err := command(ctx)
	return reportCommandError(err, output.JSONMode, os.Stdout, os.Stderr)
}

func commandServerURL() string {
	if value := strings.TrimSpace(os.Getenv("OFFGRID_SERVER_URL")); value != "" {
		return value
	}
	return fmt.Sprintf("http://127.0.0.1:%d", config.LoadConfig().ServerPort)
}

func newCommandClient() (*serviceclient.Client, error) {
	return serviceclient.New(commandServerURL(), os.Getenv("OFFGRID_API_KEY"), httpClientLong)
}
