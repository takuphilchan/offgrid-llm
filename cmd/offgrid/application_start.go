package main

import (
	"context"

	"github.com/takuphilchan/offgrid-llm/internal/app"
	"github.com/takuphilchan/offgrid-llm/internal/config"
)

var loadServeConfig = config.LoadWithPriority
var startApplication = func(ctx context.Context, cfg *config.Config) error {
	runtime, err := app.New(cfg)
	if err != nil {
		return err
	}
	return runtime.Run(ctx)
}
