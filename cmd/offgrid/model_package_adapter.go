package main

import (
	"context"

	"github.com/google/uuid"
	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func modelPackageEnvironment() cli.Environment {
	env := localAdminEnvironment()
	env.Service.Client = newCommandClient
	env.ModelPackage.NewRequestID = uuid.NewString
	return env
}

func runModelPackageCommand(ctx context.Context, args []string) error {
	return cli.RunModelPackageCommand(ctx, args, modelPackageEnvironment())
}

// The compatibility wrapper keeps existing cancellation tests on the same
// polling implementation while cmd/offgrid sheds service execution logic.
func waitForModelOperation(ctx context.Context, client *serviceclient.Client, id string) error {
	return cli.WaitForModelOperation(ctx, client, id, modelPackageEnvironment())
}
