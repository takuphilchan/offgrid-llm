package main

import (
	"context"
	"crypto/rand"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/audit"
	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/output"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

// localAdminEnvironment retains the local-store authority of these legacy
// commands. It does not turn them into service API calls.
func localAdminEnvironment() cli.Environment {
	return cli.Environment{
		Stdout: os.Stdout, Stderr: os.Stderr, Getenv: os.Getenv,
		HTTP:    http.DefaultClient,
		Options: cli.InvocationOptions{JSON: output.JSONMode},
		Style:   legacyPresenter().Style,
		LocalAdmin: cli.LocalAdminDependencies{
			LoadConfig:       config.LoadConfig,
			LoadWithPriority: config.LoadWithPriority,
			LoadFromFile:     config.LoadFromFile,
			SaveConfig:       func(cfg *config.Config, path string) error { return cfg.SaveToFile(path) },
			HomeDir:          os.UserHomeDir,
			MkdirAll:         func(path string, mode uint32) error { return os.MkdirAll(path, os.FileMode(mode)) },
			NewUserStore:     users.NewUserStore,
			NewQuotaManager:  users.NewQuotaManager,
			NewAuditLogger:   audit.NewAuditLogger,
			GlobAuditFiles:   filepath.Glob,
			Entropy:          rand.Reader,
		},
	}
}

func handleConfig(ctx context.Context, args []string) int {
	return executeCommand(ctx, func(ctx context.Context) error {
		return cli.RunConfigCommand(ctx, args, localAdminEnvironment())
	})
}

func handleMetrics(ctx context.Context, args []string) int {
	return executeCommand(ctx, func(ctx context.Context) error {
		return cli.RunMetricsCommand(ctx, args, localAdminEnvironment())
	})
}

func handleUsers(ctx context.Context, args []string) int {
	return executeCommand(ctx, func(ctx context.Context) error {
		return cli.RunUsersCommand(ctx, args, localAdminEnvironment())
	})
}

func handleAudit(ctx context.Context, args []string) int {
	return executeCommand(ctx, func(ctx context.Context) error {
		return cli.RunAuditCommand(ctx, args, localAdminEnvironment())
	})
}

func modelVerifyEnvironment() cli.Environment {
	env := localAdminEnvironment()
	env.ModelVerify = cli.ModelVerifyDependencies{
		NewRegistry:  models.NewRegistry,
		NewValidator: models.NewValidator,
	}
	return env
}

func modelQuantizeEnvironment() cli.Environment {
	env := modelVerifyEnvironment()
	env.Quantize = cli.QuantizeDependencies{
		Stat:     os.Stat,
		LookPath: exec.LookPath,
		Now:      time.Now,
		Run: func(ctx context.Context, program, source, output, quantization string, stdout, stderr io.Writer) error {
			if program != "llama-quantize" {
				return cli.Usage("unsupported quantization program")
			}
			command := exec.CommandContext(ctx, program, source, output, quantization)
			command.Env = os.Environ()
			libPath := "/usr/local/lib"
			if existing := os.Getenv("LD_LIBRARY_PATH"); existing != "" {
				libPath += ":" + existing
			}
			command.Env = append(command.Env, "LD_LIBRARY_PATH="+libPath)
			command.Stdout, command.Stderr = stdout, stderr
			return command.Run()
		},
	}
	return env
}

func modelTransferEnvironment() cli.Environment {
	env := modelVerifyEnvironment()
	env.Stdin = os.Stdin
	env.ModelTransfer.Remove = os.Remove
	env.ModelTransfer.MkdirAll = os.MkdirAll
	env.ModelTransfer.Stat = os.Stat
	env.ModelTransfer.Open = func(path string) (io.ReadCloser, error) { return os.Open(path) }
	env.ModelTransfer.Create = func(path string) (io.WriteCloser, error) { return os.Create(path) }
	env.ModelTransfer.NewUSBImporter = func(dir string, registry *models.Registry) cli.USBImporter {
		return models.NewUSBImporter(dir, registry)
	}
	env.ModelTransfer.RenderProgress = printProgressBar
	env.ModelTransfer.ReloadServer = reloadLlamaServer
	env.ModelTransfer.ReloadModel = reloadLlamaServerWithModel
	return env
}
