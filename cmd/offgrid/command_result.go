package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/output"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

type usageError struct{ message string }

func (e *usageError) Error() string          { return e.message }
func usage(format string, args ...any) error { return &usageError{fmt.Sprintf(format, args...)} }

// Command functions return errors; only the process boundary chooses an exit
// code. Tests exercise this boundary in a subprocess as well as directly.
func reportCommandError(err error, jsonMode bool, stdout, stderr io.Writer) int {
	if err == nil {
		return 0
	}
	code := 1
	detail := &serviceclient.Error{Code: "operation_failed", Message: terminalSafe(err.Error())}
	var invalid *usageError
	var serviceErr *serviceclient.Error
	switch {
	case errors.Is(err, context.Canceled):
		code = 130
		detail = &serviceclient.Error{Code: "cancelled", Message: "Operation cancelled. Inspect saved state before retrying."}
	case errors.As(err, &invalid):
		code = 2
		detail.Code = "invalid_usage"
	case errors.As(err, &serviceErr):
		copy := *serviceErr
		copy.Message = terminalSafe(err.Error()) // retain safe partial-operation context
		detail = &copy
		if detail.Code == "invalid_server" {
			code = 2
		}
	}
	if jsonMode {
		_ = json.NewEncoder(stdout).Encode(map[string]any{"error": detail})
	} else {
		fmt.Fprintf(stderr, "ERR %s\n", terminalSafe(detail.Message))
	}
	return code
}

func executeCommand(command func(context.Context) error) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	err := command(ctx)
	stop()
	if code := reportCommandError(err, output.JSONMode, os.Stdout, os.Stderr); code != 0 {
		os.Exit(code)
	}
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
