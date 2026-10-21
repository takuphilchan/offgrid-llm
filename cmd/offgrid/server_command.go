package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var offgridServerHomeDir = os.UserHomeDir
var offgridServerHealth = ensureOffgridServerRunning

// The CLI may start only its own executable with the fixed serve argument.
// Tests replace this launcher and health probe to avoid touching a live service.
var launchOffgridServe = func(executable string, log io.Writer) (func() error, error) {
	cmd := exec.Command(executable, "serve")
	cmd.Stdout = log
	cmd.Stderr = log
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd.Wait, nil
}

func ensureOffgridServerRunning() error {
	cfg := config.LoadConfig()
	healthURL := fmt.Sprintf("http://localhost:%d/health", cfg.ServerPort)

	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				return nil, nil // Bypass proxy for localhost
			},
		},
	}

	resp, err := client.Get(healthURL)
	if err != nil {
		return fmt.Errorf("server not responding: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	return nil
}

// startOffgridServerInBackground starts the OffGrid server in background
func startOffgridServerInBackground() error {
	exePath, err := os.Executable()
	if err != nil || exePath == "" {
		exePath = "offgrid"
	}

	homeDir, _ := offgridServerHomeDir()
	logDir := filepath.Join(homeDir, ".offgrid", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	logPath := filepath.Join(logDir, "server.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open server log: %w", err)
	}

	wait, err := launchOffgridServe(exePath, logFile)
	if err != nil {
		logFile.Close()
		return fmt.Errorf("failed to start %s serve: %w", exePath, err)
	}

	go func() {
		_ = wait()
		_ = logFile.Close()
	}()

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if err := offgridServerHealth(); err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("server did not become ready; check %s", logPath)
}

// ensureLlamaServerRunning checks if llama-server is responding
func ensureLlamaServerRunning() error {
	port := config.ReadLlamaPort("42382")
	healthURL := fmt.Sprintf("http://localhost:%s/health", port)

	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				return nil, nil // Bypass proxy for localhost
			},
		},
	}

	resp, err := client.Get(healthURL)
	if err != nil {
		return fmt.Errorf("llama-server not responding: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("llama-server returned status %d", resp.StatusCode)
	}

	return nil
}

// waitForModelReady waits for llama-server to report a healthy loaded model.
func waitForModelReady(port string, maxWaitSeconds int) error {
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				return nil, nil
			},
		},
	}

	startTime := time.Now()
	lastStatus := ""

	for {
		elapsed := time.Since(startTime)
		if elapsed > time.Duration(maxWaitSeconds)*time.Second {
			return fmt.Errorf("timeout waiting for model to be ready (waited %ds)", maxWaitSeconds)
		}

		// First check health endpoint
		healthResp, err := client.Get(fmt.Sprintf("http://localhost:%s/health", port))
		if err != nil {
			if lastStatus != "connecting" {
				lastStatus = "connecting"
			}
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if healthResp.StatusCode == http.StatusServiceUnavailable {
			healthResp.Body.Close()
			time.Sleep(1 * time.Second)
			continue
		}
		if healthResp.StatusCode != http.StatusOK {
			healthResp.Body.Close()
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Parse health response to check model loading status.
		var healthData struct {
			Status          string `json:"status"`
			SlotsIdle       int    `json:"slots_idle"`
			SlotsProcessing int    `json:"slots_processing"`
		}
		if err := json.NewDecoder(healthResp.Body).Decode(&healthData); err == nil {
			// llama.cpp returns "loading model" status while loading
			if healthData.Status == "loading model" {
				if lastStatus != "loading" {
					lastStatus = "loading"
				}
				healthResp.Body.Close()
				time.Sleep(1 * time.Second)
				continue
			}
			// Check if status is "ok" and we have idle slots
			if healthData.Status != "ok" {
				healthResp.Body.Close()
				time.Sleep(500 * time.Millisecond)
				continue
			}
		}
		healthResp.Body.Close()
		return nil
	}
}

// startLlamaServerInBackground starts llama-server with the specified model
func startLlamaServerInBackground(modelPath string) error {
	if models.IsManagedPackagePath(modelPath) {
		return fmt.Errorf("managed speech packages cannot be loaded by llama.cpp; use their declared speech adapter")
	}
	// Get llama-server binary via BinaryManager (auto-downloads if needed)
	homeDir, _ := os.UserHomeDir()
	binDir := filepath.Join(homeDir, ".offgrid-llm", "bin")
	bm := inference.NewBinaryManager(binDir)

	llamaServerPath, err := bm.GetLlamaServer()
	if err != nil {
		return fmt.Errorf("llama-server not available: %w", err)
	}

	// Detect system resources for optimal configuration
	res, _ := resource.DetectResources()

	// Get optimal thread count using physical core detection
	var threads int
	if res != nil {
		threads = res.GetOptimalThreads()
	} else {
		// Fallback: use half of logical cores
		cpuCores := runtime.NumCPU()
		threads = cpuCores / 2
		if threads < 1 {
			threads = 1
		}
	}

	// Adaptive context size based on available RAM
	contextSize := 4096
	if res != nil && res.AvailableRAM > 0 {
		if res.AvailableRAM < 4000 {
			contextSize = 1024
		} else if res.AvailableRAM < 6000 {
			contextSize = 2048
		} else if res.AvailableRAM < 12000 {
			contextSize = 4096
		} else {
			contextSize = 8192
		}
	}

	// Adaptive batch size - lower batch = faster time-to-first-token
	batchSize := 512
	if res != nil && res.AvailableRAM < 6000 {
		batchSize = 256
	}

	// Read port from platform-specific config file, default to 42382
	port := config.ReadLlamaPort("42382")

	// Determine GPU layers based on detection
	gpuLayers := 0
	if bm.HasGPU() {
		gpuLayers = 99 // Offload all layers to GPU
	}

	args, err := reviewedLlamaServerArgs(modelPath, port, threads, contextSize, gpuLayers, batchSize, res != nil && res.AvailableRAM < 8000)
	if err != nil {
		return err
	}
	if err := runLlamaBackground(llamaServerPath, args); err != nil {
		return fmt.Errorf("failed to start llama-server: %w", err)
	}
	return nil
}

func startDefaultServer(ctx context.Context) int {
	// Load configuration
	configPath := os.Getenv("OFFGRID_CONFIG")
	cfg, err := loadServeConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		return 1
	}
	if strings.TrimSpace(os.Getenv("OFFGRID_VERSION")) == "" {
		_ = os.Setenv("OFFGRID_VERSION", getVersion())
	}

	// Both default and explicit serve use the same composed application.
	if err := startApplication(ctx, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting server: %v\n", err)
		return 1
	}
	return 0
}
