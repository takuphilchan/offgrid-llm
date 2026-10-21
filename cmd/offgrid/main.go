package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/output"
)

// Version is set via ldflags during build.
var Version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	_, options := cli.ParseInvocation(os.Args)
	output.JSONMode = options.JSON
	if options.JSON || options.NoColor {
		disableColors()
	}
	code := runRoot(ctx, os.Args, osCLIEnvironment())
	stop()
	os.Exit(code)
}
