package main

import (
	"os"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
)

func getVersion() string {
	return cli.ResolveVersion(Version, os.ReadFile)
}
