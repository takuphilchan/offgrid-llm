package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"runtime"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
)

var llamaReloadProcess cli.CommandRunner = reviewedProcessRunner{}

func reloadLlamaServer() error { return reloadLlamaServerWithModel("") }

// reloadLlamaServerWithModel preserves the model-transfer reload hook. Every
// executable receives separate argv values; paths never enter a shell line.
func reloadLlamaServerWithModel(modelPath string) error {
	if strings.ContainsAny(modelPath, "\x00\r\n") {
		return fmt.Errorf("invalid model path for llama-server reload")
	}
	if activeLlamaSystemdService() {
		if modelPath != "" {
			if err := llamaReloadProcess.Run(context.Background(), cli.CommandRequest{Capability: "llama-active-model", Stdin: strings.NewReader(modelPath + "\n"), Stdout: io.Discard, Stderr: io.Discard}); err != nil {
				printWarning(fmt.Sprintf("Could not update active model config: %v", err))
			}
		}
		fmt.Println()
		printInfo("Reloading inference server with new model...")
		var output bytes.Buffer
		err := llamaReloadProcess.Run(context.Background(), cli.CommandRequest{Capability: "llama-systemd-restart", Stdout: &output, Stderr: &output})
		if err != nil {
			return fmt.Errorf("failed to restart llama-server: %v\nOutput: %s", err, output.String())
		}
		time.Sleep(time.Second)
		if err := llamaReloadProcess.Run(context.Background(), cli.CommandRequest{Capability: "llama-systemd-active", Stdout: io.Discard, Stderr: io.Discard}); err != nil {
			return fmt.Errorf("llama-server failed to start - check logs with: sudo journalctl -u llama-server -n 50")
		}
		if err := waitForModelReady(config.ReadLlamaPort("8081"), 30); err != nil {
			printWarning(fmt.Sprintf("Model may still be loading: %v", err))
			printInfo("Large models can take 10-30 seconds to load")
		}
		printSuccess("Inference server reloaded")
		return nil
	}

	fmt.Println()
	printInfo("Starting llama-server in background...")
	if llamaReloadProcess.Run(context.Background(), cli.CommandRequest{Capability: "llama-process-status", Stdout: io.Discard, Stderr: io.Discard}) == nil {
		if err := waitForModelReady(config.ReadLlamaPort("8081"), 5); err == nil {
			printSuccess("llama-server is already running")
			return nil
		}
	}
	if modelPath == "" {
		return fmt.Errorf("no model path selected for llama-server reload")
	}
	resources, _ := resource.DetectResources()
	threads := runtime.NumCPU() / 2
	if resources != nil {
		threads = resources.GetOptimalThreads()
	} else if threads > 1 {
		threads--
	}
	if threads < 1 {
		threads = 1
	}
	contextSize, batchSize := 4096, 512
	if resources != nil {
		switch {
		case resources.AvailableRAM > 0 && resources.AvailableRAM < 4000:
			contextSize = 1024
		case resources.AvailableRAM > 0 && resources.AvailableRAM < 6000:
			contextSize = 2048
		case resources.AvailableRAM >= 12000:
			contextSize = 8192
		}
		if resources.AvailableRAM < 6000 {
			batchSize = 256
		}
	}
	port := config.ReadLlamaPort("8081")
	args, err := reviewedLlamaServerArgs(modelPath, port, threads, contextSize, 0, batchSize, resources != nil && resources.AvailableRAM < 8000)
	if err != nil {
		return err
	}
	if err := runLlamaBackground("llama-server", args); err != nil {
		return fmt.Errorf("failed to start llama-server: %w", err)
	}
	fmt.Println()
	printInfo("Waiting for llama-server to load model...")
	if err := waitForModelReady(port, 30); err != nil {
		return fmt.Errorf("llama-server failed to start: %w", err)
	}
	printSuccess("llama-server started successfully")
	return nil
}

func activeLlamaSystemdService() bool {
	return llamaReloadProcess.Run(context.Background(), cli.CommandRequest{Capability: "llama-systemd-active", Stdout: io.Discard, Stderr: io.Discard}) == nil || llamaReloadProcess.Run(context.Background(), cli.CommandRequest{Capability: "llama-systemd-status", Stdout: io.Discard, Stderr: io.Discard}) == nil
}
