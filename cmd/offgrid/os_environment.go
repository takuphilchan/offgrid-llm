package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/audio"
	"github.com/takuphilchan/offgrid-llm/internal/batch"
	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/integrations"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

// osCLIEnvironment is built once for the process entry. Capability-specific
// adapters are attached as command families join the shared executor.
func osCLIEnvironment() cli.Environment {
	env := localAdminEnvironment()
	env.Stdin = os.Stdin
	env.Stdout = os.Stdout
	env.Stderr = os.Stderr
	env.Getenv = os.Getenv
	env.Setenv = os.Setenv
	env.HTTP = httpClient
	env.LongHTTP = httpClientLong
	env.Exec = reviewedProcessRunner{}
	env.Version = Version
	env.ReadFile = os.ReadFile
	env.Style = legacyPresenter().Style
	env.Device = func() cli.DeviceInfo {
		resources, err := resource.DetectResources()
		if err != nil || !resources.GPUAvailable {
			return cli.DeviceInfo{}
		}
		return cli.DeviceInfo{GPUName: resources.GPUName, GPUMemoryMB: resources.GPUMemory}
	}
	env.ServerURL = commandServerURL()
	env.Doctor = func(ctx context.Context) (cli.DoctorSnapshot, error) {
		return observeDoctor(ctx, config.LoadConfig(), resource.DetectResources, &http.Client{Timeout: 5 * time.Second}, filepath.Join(os.Getenv("HOME"), ".offgrid-llm", "config.json"))
	}
	env.Info = func(ctx context.Context) (cli.InfoSnapshot, error) {
		return observeInfo(ctx, config.LoadConfig(), getVersion())
	}
	env.Service = sessionEnvironment().Service
	env.Service.ControlClient = func() (*serviceclient.Client, error) {
		return serviceclient.New(commandServerURL(), os.Getenv("OFFGRID_API_KEY"), httpClient)
	}
	env.Service.CreateArtifactFile = func(path string) (cli.ArtifactFile, error) {
		return os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	}
	env.Service.OpenProfileFile = func(path string) (io.ReadCloser, error) { return os.Open(path) }
	env.ModelPackage = modelPackageEnvironment().ModelPackage
	env.Computer = cli.ComputerDependencies{
		Setup: openComputerWorkspace, Control: runAgentControl,
		Submit: runTaskSubmit, Client: newCommandClient, Stdout: os.Stdout,
	}
	env.Logs = cli.LogDependencies{HomeDir: os.UserHomeDir, Stat: os.Stat}
	env.ModelVerify = modelVerifyEnvironment().ModelVerify
	env.Quantize = modelQuantizeEnvironment().Quantize
	env.ModelTransfer = modelTransferEnvironment().ModelTransfer
	env.Batch = cli.BatchDependencies{
		EnsureServer: ensureOffgridServerRunning,
		StartServer:  startOffgridServerInBackground,
		Sleep:        time.Sleep,
		ProcessFile: func(ctx context.Context, input, output string, concurrency int) error {
			engine := inference.NewLlamaHTTPEngine("http://localhost:11611")
			return batch.NewProcessor(engine, concurrency).ProcessFile(ctx, input, output)
		},
	}
	env.Audio = cli.AudioDependencies{
		Stat:        os.Stat,
		ReadFile:    os.ReadFile,
		Create:      func(path string) (io.WriteCloser, error) { return os.Create(path) },
		LongClient:  &http.Client{Timeout: 300 * time.Second},
		ShortClient: &http.Client{Timeout: 60 * time.Second},
		GetClient:   http.DefaultClient,
		NewEngine:   func(cfg audio.Config) (cli.AudioEngine, error) { return audio.NewEngine(cfg) },
	}
	env.Integrations = cli.IntegrationDependencies{
		Managed: managedIntegration,
		HomeDir: os.UserHomeDir, Install: integrations.InstallPluginBundle,
		LookPath: exec.LookPath,
	}
	env.Interactive = osInteractiveDependencies()
	env.LegacyAgent = cli.LegacyAgentDependencies{
		SubmitTask: runTaskSubmit,
		MCP: cli.MCPDependencies{
			ReadFile: os.ReadFile, MkdirAll: os.MkdirAll,
			WriteFile: os.WriteFile, Now: time.Now,
		},
	}
	return env
}

func legacyAgentEnvironment(env cli.Environment) cli.Environment {
	// Legacy agent lists use the historical localhost endpoint. Profile and
	// task commands retain the configured, authenticated service client.
	env.Service.Client = func() (*serviceclient.Client, error) {
		return serviceclient.New(fmt.Sprintf("http://localhost:%d", config.LoadConfig().ServerPort), os.Getenv("OFFGRID_API_KEY"), httpClient)
	}
	return env
}
