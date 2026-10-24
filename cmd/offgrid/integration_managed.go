package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/integrations"
)

func integrationDefaults() (integrations.HostOptions, error) {
	cfg, err := config.LoadWithPriority(os.Getenv("OFFGRID_CONFIG"))
	if err != nil {
		return integrations.HostOptions{}, err
	}
	server := strings.TrimSpace(os.Getenv("OFFGRID_SERVER_URL"))
	if server == "" {
		server = fmt.Sprintf("http://127.0.0.1:%d", cfg.ServerPort)
	}
	return integrations.HostOptions{ServerURL: strings.TrimRight(server, "/"), APIKey: strings.TrimSpace(os.Getenv("OFFGRID_API_KEY"))}, nil
}

func managedIntegration(ctx context.Context, action, id string, args []string) error {
	return runManagedIntegration(ctx, action, id, args, false)
}

func runManagedIntegration(ctx context.Context, action, id string, args []string, legacy bool) error {
	defaults, err := integrationDefaults()
	if err != nil {
		return err
	}
	o, rest, err := integrations.ParseHostOptions(args, defaults, action == "run")
	if err != nil {
		return err
	}
	if action != "configure" && (o.Force || o.Yes || o.WithBrowser || o.RunDoctor || o.DryRun) {
		return fmt.Errorf("installation options require configure/install")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	client, err := integrations.NewServiceClient(o.ServerURL, o.APIKey, httpClient)
	if err != nil {
		return err
	}
	stateDir := os.Getenv("OFFGRID_INTEGRATIONS_DIR")
	if stateDir == "" {
		stateDir = filepath.Join(home, ".offgrid", "integrations", "state")
	}
	manager := integrations.Manager{
		Adapters: map[string]integrations.HostAdapter{"hermes": newHermesHost(home), "openclaw": newOpenClawHost(home)},
		Service:  client, Store: &integrations.ConnectionStore{Directory: stateDir, Scope: strings.Join([]string{integrations.HermesHome(home), os.Getenv("OPENCLAW_STATE_DIR"), os.Getenv("OPENCLAW_CONFIG_PATH")}, "\x00")}, Progress: hermesProgress,
		Confirm: func(_ context.Context, id, kind string) (bool, error) {
			if o.Yes {
				return true, nil
			}
			if kind == "runtime" {
				if id == "hermes" {
					return confirmHermesInstall(false)
				}
				return confirmOpenClawInstall(false)
			}
			return confirmOpenClawAction(false, fmt.Sprintf("The first-party OffGrid provider executes local code inside %s. Register and configure it?", id))
		},
	}
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	var report integrations.HostReport
	switch action {
	case "status":
		report, err = manager.Status(ctx, id, o)
	case "configure":
		report, err = manager.Configure(ctx, id, o)
	case "test":
		report, err = manager.Test(ctx, id, o)
	case "run":
		report, err = manager.Run(ctx, id, o, rest)
	default:
		return fmt.Errorf("unknown managed integration operation %q", action)
	}
	var payload any = report
	if legacy && (action == "configure" || action == "status") {
		payload = legacyIntegrationProjection(action, id, report, o, err)
	}
	if !outputJSON(payload) {
		fmt.Printf("\n  %s: %s\n", id, report.Phase)
		if report.ModelID != "" {
			fmt.Printf("  Model: %s (%s)\n", report.ModelID, report.SelectionSource)
		}
		if report.Verification != nil {
			fmt.Printf("  File contract: %s (%s)\n", report.Verification.State, report.Verification.Code)
		}
		if report.Runtime.Version != "" {
			fmt.Printf("  Runtime: %s\n", report.Runtime.Version)
		}
		for _, check := range report.Checks {
			if check.State == "blocked" {
				printWarning(check.Message)
			}
		}
		if report.Ready {
			for _, warning := range report.Service.Warnings {
				printWarning(warning)
			}
		}
		if report.Ready && action != "run" {
			fmt.Printf("  Preflight ready; model reply: %s; tool workflow: %s\n", report.Smoke.State, report.Workflow.State)
		}
		if report.RecoveryPath != "" {
			fmt.Printf("  Configuration backup: %s\n", report.RecoveryPath)
		}
		if o.DryRun {
			fmt.Printf("  Would configure provider %s at %s for model %s; no files changed\n", id, o.Endpoint(), report.ModelID)
		}
		fmt.Println()
	}
	if err == nil && action == "status" && !report.Ready {
		return fmt.Errorf("%s preflight is blocked; address the checks above", id)
	}
	return err
}

func managedAlias(action, id string, args []string) error {
	return runManagedIntegration(context.Background(), action, id, args, true)
}

// OS adapter shared helpers keep cancellation and bounded inspection output
// consistent. No shell interpolation is used for external command arguments.
type externalCommandFactory func(string, []string) (*exec.Cmd, error)

func externalCommand(ctx context.Context, factory externalCommandFactory, path string, args, environment []string, stdin io.Reader, stdout, stderr io.Writer) error {
	command, err := factory(path, args)
	if err != nil {
		return err
	}
	command = bindIntegrationContext(ctx, command)
	command.Env = environment
	command.Stdin, command.Stdout, command.Stderr = stdin, stdout, stderr
	err = runIntegrationProcess(command)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

func bindIntegrationContext(ctx context.Context, original *exec.Cmd) *exec.Cmd {
	cmd := exec.CommandContext(ctx, original.Path, original.Args[1:]...)
	cmd.Env, cmd.Dir = original.Env, original.Dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = original.Stdin, original.Stdout, original.Stderr
	configureIntegrationCancellation(cmd)
	cmd.WaitDelay = 3 * time.Second
	return cmd
}

func externalOutput(ctx context.Context, factory externalCommandFactory, path string, args []string) (string, error) {
	var out hermesOutputTail
	err := externalCommand(ctx, factory, path, args, nil, nil, &out, io.Discard)
	return strings.TrimSpace(out.String()), err
}

func hermesCommand(path string, args []string) (*exec.Cmd, error) {
	if err := validateIntegrationArgs("Hermes", append([]string{path}, args...)); err != nil {
		return nil, err
	}
	return hermesExecCommand(path, args...), nil
}

func integrationEnvironment(o integrations.HostOptions) []string {
	key := o.APIKey
	if key == "" {
		key = "offgrid-local"
	}
	values := map[string]string{"OFFGRID_BASE_URL": o.Endpoint(), "OFFGRID_API_KEY": key}
	// Stale caller settings cannot opt an ordinary invocation into verification.
	var environment []string
	for _, value := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(value), "OFFGRID_VERIFY_") {
			environment = append(environment, value)
		}
	}
	if c := o.Verification; c != nil {
		values["OFFGRID_VERIFY_URL"], values["OFFGRID_VERIFY_TOKEN"] = c.URL, c.Token
		values["OFFGRID_VERIFY_RUN"], values["OFFGRID_VERIFY_ADAPTER"] = c.RunID, c.Adapter
	}
	return mergedEnvironment(environment, values)
}
