package main

import (
	"context"
	"fmt"
	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/output"
	"net"
	"net/http"
	"os"
	"time"
)

var (
	// httpClient is a shared client with reasonable timeouts for most API calls
	httpClient = &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 5,
			IdleConnTimeout:     90 * time.Second,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
		},
	}

	// httpClientLong is for long-running requests like inference
	httpClientLong = &http.Client{
		Timeout: 5 * time.Minute,
		Transport: &http.Transport{
			MaxIdleConns:        5,
			MaxIdleConnsPerHost: 2,
			IdleConnTimeout:     120 * time.Second,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
		},
	}
)

func runRoot(ctx context.Context, rawArgs []string, env cli.Environment) int {
	args, options := cli.ParseInvocation(rawArgs)

	// Set global JSON mode
	output.JSONMode = options.JSON
	if options.JSON || options.NoColor {
		disableColors()
	}

	// Parse command
	if len(args) > 1 {
		command := args[1]
		canonical, _ := cli.CanonicalCommand(command)
		if cli.HasCommand(command) {
			commandEnv := env
			switch canonical {
			case "run", "benchmark", "compare", "metrics":
				// Preserve these commands' prior default HTTP transport and
				// timeout behavior while moving dispatch to internal/cli.
				commandEnv.HTTP = http.DefaultClient
			case "agent":
				if agentUsesLegacyService(args[2:]) {
					commandEnv = legacyAgentEnvironment(commandEnv)
				}
			}
			return cli.Execute(ctx, rawArgs[1:], commandEnv)
		}

		switch canonical {
		case "recommend":
			return handleAutoSelect(args[2:])
		case "appliance":
			return handleAppliance(args[2:])
		case "init":
			return handleInit(args[2:])
		case "test":
			return handleTest(args[2:])
		case "shell-completion":
			handleShellCompletion(args[2:])
			return 0
		case "peers":
			handlePeers(args[2:])
			return 0
		case "alias":
			handleAlias(args[2:])
			return 0
		case "favorite":
			handleFavorite(args[2:])
			return 0
		case "template":
			handleTemplate(args[2:])
			return 0
		case "workspace":
			return executeCommand(ctx, func(ctx context.Context) error { return runWorkspaceCommand(ctx, args[2:]) })
		case "completions":
			handleCompletions(args[2:])
			return 0
		case "lora":
			handleLoRA(args[2:])
			return 0
		case "hermes":
			if err := handleHermes(args[2:]); err != nil {
				printError(err.Error())
				return 1
			}
			return 0
		case "openclaw":
			if err := handleOpenClaw(args[2:]); err != nil {
				printError(err.Error())
				return 1
			}
			return 0
		case "serve":
			// Fall through to start server
		default:
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
			printHelp()
			return 1
		}
	}
	return startDefaultServer(ctx)
}

func agentUsesLegacyService(args []string) bool {
	if len(args) == 0 {
		return true
	}
	switch args[0] {
	case "profiles", "run", "artifact", "status", "approve", "deny", "cancel", "resume", "reconcile", "pause", "takeover", "steer", "reconnect", "export":
		return false
	default:
		return true
	}
}
