package main

import (
	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func parseModelSearchArgs(args []string) (models.SearchFilter, int, bool, error) {
	return cli.ParseModelSearchArgs(args)
}
