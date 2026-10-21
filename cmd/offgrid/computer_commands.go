package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
)

var computerWorkspaceProcess cli.CommandRunner = reviewedProcessRunner{}

func runComputerCommand(ctx context.Context, args []string) error {
	return cli.RunComputer(ctx, args, cli.ComputerDependencies{
		Setup: openComputerWorkspace, Control: runAgentControl,
		Submit: runTaskSubmit, Client: newCommandClient, Stdout: os.Stdout,
	})
}

func openComputerWorkspace(ctx context.Context, task string) error {
	link := "offgrid://computer"
	if task != "" {
		if len(task) != 36 || !strings.HasPrefix(task, "run-") {
			return usage("Invalid task ID")
		}
		for _, value := range task[4:] {
			if !strings.ContainsRune("0123456789abcdef", value) {
				return usage("Invalid task ID")
			}
		}
		link += "?task=" + task
	}
	if err := computerWorkspaceProcess.Run(ctx, cli.CommandRequest{Capability: "open-computer-workspace", Args: []string{link}}); err != nil {
		return fmt.Errorf("open the matching OffGrid Desktop application to grant local access: %w", err)
	}
	return nil
}
