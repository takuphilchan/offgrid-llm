package main

import (
	"context"
	"github.com/charmbracelet/x/term"
	"os"
	"path/filepath"
	"runtime"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func nativeExecutionDependencies() cli.ExecutionDependencies {
	packDirectory := func() (string, error) {
		executable, err := os.Executable()
		if err != nil {
			return "", err
		}
		return execution.FindPack(executable, Version, os.Getenv("OFFGRID_EXECUTOR_PACK_DIR"))
	}
	open := func(ctx context.Context, client *serviceclient.Client, folder string, permissions execution.Permissions, isolation execution.IsolationPolicy, approve func(execution.LocalApproval) bool) (cli.ExecutionLease, error) {
		if folder == "" {
			var err error
			folder, err = os.Getwd()
			if err != nil {
				return nil, err
			}
		}
		root, err := filepath.Abs(folder)
		if err != nil {
			return nil, err
		}
		pack, err := packDirectory()
		if err != nil {
			return nil, err
		}
		cache, err := os.UserCacheDir()
		if err != nil {
			return nil, err
		}
		return execution.StartLocalHost(ctx, client, execution.LocalHostOptions{Folder: root, PackDirectory: pack, StateDirectory: filepath.Join(cache, "offgrid", "execution-hosts"), Version: Version, Permissions: permissions, Isolation: isolation}, approve)
	}
	return cli.ExecutionDependencies{
		Trust: func() (*execution.FolderTrustStore, error) {
			root, err := os.UserConfigDir()
			if err != nil {
				return nil, err
			}
			parent := filepath.Join(root, "offgrid")
			if err = os.MkdirAll(parent, 0700); err != nil {
				return nil, err
			}
			// The trust leaf has its own account-private permissions; never alter
			// existing user configuration directories to enable remembered trust.
			return &execution.FolderTrustStore{Directory: filepath.Join(parent, "folder-trust")}, nil
		},
		CurrentFolder: os.Getwd,
		CaptureFolder: captureNativeLaunchFolder,
		OnDemand: func(ctx context.Context, client *serviceclient.Client) (bool, error) {
			identity, err := client.Identity(ctx)
			return err == nil && identity.Supports("execution-access-request-v1"), err
		},
		Interactive: func() bool { return term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd()) },
		Available: func(ctx context.Context, client *serviceclient.Client) (bool, error) {
			if _, err := packDirectory(); err != nil {
				return false, err
			}
			identity, err := client.Identity(ctx)
			return err == nil && identity.Supports("native-execution-preview-v1"), err
		},
		Open: func(ctx context.Context, client *serviceclient.Client, folder string, permissions execution.Permissions, approve func(execution.LocalApproval) bool) (cli.ExecutionLease, error) {
			return open(ctx, client, folder, permissions, execution.IsolationPolicy{}, approve)
		},
		OpenIsolated: func(ctx context.Context, client *serviceclient.Client, folder string, permissions execution.Permissions, policy execution.IsolationPolicy, approve func(execution.LocalApproval) bool) (cli.ExecutionLease, error) {
			if runtime.GOOS != "linux" || os.Getuid() < 1 || os.Getgid() < 1 {
				return nil, execution.ErrUnsupported
			}
			policy.UID, policy.GID = os.Getuid(), os.Getgid()
			return open(ctx, client, folder, permissions, policy, approve)
		},
	}
}
