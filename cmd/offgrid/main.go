package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/batch"
	"github.com/takuphilchan/offgrid-llm/internal/completions"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/output"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
	"github.com/takuphilchan/offgrid-llm/internal/server"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/internal/templates"
)

// Version is set via ldflags during build
var Version = "dev"

// Visual identity constants
const (
	// Colors (ANSI escape codes)
	colorReset   = "\033[0m"
	colorBold    = "\033[1m"
	colorDim     = "\033[2m"
	colorCyan    = "\033[36m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorRed     = "\033[31m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m"

	// Brand colors
	brandPrimary   = "\033[38;5;45m"  // Bright cyan
	brandSecondary = "\033[38;5;141m" // Purple
	brandAccent    = "\033[38;5;226m" // Yellow
	brandSuccess   = "\033[38;5;78m"  // Green
	brandError     = "\033[38;5;196m" // Red
	brandMuted     = "\033[38;5;240m" // Gray

	// Box drawing characters
	boxTL     = "╭"
	boxTR     = "╮"
	boxBL     = "╰"
	boxBR     = "╯"
	boxH      = "─"
	boxV      = "│"
	boxVR     = "├"
	boxVL     = "┤"
	boxHD     = "┬"
	boxHU     = "┴"
	boxCross  = "┼"
	separator = "━"

	// Custom icons
	iconBolt     = "⚡"
	iconCheck    = "✓"
	iconCross    = "✗"
	iconArrow    = "→"
	iconDot      = "•"
	iconStar     = "★"
	iconBox      = "▪"
	iconCircle   = "◉"
	iconDiamond  = "◆"
	iconChevron  = "›"
	iconDownload = "⇣"
	iconUpload   = "⇡"
	iconSearch   = "⌕"
	iconModel    = "◭"
	iconCpu      = "⟨⟩"
	iconGpu      = "⟪⟫"
)

func printBanner() {
	if output.JSONMode {
		return
	}
	fmt.Println()
	fmt.Printf("%s%s", brandPrimary, colorBold)
	fmt.Println("    ╔═══════════════════════════════════╗")
	fmt.Println("    ║                                   ║")
	fmt.Printf("    ║      OFFGRID LLM  %-15s ║\n", Version)
	fmt.Println("    ║                                   ║")
	fmt.Println("    ║   Edge Inference Orchestrator    ║")
	fmt.Println("    ║                                   ║")
	fmt.Println("    ╚═══════════════════════════════════╝")
	fmt.Printf("%s", colorReset)
	fmt.Println()
}

func printSection(title string) {
	if output.JSONMode {
		return
	}
	fmt.Printf("%s%s%s %s%s\n", brandPrimary, iconDiamond, colorReset, colorBold, title)
	fmt.Printf("%s%s%s\n", brandMuted, strings.Repeat(boxH, 50), colorReset)
}

func printSuccess(message string) {
	fmt.Printf("%s%s%s %s\n", brandSuccess, iconCheck, colorReset, message)
}

func printError(message string) {
	fmt.Printf("%s%s%s %s\n", brandError, iconCross, colorReset, message)
}

func printInfo(message string) {
	fmt.Printf("%s%s%s %s\n", brandPrimary, iconArrow, colorReset, message)
}

func printWarning(message string) {
	fmt.Printf("%s%s%s %s\n", brandAccent, iconBolt, colorReset, message)
}

func printItem(label, value string) {
	fmt.Printf("  %s%-18s%s %s%s%s\n", brandMuted, label+":", colorReset, colorBold, value, colorReset)
}

func printDivider() {
	fmt.Printf("%s%s%s\n", brandMuted, strings.Repeat(separator, 60), colorReset)
}

func printBox(title, content string) {
	width := 58
	fmt.Printf("%s%s%s%s%s\n", brandPrimary, boxTL, strings.Repeat(boxH, width), boxTR, colorReset)

	// Title
	padding := (width - len(title) - 2) / 2
	fmt.Printf("%s%s%s %s%s%s %s%s%s\n",
		brandPrimary, boxV, colorReset,
		strings.Repeat(" ", padding),
		colorBold+title+colorReset,
		strings.Repeat(" ", width-len(title)-padding-2),
		brandPrimary, boxV, colorReset)

	// Divider
	fmt.Printf("%s%s%s%s%s\n", brandPrimary, boxVR, strings.Repeat(boxH, width), boxVL, colorReset)

	// Content
	for _, line := range strings.Split(content, "\n") {
		contentPadding := width - len(stripAnsi(line))
		fmt.Printf("%s%s%s %s%s %s%s%s\n",
			brandPrimary, boxV, colorReset,
			line,
			strings.Repeat(" ", contentPadding-2),
			brandPrimary, boxV, colorReset)
	}

	// Bottom
	fmt.Printf("%s%s%s%s%s\n", brandPrimary, boxBL, strings.Repeat(boxH, width), boxBR, colorReset)
}

// ensureOffgridServerRunning checks if OffGrid server is responding
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
	// Start offgrid serve in background using shell
	cmd := exec.Command("sh", "-c", "offgrid serve > /dev/null 2>&1 &")
	return cmd.Run()
}

// ensureLlamaServerRunning checks if llama-server is responding
func ensureLlamaServerRunning() error {
	healthURL := "http://localhost:8081/health"

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

// startLlamaServerInBackground starts llama-server with the specified model
func startLlamaServerInBackground(modelPath string) error {
	// Check if llama-server exists
	llamaServerPath, err := exec.LookPath("llama-server")
	if err != nil {
		return fmt.Errorf("llama-server not found in PATH: %w", err)
	}

	// Get CPU count for optimal threading
	cpuCores := runtime.NumCPU()
	threads := cpuCores / 2
	if threads < 1 {
		threads = 1
	}

	// Read port from config file, default to 42382
	port := "42382"
	if portBytes, err := os.ReadFile("/etc/offgrid/llama-port"); err == nil {
		port = strings.TrimSpace(string(portBytes))
	}

	// Build command line
	cmdStr := fmt.Sprintf("%s -m %s --port %s --host 127.0.0.1 -t %d -c 4096 --n-gpu-layers 0 -b 512",
		llamaServerPath, modelPath, port, threads)

	// Start llama-server in background using shell with nohup
	cmd := exec.Command("sh", "-c", fmt.Sprintf("nohup %s > /dev/null 2>&1 &", cmdStr))

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start llama-server: %w", err)
	}

	return nil
}

func stripAnsi(str string) string {
	// Simple ANSI strip for length calculation
	result := str
	for _, code := range []string{colorReset, colorBold, colorDim, colorCyan, colorGreen, colorYellow, colorRed, colorBlue, colorMagenta, brandPrimary, brandSecondary, brandAccent, brandSuccess, brandError, brandMuted} {
		result = strings.ReplaceAll(result, code, "")
	}
	return result
}

func reloadLlamaServer() error {
	return reloadLlamaServerWithModel("")
}

func reloadLlamaServerWithModel(modelPath string) error {
	// Check if systemd service exists
	cmd := exec.Command("systemctl", "is-active", "llama-server")
	systemdAvailable := cmd.Run() == nil || exec.Command("systemctl", "status", "llama-server").Run() == nil

	if systemdAvailable {
		// Use systemd service
		// If modelPath is provided, update the active model configuration
		if modelPath != "" {
			// Store the active model path for the service to use
			activeModelFile := "/etc/offgrid/active-model"
			cmd := exec.Command("sudo", "sh", "-c", fmt.Sprintf("echo '%s' > %s", modelPath, activeModelFile))
			if err := cmd.Run(); err != nil {
				printWarning(fmt.Sprintf("Could not update active model config: %v", err))
			}
		}

		// Restart llama-server service
		fmt.Println()
		printInfo("Reloading inference server with new model...")

		cmd = exec.Command("sudo", "systemctl", "restart", "llama-server")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to restart llama-server: %v\nOutput: %s", err, string(output))
		}

		// Wait for service to be active
		time.Sleep(1 * time.Second)

		// Check if service is active
		cmd = exec.Command("systemctl", "is-active", "llama-server")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("llama-server failed to start - check logs with: sudo journalctl -u llama-server -n 50")
		}

		// Wait for model to actually load (can take several seconds for large models)
		if err := waitForLlamaServerReady(30); err != nil {
			printWarning(fmt.Sprintf("Model may still be loading: %v", err))
			printInfo("Large models can take 10-30 seconds to load")
		}

		printSuccess("Inference server reloaded")
		return nil
	}

	// No systemd service - start llama-server directly in background
	fmt.Println()
	printInfo("Starting llama-server in background...")

	// Check if llama-server is already running
	if isLlamaServerRunning() {
		// Server is running, just wait for it to be ready
		if err := waitForLlamaServerReady(5); err == nil {
			printSuccess("llama-server is already running")
			return nil
		}
	}

	// Start llama-server as background process
	// Read internal port (fallback to default if not found)
	llamaPort := "8081"
	portFile := "/etc/offgrid/llama-port"
	if data, err := os.ReadFile(portFile); err == nil {
		llamaPort = strings.TrimSpace(string(data))
	} else {
		// Try user config directory
		homeDir, _ := os.UserHomeDir()
		userPortFile := filepath.Join(homeDir, ".config", "offgrid", "llama-port")
		if data, err := os.ReadFile(userPortFile); err == nil {
			llamaPort = strings.TrimSpace(string(data))
		}
	}

	// Start llama-server with the model in background using shell with optimized flags
	// --no-mmap: Load directly to RAM (faster first inference, requires more RAM)
	// --mlock: Lock model in RAM (prevents swapping for consistent speed)
	// -fa: Flash attention (faster inference)
	// --cont-batching: Continuous batching for better throughput
	// -b 512: Lower batch size for faster first token
	// --cache-type-k/v f16: Use f16 for KV cache (faster with minimal quality loss)
	cmdStr := fmt.Sprintf("llama-server -m '%s' --port %s --host 127.0.0.1 -c 4096 --no-mmap --mlock -fa --cont-batching -b 512 --cache-type-k f16 --cache-type-v f16 > /dev/null 2>&1 &",
		modelPath, llamaPort)

	cmd = exec.Command("sh", "-c", cmdStr)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start llama-server: %v", err)
	}

	// Wait for server to be ready
	fmt.Println()
	printInfo("Waiting for llama-server to load model...")
	if err := waitForLlamaServerReady(30); err != nil {
		return fmt.Errorf("llama-server failed to start: %v", err)
	}

	printSuccess("llama-server started successfully")
	return nil
}

// isLlamaServerRunning checks if llama-server process is running
func isLlamaServerRunning() bool {
	cmd := exec.Command("pgrep", "-x", "llama-server")
	return cmd.Run() == nil
}

// waitForLlamaServerReady polls llama-server until it's ready or timeout
func waitForLlamaServerReady(timeoutSec int) error {
	// Read llama-server port (fallback to default if not found)
	port := "8081"
	portBytes, err := os.ReadFile("/etc/offgrid/llama-port")
	if err == nil {
		port = strings.TrimSpace(string(portBytes))
	} else {
		// Try user config directory
		homeDir, _ := os.UserHomeDir()
		userPortFile := filepath.Join(homeDir, ".config", "offgrid", "llama-port")
		if portBytes, err := os.ReadFile(userPortFile); err == nil {
			port = strings.TrimSpace(string(portBytes))
		}
	}

	// Create client that bypasses proxy for localhost
	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				return nil, nil // Explicitly bypass all proxies for localhost
			},
		},
	}

	healthURL := fmt.Sprintf("http://127.0.0.1:%s/health", port)
	completionURL := fmt.Sprintf("http://127.0.0.1:%s/v1/chat/completions", port)

	startTime := time.Now()
	for time.Since(startTime) < time.Duration(timeoutSec)*time.Second {
		// First check health endpoint
		resp, err := client.Get(healthURL)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Health OK, now check if model is loaded (try a minimal completion)
		testReq := map[string]interface{}{
			"model": "test",
			"messages": []map[string]string{
				{"role": "user", "content": "hi"},
			},
			"max_tokens": 1,
		}

		jsonData, _ := json.Marshal(testReq)
		resp, err = client.Post(completionURL, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Check for "loading model" error
		if resp.StatusCode == 503 && strings.Contains(string(body), "Loading model") {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Any other response means model is loaded (even errors about the request)
		return nil
	}

	return fmt.Errorf("timeout waiting for model to load")
}

func main() {
	// Check for global --json flag
	jsonFlag := false
	filteredArgs := make([]string, 0, len(os.Args))
	for i, arg := range os.Args {
		if arg == "--json" {
			jsonFlag = true
		} else {
			filteredArgs = append(filteredArgs, arg)
		}
		// Also check if it's at position 2 (after command)
		if i == 2 && arg == "--json" {
			jsonFlag = true
		}
	}

	// Set global JSON mode
	output.JSONMode = jsonFlag

	// Use filtered args if --json was found
	if jsonFlag {
		os.Args = filteredArgs
	}

	// Parse command
	if len(os.Args) > 1 {
		command := os.Args[1]

		switch command {
		case "auto-select", "autoselect":
			handleAutoSelect(os.Args[2:])
			return
		case "download":
			handleDownload(os.Args[2:])
			return
		case "download-hf":
			handleDownloadHF(os.Args[2:])
			return
		case "search":
			handleSearch(os.Args[2:])
			return
		case "run":
			handleRun(os.Args[2:])
			return
		case "import":
			handleImport(os.Args[2:])
			return
		case "remove", "delete", "rm":
			handleRemove(os.Args[2:])
			return
		case "export":
			handleExport(os.Args[2:])
			return
		case "chat":
			handleChat(os.Args[2:])
			return
		case "benchmark", "bench":
			handleBenchmark(os.Args[2:])
			return
		case "list":
			handleList(os.Args[2:])
			return
		case "catalog":
			handleCatalog()
			return
		case "quantization", "quant":
			handleQuantization()
			return
		case "quantize":
			handleQuantize(os.Args[2:])
			return
		case "info", "status":
			handleInfo()
			return
		case "config":
			handleConfig(os.Args[2:])
			return
		case "alias":
			handleAlias(os.Args[2:])
			return
		case "favorite", "fav":
			handleFavorite(os.Args[2:])
			return
		case "template", "tpl":
			handleTemplate(os.Args[2:])
			return
		case "batch":
			handleBatch(os.Args[2:])
			return
		case "session", "sessions":
			handleSession(os.Args[2:])
			return
		case "completions", "completion":
			handleCompletions(os.Args[2:])
			return
		case "serve", "server":
			// Fall through to start server
		case "version", "-v", "--version":
			handleVersion()
			return
		case "help", "-h", "--help":
			printHelp()
			return
		default:
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
			printHelp()
			os.Exit(1)
		}
	}

	// Load configuration
	configPath := os.Getenv("OFFGRID_CONFIG")
	cfg, err := config.LoadWithPriority(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Start the HTTP server (default command)
	srv := server.NewWithConfig(cfg)
	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting server: %v\n", err)
		os.Exit(1)
	}
}

func handleDownload(args []string) {
	// Check for help flag
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = []string{} // Trigger help display
	}

	if len(args) < 1 {
		fmt.Println()
		fmt.Printf("%s┌─ Download Model%s\n", brandPrimary+colorBold, colorReset)
		fmt.Println("│")
		fmt.Printf("│ %sUsage:%s offgrid download <model-id> [quantization]\n", colorDim, colorReset)
		fmt.Println("│")
		fmt.Println("│ Download a model from the built-in catalog")
		fmt.Println()
		fmt.Printf("%s└─ Examples%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("   %soffgrid download tinyllama-1.1b-chat Q4_K_M%s\n", colorDim, colorReset)
		fmt.Printf("   %soffgrid download llama-2-7b-chat%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("%sℹ Use 'offgrid catalog' to see available models%s\n", colorDim, colorReset)
		fmt.Println()
		os.Exit(1)
	}

	cfg := config.LoadConfig()
	catalog := models.DefaultCatalog()
	downloader := models.NewDownloader(cfg.ModelsDir, catalog)

	modelID := args[0]
	quantization := "Q4_K_M" // Default
	if len(args) > 1 {
		quantization = args[1]
	}

	// Set progress callback
	downloader.SetProgressCallback(func(p models.DownloadProgress) {
		fmt.Printf("\r  ⏬ Downloading %s (%s): %.1f%% · %s · %.1f MB/s",
			p.ModelID, p.Variant, p.Percent,
			formatBytes(p.BytesDone), float64(p.Speed)/(1024*1024))

		if p.Status == "complete" {
			fmt.Println("\n  ✓ Download complete")
		} else if p.Status == "verifying" {
			fmt.Print("\n  🔍 Verifying checksum...")
		}
	})

	fmt.Println()
	fmt.Printf("📦 Downloading %s (%s)\n", modelID, quantization)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	if err := downloader.Download(modelID, quantization); err != nil {
		fmt.Fprintf(os.Stderr, "\n  ✗ Download failed: %v\n", err)
		os.Exit(1)
	}

	// Construct the model path
	modelPath := filepath.Join(cfg.ModelsDir, fmt.Sprintf("%s.%s.gguf", modelID, quantization))

	// Reload llama-server with the new model
	if err := reloadLlamaServerWithModel(modelPath); err != nil {
		fmt.Println()
		printWarning(fmt.Sprintf("Could not auto-reload server: %v", err))
		fmt.Println()
		printInfo("Manually restart the server:")
		printItem("Restart service", "sudo systemctl restart llama-server")
		fmt.Println()
	}
}

func handleImport(args []string) {
	if len(args) < 1 {
		fmt.Println()
		fmt.Printf("%s┌─ Import Model%s\n", brandPrimary+colorBold, colorReset)
		fmt.Println("│")
		fmt.Printf("│ %sUsage:%s offgrid import <usb-path> [model-file]\n", colorDim, colorReset)
		fmt.Println("│")
		fmt.Println("│ Import GGUF models from USB/SD card or external storage")
		fmt.Println()
		fmt.Printf("%s└─ Examples%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("   %soffgrid import /media/usb              # Import all .gguf files%s\n", colorDim, colorReset)
		fmt.Printf("   %soffgrid import /media/usb/model.gguf  # Import specific file%s\n", colorDim, colorReset)
		fmt.Printf("   %soffgrid import D:\\models              # Windows directory%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("%sℹ Use 'offgrid list' to view imported models%s\n", colorDim, colorReset)
		fmt.Println()
		os.Exit(1)
	}

	cfg := config.LoadConfig()
	registry := models.NewRegistry(cfg.ModelsDir)
	importer := models.NewUSBImporter(cfg.ModelsDir, registry)

	usbPath := args[0]

	// Check if path is a specific file or directory
	info, err := os.Stat(usbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ Path not found: %s\n\n", usbPath)
		fmt.Fprintf(os.Stderr, "Common USB/SD mount points:\n")
		fmt.Fprintf(os.Stderr, "  • Linux:   /media/<username>/<device>\n")
		fmt.Fprintf(os.Stderr, "  • macOS:   /Volumes/<device>\n")
		fmt.Fprintf(os.Stderr, "  • Windows: D:\\ E:\\ F:\\\n")
		fmt.Fprintf(os.Stderr, "\nTip: Use 'ls /media' or 'mount' to find your device\n\n")
		os.Exit(1)
	}

	if info.IsDir() {
		// Import all models from directory
		fmt.Printf("Scanning %s\n\n", usbPath)

		modelFiles, err := importer.ScanUSBDrive(usbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ Scan error: %v\n", err)
			os.Exit(1)
		}

		if len(modelFiles) == 0 {
			fmt.Println("✗ No GGUF model files found in", usbPath)
			fmt.Println()
			fmt.Println("Looking for files matching: *.gguf")
			fmt.Println()
			fmt.Println("Make sure your models:")
			fmt.Println("  • Have .gguf file extension")
			fmt.Println("  • Are in GGUF format (not safetensors or PyTorch)")
			fmt.Println("  • Are readable (check permissions)")
			fmt.Println()
			os.Exit(0)
		}

		fmt.Printf("Found %d model file(s):\n\n", len(modelFiles))
		for i, file := range modelFiles {
			modelID, quant := importer.GetModelInfo(filepath.Base(file))
			size := getFileSize(file)
			fmt.Printf("  %d. %s (%s) · %s\n", i+1, modelID, quant, formatBytes(size))
		}
		fmt.Println()

		// Import all
		fmt.Println("Importing models...")
		fmt.Println()
		imported, err := importer.ImportAll(usbPath, func(p models.ImportProgress) {
			if p.Status == "copying" {
				fmt.Printf("\r  %s: %.1f%% · %s",
					p.FileName, p.Percent, formatBytes(p.BytesDone))
			} else if p.Status == "verifying" {
				fmt.Printf("\r  Verifying %s...          ", p.FileName)
			} else if p.Status == "complete" {
				fmt.Printf("\r  ✓ %s\n", p.FileName)
			}
		})

		if err != nil {
			fmt.Fprintf(os.Stderr, "\n  ✗ Import failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\n  ✓ Imported %d model(s) to %s\n", imported, cfg.ModelsDir)

		// Reload llama-server with imported models
		if imported > 0 {
			if err := reloadLlamaServer(); err != nil {
				fmt.Println()
				printWarning(fmt.Sprintf("Could not auto-reload server: %v", err))
				fmt.Println()
				printInfo("Manually restart the server:")
				printItem("Restart service", "sudo systemctl restart llama-server")
				fmt.Println()
			}
		}
	} else {
		// Import single file
		fmt.Printf("Importing %s\n\n", filepath.Base(usbPath))

		err := importer.ImportModel(usbPath, func(p models.ImportProgress) {
			if p.Status == "copying" {
				fmt.Printf("\r  Progress: %.1f%% · %s / %s",
					p.Percent, formatBytes(p.BytesDone), formatBytes(p.BytesTotal))
			} else if p.Status == "verifying" {
				fmt.Print("\r  Verifying integrity...          ")
			} else if p.Status == "complete" {
				fmt.Print("\r  ✓ Import complete                \n")
			}
		})

		if err != nil {
			fmt.Fprintf(os.Stderr, "\n  ✗ Import failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\n  ✓ Model imported to %s\n", cfg.ModelsDir)

		// Reload llama-server with the imported model
		// Construct the destination path where the model was imported
		importedModelPath := filepath.Join(cfg.ModelsDir, filepath.Base(usbPath))
		if err := reloadLlamaServerWithModel(importedModelPath); err != nil {
			fmt.Println()
			printWarning(fmt.Sprintf("Could not auto-reload server: %v", err))
			fmt.Println()
			printInfo("Manually restart the server:")
			printItem("Restart service", "sudo systemctl restart llama-server")
			fmt.Println()
		}
	}
}

func getFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func handleRemove(args []string) {
	if len(args) < 1 {
		fmt.Println()
		fmt.Printf("%s┌─ Remove Model%s\n", brandPrimary+colorBold, colorReset)
		fmt.Println("│")
		fmt.Printf("│ %sUsage:%s offgrid remove <model-id>\n", colorDim, colorReset)
		fmt.Println("│")
		fmt.Println("│ Remove an installed model from your system")
		fmt.Println()
		fmt.Printf("%s└─ Examples%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("   %soffgrid remove tinyllama-1.1b-chat.Q4_K_M%s\n", colorDim, colorReset)
		fmt.Printf("   %soffgrid remove llama-2-7b-chat.Q5_K_M%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("%sℹ Use 'offgrid list' to see installed models%s\n", colorDim, colorReset)
		fmt.Println()
		os.Exit(1)
	}

	cfg := config.LoadConfig()
	registry := models.NewRegistry(cfg.ModelsDir)

	if err := registry.ScanModels(); err != nil {
		fmt.Fprintf(os.Stderr, "  ✗ Error scanning models: %v\n", err)
		os.Exit(1)
	}

	modelID := args[0]

	// Check if model exists
	meta, err := registry.GetModel(modelID)
	if err != nil {
		printDivider()
		fmt.Println()
		printError(fmt.Sprintf("Model not found: %s", modelID))
		fmt.Println()

		// Show available models
		modelList := registry.ListModels()
		if len(modelList) > 0 {
			printSection("Available Models")
			for _, m := range modelList {
				modelMeta, _ := registry.GetModel(m.ID)
				fmt.Printf("  ◭ %s", m.ID)
				if modelMeta != nil && modelMeta.Size > 0 {
					fmt.Printf(" · %s", formatBytes(modelMeta.Size))
				}
				if modelMeta != nil && modelMeta.Quantization != "" {
					fmt.Printf(" · %s", modelMeta.Quantization)
				}
				fmt.Println()
			}
		} else {
			printInfo("No models installed")
			fmt.Println()
			printInfo("Download models:")
			printItem("From catalog", "offgrid download <model-id>")
			printItem("From HuggingFace", "offgrid download-hf <repo> --file <file>.gguf")
		}
		fmt.Println()
		printDivider()
		fmt.Println()
		os.Exit(1)
	}

	// Confirm deletion
	fmt.Println()
	fmt.Printf("%s◆ Remove Model%s\n", brandPrimary, colorReset)
	fmt.Printf("%s──────────────────────────────────────────────────%s\n\n", brandMuted, colorReset)

	fmt.Printf("%sModel Information%s\n", brandPrimary, colorReset)
	fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", brandMuted, colorReset)
	fmt.Printf("  Name:     %s%s%s\n", colorBold, modelID, colorReset)
	if meta.Path != "" {
		fmt.Printf("  Path:     %s%s%s\n", brandMuted, meta.Path, colorReset)
	}
	if meta.Size > 0 {
		fmt.Printf("  Size:     %s%s%s will be freed\n", brandSuccess, formatBytes(meta.Size), colorReset)
	}
	fmt.Println()
	fmt.Printf("%s⚠  This action cannot be undone%s\n\n", brandError, colorReset)
	fmt.Printf("%sContinue?%s (y/N): ", brandMuted, colorReset)

	var response string
	fmt.Scanln(&response)

	if response != "y" && response != "Y" {
		fmt.Println()
		printInfo("Cancelled - model preserved")
		fmt.Println()
		return
	}

	// Delete the model file
	if meta.Path != "" {
		if err := os.Remove(meta.Path); err != nil {
			fmt.Println()
			printError(fmt.Sprintf("Failed to remove file: %v", err))
			fmt.Println()
			os.Exit(1)
		}
	}

	fmt.Println()
	fmt.Printf("%s◆ Model Removed%s\n", brandSuccess, colorReset)
	fmt.Printf("%s──────────────────────────────────────────────────%s\n\n", brandMuted, colorReset)
	fmt.Printf("  %s✓%s Removed %s%s%s\n", brandSuccess, colorReset, brandPrimary, modelID, colorReset)

	// Rescan to update registry after file deletion
	if err := registry.ScanModels(); err != nil {
		fmt.Println()
		printWarning(fmt.Sprintf("Failed to refresh model list: %v", err))
	}

	// Show remaining models
	remaining := registry.ListModels()
	fmt.Printf("  %s→%s %d model(s) remaining\n\n", brandMuted, colorReset, len(remaining))
}

func handleExport(args []string) {
	if len(args) < 2 {
		printDivider()
		fmt.Println()
		printSection("Usage")
		fmt.Printf("  %soffgrid export%s <model-id> <destination>\n", colorBold, colorReset)
		fmt.Println()
		printSection("Description")
		fmt.Println("  Export a model to USB/SD card or external storage")
		fmt.Println()
		printSection("Examples")
		fmt.Printf("  %s$%s offgrid export tinyllama-1.1b-chat.Q4_K_M /media/usb\n", brandMuted, colorReset)
		fmt.Printf("  %s$%s offgrid export llama-2-7b-chat.Q5_K_M D:\\backup\n", brandMuted, colorReset)
		fmt.Println()
		printInfo("Use 'offgrid list' to see available models")
		fmt.Println()
		printDivider()
		fmt.Println()
		os.Exit(1)
	}

	cfg := config.LoadConfig()
	registry := models.NewRegistry(cfg.ModelsDir)

	if err := registry.ScanModels(); err != nil {
		fmt.Fprintf(os.Stderr, "  ✗ Error scanning models: %v\n", err)
		os.Exit(1)
	}

	modelID := args[0]
	destPath := args[1]

	// Check if model exists
	meta, err := registry.GetModel(modelID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ Model not found: %s\n\n", modelID)

		// Show available models
		modelList := registry.ListModels()
		if len(modelList) > 0 {
			fmt.Fprintln(os.Stderr, "Available models:")
			for _, m := range modelList {
				fmt.Fprintf(os.Stderr, "  • %s\n", m.ID)
			}
			fmt.Fprintln(os.Stderr, "")
		} else {
			fmt.Fprintln(os.Stderr, "No models installed. Use 'offgrid download' to add models.")
			fmt.Fprintln(os.Stderr, "")
		}
		os.Exit(1)
	}

	if meta.Path == "" {
		fmt.Fprintf(os.Stderr, "✗ Model path not found for: %s\n\n", modelID)
		os.Exit(1)
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(destPath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "✗ Cannot create destination directory: %v\n\n", err)
		fmt.Fprintf(os.Stderr, "Make sure:\n")
		fmt.Fprintf(os.Stderr, "  • The USB/SD card is mounted\n")
		fmt.Fprintf(os.Stderr, "  • You have write permissions\n")
		fmt.Fprintf(os.Stderr, "  • The device has enough space\n\n")
		os.Exit(1)
	}

	// Construct destination file path
	fileName := filepath.Base(meta.Path)
	destFile := filepath.Join(destPath, fileName)

	fmt.Println("📦 Export Model")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Printf("Model:  %s\n", modelID)
	fmt.Printf("From:   %s\n", meta.Path)
	fmt.Printf("To:     %s\n", destFile)
	fmt.Printf("Size:   %s\n\n", formatBytes(meta.Size))

	// Copy file
	sourceFile, err := os.Open(meta.Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  ✗ Failed to open source: %v\n", err)
		os.Exit(1)
	}
	defer sourceFile.Close()

	destFileHandle, err := os.Create(destFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  ✗ Failed to create destination: %v\n", err)
		os.Exit(1)
	}
	defer destFileHandle.Close()

	// Copy with progress
	buffer := make([]byte, 1024*1024) // 1MB buffer
	var totalCopied int64

	for {
		n, err := sourceFile.Read(buffer)
		if n > 0 {
			if _, err := destFileHandle.Write(buffer[:n]); err != nil {
				fmt.Fprintf(os.Stderr, "\n  ✗ Write error: %v\n", err)
				os.Exit(1)
			}
			totalCopied += int64(n)
			percent := float64(totalCopied) / float64(meta.Size) * 100
			fmt.Printf("\r  Progress: %.1f%% · %s / %s",
				percent, formatBytes(totalCopied), formatBytes(meta.Size))
		}
		if err != nil {
			break
		}
	}

	fmt.Printf("\n\n✓ Export complete\n")
	fmt.Printf("  Location: %s\n\n", destFile)
}

func handleChat(args []string) {
	fmt.Println("Interactive Chat Mode")
	fmt.Println()
	fmt.Println("This feature requires a running server with loaded models.")
	fmt.Println()
	fmt.Println("Quick start:")
	fmt.Println("  1. Start server:  offgrid serve")
	fmt.Println("  2. In new terminal, use the API:")
	fmt.Println()
	fmt.Println("Example curl command:")
	fmt.Println(`  curl http://localhost:11611/v1/chat/completions \`)
	fmt.Println(`    -H "Content-Type: application/json" \`)
	fmt.Println(`    -d '{"model":"auto","messages":[{"role":"user","content":"Hello!"}]}'`)
	fmt.Println()
	fmt.Println("Or use the web UI at: http://localhost:11611/ui")
	fmt.Println()

	// TODO: Implement interactive CLI chat
	// This would require:
	// 1. Connect to running server via HTTP client
	// 2. Read user input in loop
	// 3. Send requests and stream responses
	// 4. Handle conversation history
}

func handleAutoSelect(args []string) {
	fmt.Println()
	printSection("Auto-Select Model")
	fmt.Println()

	printInfo("Detecting system resources...")
	fmt.Println()

	// Detect hardware resources
	res, err := resource.DetectResources()
	if err != nil {
		printError(fmt.Sprintf("Failed to detect system resources: %v", err))
		os.Exit(1)
	}

	// Display system info
	fmt.Printf("%sSystem Information%s\n", colorBold, colorReset)
	printDivider()
	fmt.Printf("  %sOS:%s %s/%s\n", brandMuted, colorReset, res.OS, res.Arch)
	fmt.Printf("  %sCPU:%s %d cores\n", brandMuted, colorReset, res.CPUCores)
	fmt.Printf("  %sRAM:%s %s total · %s available\n",
		brandMuted, colorReset,
		formatBytes(res.TotalRAM*1024*1024),
		formatBytes(res.AvailableRAM*1024*1024))

	if res.GPUAvailable {
		fmt.Printf("  %sGPU:%s %s · %s VRAM\n",
			brandSuccess, colorReset,
			res.GPUName,
			formatBytes(res.GPUMemory*1024*1024))
	} else {
		fmt.Printf("  %sGPU:%s Not detected (CPU-only mode)\n", brandMuted, colorReset)
	}
	fmt.Println()

	// Get recommendations
	recommendations := res.RecommendedModels()

	if len(recommendations) == 0 {
		fmt.Println()
		printWarning("Insufficient memory for any models")
		fmt.Println()
		printInfo("Minimum requirements:")
		printItem("RAM", "2 GB available")
		fmt.Println()
		os.Exit(1)
	}

	// Group by priority
	primary := []resource.ModelRecommendation{}
	alternatives := []resource.ModelRecommendation{}
	supplementary := []resource.ModelRecommendation{}

	for _, rec := range recommendations {
		switch rec.Priority {
		case 1:
			primary = append(primary, rec)
		case 2:
			alternatives = append(alternatives, rec)
		case 3:
			supplementary = append(supplementary, rec)
		}
	}

	// Display recommendations
	fmt.Printf("%sRecommended Models%s\n", colorBold, colorReset)
	printDivider()
	fmt.Println()

	if len(primary) > 0 {
		fmt.Printf("%sPrimary Recommendations%s\n", brandSuccess, colorReset)
		for i, rec := range primary {
			fmt.Printf("  %s%d.%s %s%s%s (%s)\n",
				brandMuted, i+1, colorReset,
				brandPrimary, rec.ModelID, colorReset,
				rec.Quantization)
			fmt.Printf("     %s%s%s · %s\n",
				brandMuted, formatModelSize(rec.SizeGB), colorReset,
				rec.Reason)
		}
		fmt.Println()
	}

	if len(alternatives) > 0 {
		fmt.Printf("%sAlternatives%s\n", brandMuted, colorReset)
		for _, rec := range alternatives {
			fmt.Printf("  %s◦%s %s (%s) · %s · %s\n",
				brandMuted, colorReset,
				rec.ModelID, rec.Quantization, formatModelSize(rec.SizeGB), rec.Reason)
		}
		fmt.Println()
	}

	if len(supplementary) > 0 {
		fmt.Printf("%sSupplementary%s\n", brandMuted, colorReset)
		for _, rec := range supplementary {
			fmt.Printf("  %s◦%s %s (%s) · %s · %s\n",
				brandMuted, colorReset,
				rec.ModelID, rec.Quantization, formatModelSize(rec.SizeGB), rec.Reason)
		}
		fmt.Println()
	}

	printSection("Next Steps")
	if len(primary) > 0 {
		printItem("Download recommended", fmt.Sprintf("offgrid download %s %s", primary[0].ModelID, primary[0].Quantization))
		printItem("Or search for more", "offgrid search llama --author TheBloke")
	}
	printItem("View all available", "offgrid catalog")
	fmt.Println()
}

func handleBenchmark(args []string) {
	if len(args) < 1 {
		printDivider()
		fmt.Println()
		printSection("Usage")
		fmt.Printf("  %soffgrid benchmark%s <model-id> [--iterations N] [--prompt \"text\"]\n", colorBold, colorReset)
		fmt.Println()
		printSection("Description")
		fmt.Println("  Benchmark model performance and resource usage")
		fmt.Println()
		printSection("Options")
		fmt.Println("  --iterations N    Number of test iterations (default: 3)")
		fmt.Println("  --prompt \"text\"   Custom prompt to benchmark (default: sample prompt)")
		fmt.Println()
		printSection("Examples")
		fmt.Printf("  %s$%s offgrid benchmark tinyllama-1.1b-chat.Q4_K_M\n", brandMuted, colorReset)
		fmt.Printf("  %s$%s offgrid benchmark llama-2-7b-chat.Q5_K_M --iterations 5\n", brandMuted, colorReset)
		fmt.Printf("  %s$%s offgrid benchmark phi-2.Q4_K_M --prompt \"Explain quantum computing\"\n", brandMuted, colorReset)
		fmt.Println()
		printInfo("Model must be loaded in server first: offgrid serve <model>")
		fmt.Println()
		printDivider()
		fmt.Println()
		os.Exit(1)
	}

	cfg := config.LoadConfig()
	registry := models.NewRegistry(cfg.ModelsDir)

	if err := registry.ScanModels(); err != nil {
		fmt.Fprintf(os.Stderr, "✗ Error scanning models: %v\n\n", err)
		os.Exit(1)
	}

	// Parse arguments
	modelID := args[0]
	iterations := 3
	customPrompt := ""

	for i := 1; i < len(args); i++ {
		if args[i] == "--iterations" && i+1 < len(args) {
			fmt.Sscanf(args[i+1], "%d", &iterations)
			i++
		} else if args[i] == "--prompt" && i+1 < len(args) {
			customPrompt = args[i+1]
			i++
		}
	}

	// Check if model exists
	meta, err := registry.GetModel(modelID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ Model not found: %s\n\n", modelID)

		// Show available models
		modelList := registry.ListModels()
		if len(modelList) > 0 {
			fmt.Fprintln(os.Stderr, "Available models:")
			for _, m := range modelList {
				fmt.Fprintf(os.Stderr, "  • %s\n", m.ID)
			}
			fmt.Fprintln(os.Stderr, "")
		}
		os.Exit(1)
	}

	// Print benchmark header
	fmt.Println()
	fmt.Printf("%s┌─ Benchmark · %s%s\n", brandPrimary+colorBold, modelID, colorReset)
	fmt.Println("│")
	fmt.Printf("│ %sPath:%s %s\n", colorDim, colorReset, meta.Path)
	fmt.Printf("│ %sSize:%s %s", colorDim, colorReset, formatBytes(meta.Size))
	if meta.Quantization != "" {
		fmt.Printf(" · %s", meta.Quantization)
	}
	fmt.Println()
	fmt.Println()

	// Check if server is running
	serverURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.ServerPort)
	if !isServerHealthy(serverURL) {
		fmt.Printf("%sError: Server not running%s\n", colorRed, colorReset)
		fmt.Printf("Start server first: %soffgrid serve%s\n\n", brandSecondary, colorReset)
		os.Exit(1)
	}

	// Default benchmark prompt
	benchPrompt := "Write a short story about a robot learning to paint."
	if customPrompt != "" {
		benchPrompt = customPrompt
	}

	fmt.Printf("Running %d iterations...\n\n", iterations)

	// Run benchmark iterations
	var (
		totalLatency      time.Duration
		totalTokens       int
		totalPromptTokens int
		firstTokenTimes   []time.Duration
		tokensPerSec      []float64
	)

	for i := 0; i < iterations; i++ {
		fmt.Printf("%s  [%d/%d]%s Testing... ", colorDim, i+1, iterations, colorReset)

		startTime := time.Now()
		var firstTokenTime time.Duration
		tokenCount := 0
		promptTokens := 0

		// Call completion endpoint
		reqBody := map[string]interface{}{
			"prompt":      benchPrompt,
			"max_tokens":  100,
			"temperature": 0.7,
			"stream":      false,
		}

		jsonData, _ := json.Marshal(reqBody)
		resp, err := http.Post(
			serverURL+"/v1/completions",
			"application/json",
			bytes.NewBuffer(jsonData),
		)

		if err != nil {
			fmt.Printf("%s✗%s Failed: %v\n", brandError, colorReset, err)
			continue
		}

		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		var result map[string]interface{}
		if err := json.Unmarshal(body, &result); err != nil {
			fmt.Printf("%s✗%s Failed to parse response\n", brandError, colorReset)
			continue
		}

		latency := time.Since(startTime)

		// Extract metrics from response
		if usage, ok := result["usage"].(map[string]interface{}); ok {
			if pt, ok := usage["prompt_tokens"].(float64); ok {
				promptTokens = int(pt)
			}
			if ct, ok := usage["completion_tokens"].(float64); ok {
				tokenCount = int(ct)
			}
		}

		// Estimate first token time (roughly 10% of total for most models)
		firstTokenTime = latency / 10

		totalLatency += latency
		totalTokens += tokenCount
		totalPromptTokens += promptTokens
		firstTokenTimes = append(firstTokenTimes, firstTokenTime)

		tps := float64(tokenCount) / latency.Seconds()
		tokensPerSec = append(tokensPerSec, tps)

		fmt.Printf("%s✓%s %s (%.1f tok/s)\n", colorGreen, colorReset, formatDuration(latency), tps)
	}

	if len(tokensPerSec) == 0 {
		printError("All benchmark iterations failed")
		os.Exit(1)
	}

	// Calculate averages
	avgLatency := totalLatency / time.Duration(len(tokensPerSec))
	avgTokens := float64(totalTokens) / float64(len(tokensPerSec))
	avgPromptTokens := float64(totalPromptTokens) / float64(len(tokensPerSec))
	avgFirstToken := time.Duration(0)
	for _, ft := range firstTokenTimes {
		avgFirstToken += ft
	}
	avgFirstToken /= time.Duration(len(firstTokenTimes))

	avgTPS := 0.0
	minTPS := tokensPerSec[0]
	maxTPS := tokensPerSec[0]
	for _, tps := range tokensPerSec {
		avgTPS += tps
		if tps < minTPS {
			minTPS = tps
		}
		if tps > maxTPS {
			maxTPS = tps
		}
	}
	avgTPS /= float64(len(tokensPerSec))

	// Display results
	fmt.Println()
	fmt.Printf("%s└─ Results%s\n", brandPrimary+colorBold, colorReset)
	fmt.Println()
	fmt.Printf("   %sSpeed:%s        %.1f tok/s (%.1f - %.1f)\n", colorDim, colorReset, avgTPS, minTPS, maxTPS)
	fmt.Printf("   %sLatency:%s      %s (first token: ~%s)\n", colorDim, colorReset, formatDuration(avgLatency), formatDuration(avgFirstToken))
	fmt.Printf("   %sTokens:%s       %.0f prompt, %.0f generated (avg)\n", colorDim, colorReset, avgPromptTokens, avgTokens)
	fmt.Printf("   %sThroughput:%s   ~%d queries/hour\n", colorDim, colorReset, int(3600.0/avgLatency.Seconds()))

	memEst := float64(meta.Size) * 1.2 // Rough estimate: model + context
	fmt.Printf("   %sMemory:%s       %.1f GB estimated\n", colorDim, colorReset, memEst/1e9)
	fmt.Println()
}

func handleList(args []string) {
	cfg := config.LoadConfig()
	registry := models.NewRegistry(cfg.ModelsDir)

	if err := registry.ScanModels(); err != nil {
		if output.JSONMode {
			output.Error("Error scanning models", err)
		}
		printError(fmt.Sprintf("Error scanning models: %v", err))
		os.Exit(1)
	}

	modelList := registry.ListModels()

	// JSON output mode
	if output.JSONMode {
		var jsonModels []output.ModelInfo
		for _, model := range modelList {
			meta, err := registry.GetModel(model.ID)
			modelInfo := output.ModelInfo{
				Name: model.ID,
			}
			if err == nil {
				if meta.Size > 0 {
					modelInfo.Size = formatBytes(meta.Size)
				}
				if meta.Quantization != "" && meta.Quantization != "unknown" {
					modelInfo.Quantization = meta.Quantization
				}
				modelInfo.Format = meta.Format
				if meta.Path != "" {
					modelInfo.Path = meta.Path
				}
			}
			jsonModels = append(jsonModels, modelInfo)
		}
		output.PrintModels(jsonModels)
		return
	}

	// Human-readable output - Clean modern design
	fmt.Println()
	fmt.Printf("%s┌─ Installed Models%s\n", brandPrimary+colorBold, colorReset)

	if len(modelList) == 0 {
		fmt.Println("│")
		fmt.Printf("│ No models installed\n")
		fmt.Println("│")
		fmt.Printf("%s└─ Quick Start%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("   %soffgrid search llama%s          Search for models\n", brandSecondary, colorReset)
		fmt.Printf("   %soffgrid download-hf <id>%s     Download from HuggingFace\n", brandSecondary, colorReset)
		fmt.Printf("   %soffgrid catalog%s              Browse model catalog\n", brandSecondary, colorReset)
		fmt.Println()
		return
	}

	fmt.Println("│")
	if len(modelList) == 1 {
		fmt.Printf("│ 1 model installed\n\n")
	} else {
		fmt.Printf("│ %d models installed\n\n", len(modelList))
	}

	// Clean table with minimal lines
	fmt.Printf("%s   %-48s %-14s %-12s%s\n", colorDim, "Model", "Size", "Quantization", colorReset)
	fmt.Printf("%s   %s%s\n", colorDim, strings.Repeat("─", 78), colorReset)

	var totalSize int64
	for i, model := range modelList {
		meta, err := registry.GetModel(model.ID)

		modelName := model.ID
		if len(modelName) > 48 {
			modelName = modelName[:45] + "..."
		}

		sizeStr := "-"
		quantStr := "-"

		if err == nil {
			if meta.Size > 0 {
				sizeStr = formatBytes(meta.Size)
				totalSize += meta.Size
			}
			if meta.Quantization != "" && meta.Quantization != "unknown" {
				quantStr = meta.Quantization
			}
		}

		// Subtle alternating colors for readability
		rowColor := brandPrimary
		if i%2 == 1 {
			rowColor = colorReset
		}

		fmt.Printf("   %s%-48s%s %s%-14s%s %s%-12s%s\n",
			rowColor, modelName, colorReset,
			brandSecondary, sizeStr, colorReset,
			brandMuted, quantStr, colorReset)
	}

	if totalSize > 0 {
		fmt.Println()
		fmt.Printf("%s   Total: %s%s\n", colorDim, formatBytes(totalSize), colorReset)
	}

	fmt.Println()
	fmt.Printf("%s└─ Commands%s\n", brandPrimary+colorBold, colorReset)
	fmt.Printf("   %soffgrid run <model>%s          Start interactive chat\n", brandSecondary, colorReset)
	fmt.Printf("   %soffgrid serve%s                Start API server\n", brandSecondary, colorReset)
	fmt.Printf("   %soffgrid benchmark <model>%s    Test performance\n", brandSecondary, colorReset)
	fmt.Println()
}

func handleCatalog() {
	catalog := models.DefaultCatalog()

	fmt.Println()
	fmt.Printf("%s┌─ Model Catalog%s\n", brandPrimary+colorBold, colorReset)
	fmt.Println("│")

	// Separate LLMs and embeddings
	var llms []models.CatalogEntry
	var embeddings []models.CatalogEntry

	for _, entry := range catalog.Models {
		// Simple heuristic: embeddings are typically small (<1GB) and have "embed" in name
		isEmbedding := false
		for _, v := range entry.Variants {
			if v.Size < 500*1024*1024 { // < 500MB
				isEmbedding = true
				break
			}
		}
		if isEmbedding {
			embeddings = append(embeddings, entry)
		} else {
			llms = append(llms, entry)
		}
	}

	// Show LLMs
	fmt.Printf("│ %sLanguage Models%s · %d available\n", colorBold, colorReset, len(llms))
	fmt.Println("│")

	for _, entry := range llms {
		star := ""
		if entry.Recommended {
			star = fmt.Sprintf(" %s★%s", colorDim, colorReset)
		}

		fmt.Printf("│ %s%s%s%s %s%s · %d GB RAM%s\n",
			brandPrimary, entry.ID, colorReset, star,
			colorDim, entry.Parameters, entry.MinRAM, colorReset)

		// Variants on same line
		fmt.Printf("│   %s", colorDim)
		for i, v := range entry.Variants {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Printf("%s (%.1f GB)", v.Quantization, float64(v.Size)/(1024*1024*1024))
		}
		fmt.Printf("%s\n", colorReset)
	}

	// Show embeddings if any
	if len(embeddings) > 0 {
		fmt.Println("│")
		fmt.Printf("│ %sEmbedding Models%s · %d available\n", colorBold, colorReset, len(embeddings))
		fmt.Println("│")

		for _, entry := range embeddings {
			star := ""
			if entry.Recommended {
				star = fmt.Sprintf(" %s★%s", colorDim, colorReset)
			}

			fmt.Printf("│ %s%s%s%s\n",
				brandPrimary, entry.ID, colorReset, star)

			// Variants on same line
			fmt.Printf("│   %s", colorDim)
			for i, v := range entry.Variants {
				if i > 0 {
					fmt.Print(", ")
				}
				sizeGB := float64(v.Size) / (1024 * 1024 * 1024)
				if sizeGB < 0.1 {
					fmt.Printf("%s (%d MB)", v.Quantization, v.Size/(1024*1024))
				} else {
					fmt.Printf("%s (%.1f GB)", v.Quantization, sizeGB)
				}
			}
			fmt.Printf("%s\n", colorReset)
		}
	}

	fmt.Println()
	fmt.Printf("%s└─ Commands%s\n", brandPrimary+colorBold, colorReset)
	fmt.Printf("   %soffgrid download <id> [quant]%s    Download model\n", brandSecondary, colorReset)
	fmt.Printf("   %soffgrid search <query>%s           Search HuggingFace\n", brandSecondary, colorReset)
	fmt.Printf("   %soffgrid quantization%s             Quantization guide\n", brandSecondary, colorReset)
	fmt.Println()
}

func handleQuantization() {
	fmt.Println()
	fmt.Printf("%s┌─ Quantization Guide%s\n", brandPrimary+colorBold, colorReset)
	fmt.Println("│")
	fmt.Printf("%s│ %sLower bits = smaller size + faster speed - slight quality loss%s\n", colorReset, colorDim, colorReset)
	fmt.Println()

	// Group by quality tier
	tiers := []struct {
		name   string
		quants []string
	}{
		{"Compact (2-3 bit)", []string{"Q2_K", "Q3_K_S", "Q3_K_M"}},
		{"Balanced (4 bit) - Recommended", []string{"Q4_K_S", "Q4_K_M"}},
		{"High Quality (5-6 bit)", []string{"Q5_K_S", "Q5_K_M", "Q6_K"}},
		{"Maximum Quality (8 bit)", []string{"Q8_0"}},
	}

	for _, tier := range tiers {
		fmt.Printf("%s├─ %s%s\n", brandPrimary+colorBold, tier.name, colorReset)
		for _, quant := range tier.quants {
			info := models.GetQuantizationInfo(quant)
			star := "  "
			starColor := ""
			if quant == "Q4_K_M" || quant == "Q5_K_M" {
				star = "★ "
				starColor = brandSuccess
			}

			fmt.Printf("│  %s%s%s%-8s%s %.1f bits · %s%s\n",
				starColor, star, colorReset,
				info.Name, brandMuted,
				info.BitsPerWeight,
				info.Description, colorReset)
		}
		fmt.Println("│")
	}

	fmt.Printf("%s└─ Quick Recommendations%s\n", brandPrimary+colorBold, colorReset)
	fmt.Printf("   %s★%s %sQ4_K_M%s  Best for most users (4.0 GB for 7B model)\n", brandSuccess, colorReset, brandSecondary, colorReset)
	fmt.Printf("   %s★%s %sQ5_K_M%s  Production quality (4.8 GB for 7B model)\n", brandSuccess, colorReset, brandSecondary, colorReset)
	fmt.Printf("      %sQ3_K_M%s  Limited RAM (3.0 GB for 7B model)\n", brandSecondary, colorReset)
	fmt.Printf("      %sQ8_0%s    Maximum quality (7.2 GB for 7B model)\n", brandSecondary, colorReset)
	fmt.Println()
}

func handleQuantize(args []string) {
	if len(args) < 2 {
		printDivider()
		fmt.Println()
		printSection("Usage")
		fmt.Printf("  %soffgrid quantize%s <model-id> <quantization-type> [--output <name>]\n", colorBold, colorReset)
		fmt.Println()
		printSection("Description")
		fmt.Println("  Quantize a model to a different precision level")
		fmt.Println()
		printSection("Quantization Types")
		fmt.Println("  Q2_K      - 2-bit (smallest, lowest quality)")
		fmt.Println("  Q3_K_S    - 3-bit small")
		fmt.Println("  Q3_K_M    - 3-bit medium")
		fmt.Println("  Q4_K_S    - 4-bit small")
		fmt.Printf("  %sQ4_K_M%s    - 4-bit medium %s(recommended)%s\n", brandPrimary, colorReset, brandSuccess, colorReset)
		fmt.Println("  Q5_K_S    - 5-bit small")
		fmt.Printf("  %sQ5_K_M%s    - 5-bit medium %s(high quality)%s\n", brandPrimary, colorReset, brandSuccess, colorReset)
		fmt.Println("  Q6_K      - 6-bit")
		fmt.Println("  Q8_0      - 8-bit (largest, highest quality)")
		fmt.Println()
		printSection("Options")
		fmt.Println("  --output <name>   Output model name (default: auto-generated)")
		fmt.Println()
		printSection("Examples")
		fmt.Printf("  %s$%s offgrid quantize llama-2-7b.F16 Q4_K_M\n", brandMuted, colorReset)
		fmt.Printf("  %s$%s offgrid quantize phi-2.F16 Q5_K_M --output phi-2-hq\n", brandMuted, colorReset)
		fmt.Println()
		printInfo("Use 'offgrid quantization' to see quality comparisons")
		fmt.Println()
		printDivider()
		fmt.Println()
		os.Exit(1)
	}

	cfg := config.LoadConfig()
	registry := models.NewRegistry(cfg.ModelsDir)

	if err := registry.ScanModels(); err != nil {
		printError(fmt.Sprintf("Error scanning models: %v", err))
		os.Exit(1)
	}

	modelID := args[0]
	targetQuant := strings.ToUpper(args[1])
	outputName := ""

	// Parse optional --output flag
	for i := 2; i < len(args); i++ {
		if args[i] == "--output" && i+1 < len(args) {
			outputName = args[i+1]
			i++
		}
	}

	// Check if model exists
	meta, err := registry.GetModel(modelID)
	if err != nil {
		printError(fmt.Sprintf("Model not found: %s", modelID))
		fmt.Println()
		modelList := registry.ListModels()
		if len(modelList) > 0 {
			fmt.Println("Available models:")
			for _, m := range modelList {
				fmt.Printf("  • %s\n", m.ID)
			}
			fmt.Println()
		}
		os.Exit(1)
	}

	// Validate quantization type
	validQuants := []string{"Q2_K", "Q3_K_S", "Q3_K_M", "Q3_K_L", "Q4_0", "Q4_1", "Q4_K_S", "Q4_K_M", "Q5_0", "Q5_1", "Q5_K_S", "Q5_K_M", "Q6_K", "Q8_0"}
	isValid := false
	for _, q := range validQuants {
		if targetQuant == q {
			isValid = true
			break
		}
	}
	if !isValid {
		printError(fmt.Sprintf("Invalid quantization type: %s", targetQuant))
		fmt.Printf("\n%sValid types:%s Q2_K, Q3_K_S, Q3_K_M, Q4_K_S, Q4_K_M, Q5_K_S, Q5_K_M, Q6_K, Q8_0\n\n", brandMuted, colorReset)
		os.Exit(1)
	}

	// Generate output filename
	if outputName == "" {
		// Remove extension and current quantization from model ID
		baseName := strings.TrimSuffix(modelID, filepath.Ext(modelID))
		baseName = strings.TrimSuffix(baseName, ".gguf")
		// Remove existing quantization suffix if present
		for _, q := range validQuants {
			if strings.HasSuffix(baseName, "."+q) {
				baseName = strings.TrimSuffix(baseName, "."+q)
				break
			}
			if strings.HasSuffix(baseName, "-"+q) {
				baseName = strings.TrimSuffix(baseName, "-"+q)
				break
			}
		}
		outputName = fmt.Sprintf("%s.%s", baseName, targetQuant)
	}
	outputPath := filepath.Join(cfg.ModelsDir, outputName+".gguf")

	// Check if output file already exists
	if _, err := os.Stat(outputPath); err == nil {
		printError(fmt.Sprintf("Output file already exists: %s", outputName+".gguf"))
		fmt.Printf("\n%sUse --output to specify a different name%s\n\n", brandMuted, colorReset)
		os.Exit(1)
	}

	// Print quantization header
	fmt.Printf("\n%s◆ Quantize Model%s\n", brandPrimary, colorReset)
	fmt.Printf("%s──────────────────────────────────────────────────%s\n\n", brandMuted, colorReset)

	fmt.Printf("%sSource Model%s\n", brandPrimary, colorReset)
	fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", brandMuted, colorReset)
	fmt.Printf("  Name:         %s%s%s\n", colorBold, modelID, colorReset)
	fmt.Printf("  Path:         %s%s%s\n", brandMuted, meta.Path, colorReset)
	fmt.Printf("  Size:         %s%s%s", brandPrimary, formatBytes(meta.Size), colorReset)
	if meta.Quantization != "" {
		fmt.Printf(" · %s%s%s", brandMuted, meta.Quantization, colorReset)
	}
	fmt.Println()
	fmt.Println()

	quantInfo := models.GetQuantizationInfo(targetQuant)
	fmt.Printf("%sTarget Quantization%s\n", brandPrimary, colorReset)
	fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", brandMuted, colorReset)
	fmt.Printf("  Type:         %s%s%s\n", brandPrimary, targetQuant, colorReset)
	fmt.Printf("  Bits:         %.1f bits per weight\n", quantInfo.BitsPerWeight)
	fmt.Printf("  Quality:      %s\n", quantInfo.Description)
	fmt.Printf("  Output:       %s%s.gguf%s\n", brandMuted, outputName, colorReset)
	fmt.Println()

	// Check if llama-quantize is available
	llamaQuantize := "llama-quantize"
	if _, err := exec.LookPath(llamaQuantize); err != nil {
		printError("llama-quantize tool not found")
		fmt.Printf("\n%sInstall llama.cpp first:%s\n", brandMuted, colorReset)
		fmt.Println("  cd /tmp && git clone https://github.com/ggerganov/llama.cpp")
		fmt.Println("  cd llama.cpp && make")
		fmt.Println("  sudo cp llama-quantize /usr/local/bin/")
		fmt.Println()
		os.Exit(1)
	}

	// Run quantization
	fmt.Printf("%s→%s Starting quantization...\n\n", brandPrimary, colorReset)

	cmd := exec.Command(llamaQuantize, meta.Path, outputPath, targetQuant)

	// Set LD_LIBRARY_PATH to include /usr/local/lib for llama.cpp shared libraries
	env := os.Environ()
	ldLibPath := "/usr/local/lib"
	if existingPath := os.Getenv("LD_LIBRARY_PATH"); existingPath != "" {
		ldLibPath = ldLibPath + ":" + existingPath
	}
	env = append(env, "LD_LIBRARY_PATH="+ldLibPath)
	cmd.Env = env

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	startTime := time.Now()
	if err := cmd.Run(); err != nil {
		fmt.Println()
		printError(fmt.Sprintf("Quantization failed: %v", err))
		os.Exit(1)
	}
	duration := time.Since(startTime)

	// Get output file size
	outputStat, err := os.Stat(outputPath)
	if err != nil {
		printError(fmt.Sprintf("Failed to stat output file: %v", err))
		os.Exit(1)
	}

	// Calculate compression ratio
	compressionRatio := float64(meta.Size) / float64(outputStat.Size())
	sizeSaved := meta.Size - outputStat.Size()

	// Display results
	fmt.Println()
	fmt.Printf("%s◆ Quantization Complete%s\n", brandSuccess, colorReset)
	fmt.Printf("%s──────────────────────────────────────────────────%s\n\n", brandMuted, colorReset)

	fmt.Printf("%sResults%s\n", brandSuccess, colorReset)
	fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", brandMuted, colorReset)
	fmt.Printf("  Original:     %s\n", formatBytes(meta.Size))
	fmt.Printf("  Quantized:    %s%s%s\n", brandSuccess, formatBytes(outputStat.Size()), colorReset)
	fmt.Printf("  Saved:        %s%s%s (%.1fx smaller)\n", brandPrimary, formatBytes(sizeSaved), colorReset, compressionRatio)
	fmt.Printf("  Time:         %s\n", formatDuration(duration))
	fmt.Println()

	fmt.Printf("%sModel Ready%s\n", brandSuccess, colorReset)
	fmt.Printf("  Name:         %s%s%s\n", brandPrimary, outputName, colorReset)
	fmt.Printf("  Location:     %s%s%s\n", brandMuted, outputPath, colorReset)
	fmt.Println()

	fmt.Printf("%s◆ Next Steps%s\n", brandPrimary, colorReset)
	fmt.Printf("%s──────────────────────────────────────────────────%s\n", brandMuted, colorReset)
	fmt.Printf("  Test model:   %soffgrid run %s%s\n", brandMuted, outputName, colorReset)
	fmt.Printf("  Benchmark:    %soffgrid benchmark %s%s\n", brandMuted, outputName, colorReset)
	fmt.Println()
}

func printHelp() {
	fmt.Println()
	fmt.Printf("%s┌─ OffGrid LLM%s\n", brandPrimary+colorBold, colorReset)
	fmt.Println("│")
	fmt.Printf("│ %sUsage:%s offgrid [command] [options]\n", colorDim, colorReset)
	fmt.Println()

	// Model Management
	fmt.Printf("%s├─ Model Management%s\n", brandPrimary+colorBold, colorReset)
	fmt.Printf("│  %sauto-select%s          Auto-detect hardware and recommend models\n", brandSecondary, colorReset)
	fmt.Printf("│  %slist%s                List installed models\n", brandSecondary, colorReset)
	fmt.Printf("│  %scatalog%s             Browse available models\n", brandSecondary, colorReset)
	fmt.Printf("│  %ssearch%s <query>      Search HuggingFace\n", brandSecondary, colorReset)
	fmt.Printf("│  %sdownload%s <id>       Download from catalog\n", brandSecondary, colorReset)
	fmt.Printf("│  %sdownload-hf%s <id>    Download from HuggingFace\n", brandSecondary, colorReset)
	fmt.Printf("│  %simport%s <path>       Import from USB/SD card\n", brandSecondary, colorReset)
	fmt.Printf("│  %sexport%s <id> <dst>   Export to USB/SD card\n", brandSecondary, colorReset)
	fmt.Printf("│  %sremove%s <id>         Remove installed model\n", brandSecondary, colorReset)
	fmt.Printf("│  %squantize%s <id> <q>   Quantize model to reduce size\n", brandSecondary, colorReset)
	fmt.Println("│")

	// Inference & Chat
	fmt.Printf("%s├─ Inference & Chat%s\n", brandPrimary+colorBold, colorReset)
	fmt.Printf("│  %sserve%s               Start API server (default)\n", brandSecondary, colorReset)
	fmt.Printf("│  %srun%s <model>         Interactive chat session\n", brandSecondary, colorReset)
	fmt.Printf("│  %ssession%s <cmd>       Manage chat sessions\n", brandSecondary, colorReset)
	fmt.Printf("│  %stemplate%s <cmd>      Manage prompt templates\n", brandSecondary, colorReset)
	fmt.Printf("│  %sbatch%s <file>        Batch process prompts\n", brandSecondary, colorReset)
	fmt.Println("│")

	// Configuration
	fmt.Printf("%s├─ Configuration & Tools%s\n", brandPrimary+colorBold, colorReset)
	fmt.Printf("│  %sversion%s             Show version information\n", brandSecondary, colorReset)
	fmt.Printf("│  %sinfo%s                System information\n", brandSecondary, colorReset)
	fmt.Printf("│  %sconfig%s <action>     Manage configuration\n", brandSecondary, colorReset)
	fmt.Printf("│  %squantization%s        Quantization guide\n", brandSecondary, colorReset)
	fmt.Printf("│  %salias%s <cmd>         Model aliases\n", brandSecondary, colorReset)
	fmt.Printf("│  %sfavorite%s <cmd>      Favorite models\n", brandSecondary, colorReset)
	fmt.Printf("│  %sbenchmark%s <id>      Performance testing\n", brandSecondary, colorReset)
	fmt.Printf("│  %scompletions%s <shell>  Shell completions\n", brandSecondary, colorReset)
	fmt.Printf("│  %shelp%s                Show this help\n", brandSecondary, colorReset)
	fmt.Println()

	fmt.Printf("%s└─ Examples%s\n", brandPrimary+colorBold, colorReset)
	fmt.Printf("   %soffgrid search llama --author TheBloke%s\n", colorDim, colorReset)
	fmt.Printf("   %soffgrid download tinyllama-1.1b-chat Q4_K_M%s\n", colorDim, colorReset)
	fmt.Printf("   %soffgrid download-hf TheBloke/Llama-2-7B-Chat-GGUF%s\n", colorDim, colorReset)
	fmt.Printf("   %soffgrid run tinyllama-1.1b-chat.Q4_K_M --save project%s\n", colorDim, colorReset)
	fmt.Printf("   %soffgrid import /media/usb%s\n", colorDim, colorReset)
	fmt.Printf("   %soffgrid session list%s\n", colorDim, colorReset)
	fmt.Println()
}

func handleVersion() {
	if output.JSONMode {
		output.PrintJSON(map[string]interface{}{
			"version": Version,
			"go":      runtime.Version(),
			"os":      runtime.GOOS,
			"arch":    runtime.GOARCH,
		})
		return
	}

	fmt.Println()
	fmt.Printf("%s┌─ OffGrid LLM%s\n", brandPrimary+colorBold, colorReset)
	fmt.Println("│")
	fmt.Printf("│ %sVersion:%s     %s\n", colorDim, colorReset, Version)
	fmt.Printf("│ %sGo:%s         %s\n", colorDim, colorReset, runtime.Version())
	fmt.Printf("│ %sPlatform:%s   %s/%s\n", colorDim, colorReset, runtime.GOOS, runtime.GOARCH)
	fmt.Println()
}

func handleInfo() {
	cfg := config.LoadConfig()
	registry := models.NewRegistry(cfg.ModelsDir)

	if err := registry.ScanModels(); err != nil {
		if !output.JSONMode {
			fmt.Fprintf(os.Stderr, "  ✗ Model scan error: %v\n", err)
		}
	}

	modelList := registry.ListModels()

	// JSON output mode
	if output.JSONMode {
		var cpuInfo string
		var memInfo string
		var gpuInfo string
		var osInfo string
		var archInfo string

		// Simple system info gathering
		if runtime.GOOS != "" {
			osInfo = runtime.GOOS
			archInfo = runtime.GOARCH
		}

		// Get CPU count
		cpuInfo = fmt.Sprintf("%d cores", runtime.NumCPU())

		// Try to get memory info
		if memStat, err := os.ReadFile("/proc/meminfo"); err == nil {
			lines := strings.Split(string(memStat), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "MemTotal:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						memInfo = fields[1] + " kB"
						break
					}
				}
			}
		}

		sysInfo := output.SystemInfo{
			CPU:          cpuInfo,
			Memory:       memInfo,
			GPU:          gpuInfo,
			OS:           osInfo,
			Architecture: archInfo,
		}

		var jsonModels []output.ModelInfo
		for _, model := range modelList {
			meta, _ := registry.GetModel(model.ID)
			modelInfo := output.ModelInfo{
				Name: model.ID,
			}
			if meta.Size > 0 {
				modelInfo.Size = formatBytes(meta.Size)
			}
			if meta.Quantization != "" && meta.Quantization != "unknown" {
				modelInfo.Quantization = meta.Quantization
			}
			if meta.Path != "" {
				modelInfo.Path = meta.Path
			}
			jsonModels = append(jsonModels, modelInfo)
		}

		output.PrintJSON(map[string]interface{}{
			"version": "0.1.0-alpha",
			"config": map[string]interface{}{
				"port":        cfg.ServerPort,
				"models_dir":  cfg.ModelsDir,
				"max_context": cfg.MaxContextSize,
				"threads":     cfg.NumThreads,
				"max_memory":  cfg.MaxMemoryMB,
				"p2p_enabled": cfg.EnableP2P,
			},
			"models": map[string]interface{}{
				"installed": jsonModels,
				"count":     len(jsonModels),
			},
			"system": sysInfo,
		})
		return
	}

	// Human-readable output
	fmt.Println()
	fmt.Printf("%s┌─ OffGrid LLM %sv0.1.0-alpha%s\n", brandPrimary+colorBold, colorDim, colorReset)
	fmt.Println("│")

	// Configuration
	fmt.Printf("│ %sConfiguration%s\n", colorBold, colorReset)
	fmt.Printf("│ %sPort:%s %d  %s│%s  %sModels:%s %s\n",
		colorDim, colorReset, cfg.ServerPort,
		colorDim, colorReset,
		colorDim, colorReset, cfg.ModelsDir)
	fmt.Printf("│ %sThreads:%s %d  %s│%s  %sContext:%s %d tokens  %s│%s  %sMemory:%s %d MB\n",
		colorDim, colorReset, cfg.NumThreads,
		colorDim, colorReset,
		colorDim, colorReset, cfg.MaxContextSize,
		colorDim, colorReset,
		colorDim, colorReset, cfg.MaxMemoryMB)
	if cfg.EnableP2P {
		fmt.Printf("│ %sP2P:%s enabled (port %d)\n", colorDim, colorReset, cfg.P2PPort)
	}
	fmt.Println("│")

	// Installed Models
	var totalSize int64
	if len(modelList) == 1 {
		fmt.Printf("│ %sInstalled Models%s · 1 model\n", colorBold, colorReset)
	} else {
		fmt.Printf("│ %sInstalled Models%s · %d models\n", colorBold, colorReset, len(modelList))
	}

	if len(modelList) > 0 {
		for _, model := range modelList {
			meta, err := registry.GetModel(model.ID)
			if err == nil {
				statusIcon := "○"
				statusColor := colorDim
				if meta.IsLoaded {
					statusIcon = "●"
					statusColor = brandSuccess
				}

				fmt.Printf("│ %s%s%s %s", statusColor, statusIcon, colorReset, model.ID)
				if meta.Size > 0 {
					fmt.Printf(" %s·%s %s", colorDim, colorReset, formatBytes(meta.Size))
					totalSize += meta.Size
				}
				if meta.Quantization != "" && meta.Quantization != "unknown" {
					fmt.Printf(" %s·%s %s", colorDim, colorReset, meta.Quantization)
				}
				fmt.Println()
			}
		}
		if totalSize > 0 {
			fmt.Printf("│ %sTotal:%s %s\n", colorDim, colorReset, formatBytes(totalSize))
		}
	} else {
		fmt.Printf("│ No models installed\n")
	}
	fmt.Println("│")

	// Catalog info
	catalog := models.DefaultCatalog()
	recommended := 0
	for _, entry := range catalog.Models {
		if entry.Recommended {
			recommended++
		}
	}
	fmt.Printf("│ %sCatalog%s · %d models (%d recommended)\n",
		colorBold, colorReset, len(catalog.Models), recommended)
	fmt.Println()

	// Commands
	if len(modelList) == 0 {
		fmt.Printf("%s└─ Quick Start%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("   %soffgrid search llama%s         Search for models\n", brandSecondary, colorReset)
		fmt.Printf("   %soffgrid download <id>%s        Download from catalog\n", brandSecondary, colorReset)
		fmt.Printf("   %soffgrid serve%s                Start API server\n", brandSecondary, colorReset)
	} else {
		fmt.Printf("%s└─ Server%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("   %soffgrid serve%s                Start server\n", brandSecondary, colorReset)
		fmt.Printf("   %shttp://localhost:%d%s      API endpoint\n", brandSecondary, cfg.ServerPort, colorReset)
		fmt.Printf("   %shttp://localhost:%d/health%s  Health check\n", brandSecondary, cfg.ServerPort, colorReset)
	}
	fmt.Println()
}

func handleConfig(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: offgrid config <action>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Actions:")
		fmt.Fprintln(os.Stderr, "  init [path]      Create a new config file (YAML/JSON)")
		fmt.Fprintln(os.Stderr, "  show             Display current configuration")
		fmt.Fprintln(os.Stderr, "  validate [path]  Validate a config file")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  offgrid config init                    # Create ~/.offgrid-llm/config.yaml")
		fmt.Fprintln(os.Stderr, "  offgrid config init custom.json        # Create custom.json")
		fmt.Fprintln(os.Stderr, "  offgrid config show                    # Show current config")
		fmt.Fprintln(os.Stderr, "  offgrid config validate config.yaml    # Validate config")
		os.Exit(1)
	}

	action := args[0]

	switch action {
	case "init":
		// Determine output path
		outputPath := ""
		if len(args) > 1 {
			outputPath = args[1]
		} else {
			homeDir, _ := os.UserHomeDir()
			configDir := filepath.Join(homeDir, ".offgrid-llm")
			os.MkdirAll(configDir, 0755)
			outputPath = filepath.Join(configDir, "config.yaml")
		}

		// Create default config
		cfg := config.LoadConfig()

		// Save to file
		if err := cfg.SaveToFile(outputPath); err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ Failed to create config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("  ✓ Created config: %s\n\n", outputPath)
		fmt.Println("To use:")
		fmt.Printf("  export OFFGRID_CONFIG=%s\n", outputPath)
		fmt.Println("  offgrid")
		fmt.Println()

	case "show":
		configPath := os.Getenv("OFFGRID_CONFIG")
		cfg, err := config.LoadWithPriority(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ Failed to load config: %v\n", err)
			os.Exit(1)
		}

		fmt.Println()
		fmt.Println("Configuration")
		fmt.Println()
		fmt.Println("Server")
		fmt.Printf("  Host:             %s\n", cfg.ServerHost)
		fmt.Printf("  Port:             %d\n", cfg.ServerPort)
		fmt.Println()
		fmt.Println("Models")
		fmt.Printf("  Directory:        %s\n", cfg.ModelsDir)
		fmt.Printf("  Default:          %s\n", cfg.DefaultModel)
		fmt.Printf("  Max context:      %d\n", cfg.MaxContextSize)
		fmt.Printf("  Threads:          %d\n", cfg.NumThreads)
		fmt.Println()
		fmt.Println("Resources")
		fmt.Printf("  Max memory:       %d MB\n", cfg.MaxMemoryMB)
		fmt.Printf("  Max models:       %d\n", cfg.MaxModels)
		fmt.Printf("  GPU:              %v\n", cfg.EnableGPU)
		if cfg.EnableGPU {
			fmt.Printf("  GPU layers:       %d\n", cfg.NumGPULayers)
		}
		fmt.Println()
		if cfg.EnableP2P {
			fmt.Println("P2P")
			fmt.Printf("  Enabled:          %v\n", cfg.EnableP2P)
			fmt.Printf("  Port:             %d\n", cfg.P2PPort)
			fmt.Printf("  Discovery:        %d\n", cfg.DiscoveryPort)
			fmt.Println()
		}
		fmt.Println("Logging")
		fmt.Printf("  Level:            %s\n", cfg.LogLevel)
		if cfg.LogFile != "" {
			fmt.Printf("  File:             %s\n", cfg.LogFile)
		}
		fmt.Println()

	case "validate":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: offgrid config validate <path>")
			os.Exit(1)
		}

		configPath := args[1]
		cfg, err := config.LoadFromFile(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ Invalid config: %v\n", err)
			os.Exit(1)
		}

		if err := cfg.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ Validation failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("  ✓ Config valid: %s\n", configPath)
		fmt.Println()

	default:
		fmt.Fprintf(os.Stderr, "Unknown action: %s\n", action)
		os.Exit(1)
	}
}

func handleSearch(args []string) {
	// Parse search query and filters
	var query string
	var filters models.SearchFilter

	// Default filters
	filters.OnlyGGUF = true
	filters.ExcludeGated = true
	filters.Limit = 20
	filters.SortBy = "downloads"

	// Parse arguments
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--author" || arg == "-a":
			if i+1 < len(args) {
				filters.Author = args[i+1]
				i++
			}
		case arg == "--quant" || arg == "-q":
			if i+1 < len(args) {
				filters.Quantization = args[i+1]
				i++
			}
		case arg == "--sort" || arg == "-s":
			if i+1 < len(args) {
				filters.SortBy = args[i+1]
				i++
			}
		case arg == "--limit" || arg == "-l":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &filters.Limit)
				i++
			}
		case arg == "--all":
			filters.ExcludeGated = false
		case arg == "--help" || arg == "-h":
			fmt.Println()
			fmt.Printf("%s┌─ Search HuggingFace%s\n", brandPrimary+colorBold, colorReset)
			fmt.Println("│")
			fmt.Printf("│ %sUsage:%s offgrid search [query] [options]\n", colorDim, colorReset)
			fmt.Println("│")
			fmt.Printf("%s├─ Options%s\n", brandPrimary+colorBold, colorReset)
			fmt.Printf("│  %s-a, --author <name>%s   Filter by author (e.g., 'TheBloke')\n", brandSecondary, colorReset)
			fmt.Printf("│  %s-q, --quant <type>%s    Filter by quantization (e.g., 'Q4_K_M')\n", brandSecondary, colorReset)
			fmt.Printf("│  %s-s, --sort <field>%s    Sort by: downloads, likes, created, modified\n", brandSecondary, colorReset)
			fmt.Printf("│  %s-l, --limit <n>%s       Limit results (default: 20)\n", brandSecondary, colorReset)
			fmt.Printf("│  %s--all%s                Include gated models\n", brandSecondary, colorReset)
			fmt.Println()
			fmt.Printf("%s└─ Examples%s\n", brandPrimary+colorBold, colorReset)
			fmt.Printf("   %soffgrid search llama%s\n", colorDim, colorReset)
			fmt.Printf("   %soffgrid search mistral --author TheBloke --quant Q4_K_M%s\n", colorDim, colorReset)
			fmt.Printf("   %soffgrid search --sort likes --limit 10%s\n", colorDim, colorReset)
			fmt.Println()
			return
		default:
			if query == "" {
				query = arg
			}
		}
	}

	filters.Query = query

	if !output.JSONMode {
		fmt.Printf("\n%s%s%s Searching HuggingFace Hub%s\n", brandPrimary, iconSearch, colorBold, colorReset)
		printDivider()
		fmt.Println()
	}

	hf := models.NewHuggingFaceClient()
	results, err := hf.SearchModels(filters)
	if err != nil {
		if output.JSONMode {
			output.Error("Search failed", err)
		}
		printError(fmt.Sprintf("Search failed: %v", err))
		fmt.Println()
		os.Exit(1)
	}

	// JSON output mode
	if output.JSONMode {
		var jsonResults []output.SearchResult
		for _, result := range results {
			searchResult := output.SearchResult{
				Name:      result.Model.ID,
				ModelID:   result.Model.ID,
				Downloads: int(result.Model.Downloads),
				Likes:     result.Model.Likes,
			}
			if len(result.Model.Tags) > 0 {
				searchResult.Tags = result.Model.Tags
			}
			jsonResults = append(jsonResults, searchResult)
		}
		output.PrintSearchResults(jsonResults)
		return
	}

	// Human-readable output
	if len(results) == 0 {
		fmt.Println()
		printWarning("No models found matching your criteria")
		fmt.Println()
		printInfo("Try broadening your search or adjusting filters")
		fmt.Println()
		return
	}

	fmt.Println()
	fmt.Printf("Found %s%d%s model(s)\n", brandPrimary, len(results), colorReset)
	printDivider()
	fmt.Println()

	for i, result := range results {
		model := result.Model

		// Model name with number
		fmt.Printf("%s%2d.%s %s%s%s %s\n",
			brandMuted, i+1, colorReset,
			brandPrimary, iconModel, colorReset,
			colorBold+model.ID+colorReset)

		// Stats line with colors
		fmt.Printf("     %s%s%s %s",
			brandAccent, iconDownload, colorReset,
			formatNumber(model.Downloads))
		fmt.Printf("  %s❤%s %s",
			brandError, colorReset,
			formatNumber(int64(model.Likes)))

		// Recommended variant with color
		if result.BestVariant != nil && result.BestVariant.SizeGB > 0 {
			fmt.Printf("  %s│%s %s%s%s (%.1f GB)",
				brandMuted, colorReset,
				brandSuccess, result.BestVariant.Quantization, colorReset,
				result.BestVariant.SizeGB)
		}
		fmt.Println()

		// Available variants
		if len(result.GGUFFiles) > 0 {
			fmt.Printf("     %sVariants:%s ", brandMuted, colorReset)
			shown := 0
			for _, file := range result.GGUFFiles {
				if shown >= 6 {
					fmt.Printf("%s(+%d more)%s", brandMuted, len(result.GGUFFiles)-shown, colorReset)
					break
				}
				if shown > 0 {
					fmt.Printf("%s, %s", brandMuted, colorReset)
				}
				fmt.Printf("%s", file.Quantization)
				shown++
			}
			fmt.Println()
		}

		// Download command with color
		if result.BestVariant != nil {
			fmt.Printf("     %s→%s %soffgrid download-hf %s --file %s%s\n",
				brandPrimary, colorReset,
				brandMuted, model.ID, result.BestVariant.Filename, colorReset)
		}

		if i < len(results)-1 {
			fmt.Println()
		}
	}

	fmt.Println()
	printDivider()
	fmt.Println()
	fmt.Println()
}

func handleDownloadHF(args []string) {
	// Check for help flag
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = []string{} // Trigger help display
	}

	if len(args) < 1 {
		printDivider()
		fmt.Println()
		printSection("Usage")
		fmt.Printf("  %soffgrid download-hf%s <model-id> [options]\n", colorBold, colorReset)
		fmt.Println()
		printSection("Options")
		printItem("--file <filename>", "Specific GGUF file to download")
		printItem("--quant <type>", "Filter by quantization (e.g., Q4_K_M)")
		fmt.Println()
		printSection("Examples")
		fmt.Printf("  %s$%s offgrid download-hf TheBloke/Llama-2-7B-Chat-GGUF --file llama-2-7b-chat.Q4_K_M.gguf\n", brandMuted, colorReset)
		fmt.Printf("  %s$%s offgrid download-hf TheBloke/Mistral-7B-Instruct-v0.2-GGUF --quant Q4_K_M\n", brandMuted, colorReset)
		fmt.Println()
		printInfo("Use 'offgrid search <query>' to find models first")
		fmt.Println()
		printDivider()
		fmt.Println()
		os.Exit(1)
	}

	modelID := args[0]
	var filename string
	var quantFilter string

	// Parse options
	for i := 1; i < len(args); i++ {
		if args[i] == "--file" && i+1 < len(args) {
			filename = args[i+1]
			i++
		} else if args[i] == "--quant" && i+1 < len(args) {
			quantFilter = args[i+1]
			i++
		}
	}

	cfg := config.LoadConfig()
	hf := models.NewHuggingFaceClient()

	fmt.Println()
	fmt.Printf("%s◆ Download from HuggingFace%s\n", brandPrimary, colorReset)
	fmt.Printf("%s──────────────────────────────────────────────────%s\n\n", brandMuted, colorReset)
	fmt.Printf("%s→%s Fetching model info: %s%s%s\n", brandPrimary, colorReset, colorBold, modelID, colorReset)
	fmt.Println()

	model, err := hf.GetModelInfo(modelID)
	if err != nil {
		printError(fmt.Sprintf("Failed to fetch model: %v", err))
		fmt.Println()
		printInfo("Make sure:")
		fmt.Println("  • The model ID is correct")
		fmt.Println("  • You have internet connectivity")
		fmt.Println()
		os.Exit(1)
	}

	// Parse GGUF files
	ggufFiles := []models.GGUFFileInfo{}
	for _, sibling := range model.Siblings {
		if !strings.HasSuffix(strings.ToLower(sibling.Filename), ".gguf") {
			continue
		}

		info := models.GGUFFileInfo{
			Filename:     sibling.Filename,
			Size:         sibling.Size,
			SizeGB:       float64(sibling.Size) / (1024 * 1024 * 1024),
			Quantization: extractQuantFromFilename(sibling.Filename),
			DownloadURL:  fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", modelID, sibling.Filename),
		}

		// Apply filters
		if filename != "" && sibling.Filename != filename {
			continue
		}
		if quantFilter != "" && !strings.EqualFold(info.Quantization, quantFilter) {
			continue
		}

		ggufFiles = append(ggufFiles, info)
	}

	if len(ggufFiles) == 0 {
		printError("No matching GGUF files found")
		fmt.Println()
		if quantFilter != "" {
			fmt.Printf("No files with quantization '%s%s%s' found.\n", brandPrimary, quantFilter, colorReset)
			fmt.Println()
			printInfo("Try without --quant filter or use 'offgrid search' to see available quantizations")
		} else {
			printInfo("This model may not have GGUF format files.")
			fmt.Println()
			printInfo("Search for GGUF models:")
			fmt.Printf("  %soffgrid search <query> --author TheBloke%s\n", brandMuted, colorReset)
		}
		fmt.Println()
		os.Exit(1)
	}

	// If multiple files, let user choose
	var selectedFile models.GGUFFileInfo
	if len(ggufFiles) == 1 {
		selectedFile = ggufFiles[0]
	} else {
		fmt.Printf("%sAvailable Files%s\n", brandPrimary, colorReset)
		fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", brandMuted, colorReset)
		for i, file := range ggufFiles {
			fmt.Printf("  %s%d.%s %s (%s%s%s)\n",
				brandMuted, i+1, colorReset,
				file.Filename,
				brandPrimary, file.Quantization, colorReset)
		}
		fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", brandMuted, colorReset)
		fmt.Printf("\n%sSelect file%s (1-%d): ", brandMuted, colorReset, len(ggufFiles))

		var choice int
		fmt.Scanf("%d", &choice)
		if choice < 1 || choice > len(ggufFiles) {
			fmt.Println()
			printError("Invalid choice")
			fmt.Println()
			os.Exit(1)
		}
		selectedFile = ggufFiles[choice-1]
	}

	// Create destination path
	destPath := filepath.Join(cfg.ModelsDir, selectedFile.Filename)

	// Check if file already exists
	if _, err := os.Stat(destPath); err == nil {
		fmt.Println()
		printWarning("Model already exists")
		fmt.Println()
		fmt.Printf("%sExisting Model%s\n", brandPrimary, colorReset)
		fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", brandMuted, colorReset)
		fmt.Printf("  Name:     %s%s%s\n", colorBold, selectedFile.Filename, colorReset)
		fmt.Printf("  Location: %s%s%s\n", brandMuted, destPath, colorReset)

		// Get existing file size
		if fileInfo, err := os.Stat(destPath); err == nil {
			fmt.Printf("  Size:     %s\n", formatBytes(fileInfo.Size()))
		}
		fmt.Println()
		fmt.Printf("%sRedownload and replace?%s (y/N): ", brandMuted, colorReset)

		var response string
		fmt.Scanln(&response)

		if response != "y" && response != "Y" {
			fmt.Println()
			printInfo("Download cancelled - existing model preserved")
			fmt.Println()
			return
		}
		fmt.Println()
	}

	fmt.Println()
	fmt.Printf("%sDownloading%s\n", brandPrimary, colorReset)
	fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", brandMuted, colorReset)
	fmt.Printf("  File:         %s%s%s\n", colorBold, selectedFile.Filename, colorReset)
	if selectedFile.SizeGB > 0 {
		fmt.Printf("  Size:         %.1f GB\n", selectedFile.SizeGB)
	}
	fmt.Printf("  Quantization: %s%s%s\n", brandPrimary, selectedFile.Quantization, colorReset)
	fmt.Println()

	// Download with progress
	startTime := time.Now()
	lastUpdate := time.Now()
	var lastProgress int64

	err = hf.DownloadGGUF(modelID, selectedFile.Filename, destPath, func(current, total int64) {
		percent := float64(current) / float64(total) * 100

		// Calculate speed from bytes downloaded since last update
		now := time.Now()
		elapsed := now.Sub(lastUpdate).Seconds()

		var speed float64
		if elapsed > 0.5 { // Update speed every half second
			bytesThisInterval := current - lastProgress
			speed = float64(bytesThisInterval) / elapsed / (1024 * 1024) // MB/s
			lastUpdate = now
			lastProgress = current
		} else if lastProgress == 0 {
			// First update - use overall speed
			totalElapsed := now.Sub(startTime).Seconds()
			if totalElapsed > 0.5 {
				speed = float64(current) / totalElapsed / (1024 * 1024)
			}
		} else {
			// Keep previous speed if interval too short
			return // Don't update display yet
		}

		// Format size appropriately (MB for small files, GB for large)
		var currentStr, totalStr string
		if total < 1024*1024*1024 { // Less than 1 GB, show in MB
			currentStr = fmt.Sprintf("%.1f MB", float64(current)/(1024*1024))
			totalStr = fmt.Sprintf("%.1f MB", float64(total)/(1024*1024))
		} else {
			currentStr = fmt.Sprintf("%.1f GB", float64(current)/(1024*1024*1024))
			totalStr = fmt.Sprintf("%.1f GB", float64(total)/(1024*1024*1024))
		}

		fmt.Printf("\r  %s→%s Progress: %.1f%% (%s / %s) · %.1f MB/s  ",
			brandPrimary, colorReset,
			percent,
			currentStr,
			totalStr,
			speed)
	})

	if err != nil {
		fmt.Println()
		printError(fmt.Sprintf("Download failed: %v", err))
		fmt.Println()
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println()
	fmt.Printf("%s◆ Download Complete%s\n", brandSuccess, colorReset)
	fmt.Printf("%s──────────────────────────────────────────────────%s\n\n", brandMuted, colorReset)
	fmt.Printf("%sModel Ready%s\n", brandSuccess, colorReset)
	fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", brandMuted, colorReset)
	fmt.Printf("  Name:     %s%s%s\n", brandPrimary, selectedFile.Filename, colorReset)
	fmt.Printf("  Location: %s%s%s\n", brandMuted, destPath, colorReset)
	fmt.Println()

	// Reload llama-server with the downloaded model
	if err := reloadLlamaServerWithModel(destPath); err != nil {
		printWarning(fmt.Sprintf("Could not auto-reload server: %v", err))
		fmt.Println()
		printInfo("Manually restart the server:")
		fmt.Printf("  %ssudo systemctl restart llama-server%s\n", brandMuted, colorReset)
		fmt.Println()
	}

	// Extract model name (filename without .gguf extension for CLI)
	modelName := selectedFile.Filename
	if strings.HasSuffix(modelName, ".gguf") {
		modelName = modelName[:len(modelName)-5]
	}

	fmt.Printf("%s◆ Next Steps%s\n", brandPrimary, colorReset)
	fmt.Printf("%s──────────────────────────────────────────────────%s\n", brandMuted, colorReset)
	fmt.Printf("  Run model:    %soffgrid run %s%s\n", brandMuted, modelName, colorReset)
	fmt.Printf("  Benchmark:    %soffgrid benchmark %s%s\n", brandMuted, modelName, colorReset)
	fmt.Println()
}

func extractQuantFromFilename(filename string) string {
	patterns := []string{
		"Q2_K", "Q3_K_S", "Q3_K_M", "Q3_K_L",
		"Q4_0", "Q4_1", "Q4_K_S", "Q4_K_M",
		"Q5_0", "Q5_1", "Q5_K_S", "Q5_K_M",
		"Q6_K", "Q8_0", "F16", "F32",
	}

	upper := strings.ToUpper(filename)
	for _, pattern := range patterns {
		if strings.Contains(upper, pattern) {
			return pattern
		}
	}

	return "unknown"
}

func handleRun(args []string) {
	// Check for help flag first
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = []string{} // Trigger help display
	}

	if len(args) < 1 {
		printDivider()
		fmt.Println()
		printSection("Usage")
		fmt.Printf("  %soffgrid run%s <model-name> [--save <name>] [--load <name>]\n", colorBold, colorReset)
		fmt.Println()
		printSection("Description")
		fmt.Println("  Start an interactive chat session with a model")
		fmt.Println()
		printSection("Options")
		fmt.Printf("  %s--save <name>%s    Save conversation to session\n", brandPrimary, colorReset)
		fmt.Printf("  %s--load <name>%s    Load and continue existing session\n", brandPrimary, colorReset)
		fmt.Println()
		printSection("Examples")
		fmt.Printf("  %s$%s offgrid run tinyllama-1.1b-chat.Q4_K_M\n", brandMuted, colorReset)
		fmt.Printf("  %s$%s offgrid run llama --save my-project\n", brandMuted, colorReset)
		fmt.Printf("  %s$%s offgrid run llama --load my-project\n", brandMuted, colorReset)
		fmt.Println()
		printInfo("Use 'offgrid list' to see available models")
		printInfo("Use 'offgrid session list' to see saved sessions")
		fmt.Println()
		printDivider()
		fmt.Println()
		os.Exit(1)
	}

	modelName := args[0]
	var sessionName string
	var loadSession bool
	var saveSession bool

	// Parse flags
	for i := 1; i < len(args); i++ {
		if args[i] == "--save" && i+1 < len(args) {
			sessionName = args[i+1]
			saveSession = true
			i++
		} else if args[i] == "--load" && i+1 < len(args) {
			sessionName = args[i+1]
			loadSession = true
			i++
		}
	}

	// Strip .gguf extension if present (for user convenience)
	if strings.HasSuffix(strings.ToLower(modelName), ".gguf") {
		modelName = modelName[:len(modelName)-5]
	}

	cfg := config.LoadConfig()

	// Check if model exists locally (for validation only)
	registry := models.NewRegistry(cfg.ModelsDir)
	if err := registry.ScanModels(); err != nil {
		fmt.Fprintf(os.Stderr, "\n✗ Error: Failed to scan models directory\n")
		fmt.Fprintf(os.Stderr, "  %v\n\n", err)
		os.Exit(1)
	}

	// Try to find the model (for validation)
	_, err := registry.GetModel(modelName)
	if err != nil {
		fmt.Println()
		printError(fmt.Sprintf("Model not found: %s", modelName))
		fmt.Println()

		// Show available models
		availableModels := registry.ListModels()
		if len(availableModels) > 0 {
			printSection("Available Models")
			for _, model := range availableModels {
				fmt.Printf("  %s◭%s %s\n", brandSecondary, colorReset, model.ID)
			}
		} else {
			printSection("Get Started")
			printItem("Search models", "offgrid search llama --author TheBloke")
			printItem("Download model", "offgrid download-hf <model-id> --quant Q4_K_M")
		}
		fmt.Println()
		os.Exit(1)
	}

	// Check if this is an embedding model (not designed for chat)
	isEmbeddingModel := strings.Contains(strings.ToLower(modelName), "minilm") ||
		strings.Contains(strings.ToLower(modelName), "e5-") ||
		strings.Contains(strings.ToLower(modelName), "bge-") ||
		strings.Contains(strings.ToLower(modelName), "gte-") ||
		strings.Contains(strings.ToLower(modelName), "embedding")

	if isEmbeddingModel {
		fmt.Println()
		printWarning("This appears to be an embedding model, not a chat model")
		fmt.Println()
		printInfo("Embedding models are designed for:")
		fmt.Println("  • Converting text to vectors")
		fmt.Println("  • Semantic search")
		fmt.Println("  • Text similarity")
		fmt.Println()
		printInfo("For chat/text generation, use a language model instead:")
		fmt.Println("  • tinyllama-1.1b-chat")
		fmt.Println("  • phi-2")
		fmt.Println("  • llama-2-7b-chat")
		fmt.Println("  • mistral-7b-instruct")
		fmt.Println()
		fmt.Printf("%sContinue anyway?%s (y/N): ", brandMuted, colorReset)

		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println()
			printInfo("Aborted. Use 'offgrid list' to see available models")
			fmt.Println()
			os.Exit(0)
		}
		fmt.Println()
	}

	// Check if OffGrid API server is running and start it if needed
	if err := ensureOffgridServerRunning(); err != nil {
		fmt.Println()
		printWarning(fmt.Sprintf("OffGrid server not running: %v", err))
		printInfo("Starting OffGrid server in background...")
		if err := startOffgridServerInBackground(); err != nil {
			fmt.Println()
			printError("Failed to start OffGrid server")
			printInfo("Please start the server manually:")
			printItem("Start server", "offgrid serve &")
			fmt.Println()
			os.Exit(1)
		}
		// Wait for server to be ready
		time.Sleep(2 * time.Second)
	}

	// Get the full model path
	model, err := registry.GetModel(modelName)
	if err != nil {
		fmt.Println()
		printError(fmt.Sprintf("Failed to get model path: %v", err))
		fmt.Println()
		os.Exit(1)
	}

	// Check if llama-server is running and start it if needed
	if err := ensureLlamaServerRunning(); err != nil {
		fmt.Println()
		printInfo("Starting llama-server with model...")
		printInfo("Using optimized flags: --no-mmap --mlock -fa (faster loading)")
		fmt.Println()
		printWarning("TIP: For instant responses, keep llama-server running:")
		fmt.Printf("  %s$%s sudo systemctl enable --now llama-server\n", brandMuted, colorReset)
		fmt.Println()
		if err := startLlamaServerInBackground(model.Path); err != nil {
			fmt.Println()
			printError("Failed to start llama-server")
			printInfo("Please start llama-server manually:")
			printItem("Start llama-server", fmt.Sprintf("llama-server -m %s --port 8081 &", model.Path))
			fmt.Println()
			os.Exit(1)
		}
		// Wait for llama-server to load the model
		fmt.Println()
		printInfo("Loading model into RAM (first-time: 2-4 seconds)...")
		printInfo("With daemon mode enabled, this happens only once!")

		// Read llama-server port from config
		llamaPort := "48081"
		if portBytes, err := os.ReadFile("/etc/offgrid/llama-port"); err == nil {
			llamaPort = strings.TrimSpace(string(portBytes))
		}

		// Poll llama-server health endpoint until model is ready
		maxWait := 60 // seconds
		for i := 0; i < maxWait; i++ {
			time.Sleep(1 * time.Second)

			resp, err := http.Get(fmt.Sprintf("http://localhost:%s/health", llamaPort))
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == 200 {
					// Model is loaded and ready
					break
				}
			}

			if i == maxWait-1 {
				fmt.Println()
				printError("Timeout waiting for model to load")
				fmt.Println()
				os.Exit(1)
			}
		}
		fmt.Println()
		printInfo("Model loaded successfully!")
	}

	// Setup session management
	homeDir, _ := os.UserHomeDir()
	sessionsDir := filepath.Join(homeDir, ".offgrid", "sessions")
	sessionMgr := sessions.NewSessionManager(sessionsDir)

	var currentSession *sessions.Session

	// Load existing session or create new one
	if loadSession {
		sess, err := sessionMgr.Load(sessionName)
		if err != nil {
			printError(fmt.Sprintf("Failed to load session '%s': %v", sessionName, err))
			fmt.Println()
			printInfo("Available sessions:")
			sessionList, _ := sessionMgr.List()
			for _, s := range sessionList {
				fmt.Printf("  • %s\n", s.Name)
			}
			fmt.Println()
			os.Exit(1)
		}
		currentSession = sess
		printSuccess(fmt.Sprintf("Loaded session '%s' (%d messages)", sessionName, sess.MessageCount()))
		fmt.Println()

		// Display previous conversation
		if sess.MessageCount() > 0 {
			printInfo("Previous conversation:")
			fmt.Println()
			for _, msg := range sess.Messages {
				if msg.Role == "user" {
					fmt.Printf("  %sYou:%s %s\n", brandPrimary, colorReset, msg.Content)
				} else {
					fmt.Printf("  %sAssistant:%s %s\n", brandSuccess, colorReset, msg.Content)
				}
			}
			fmt.Println()
			printDivider()
			fmt.Println()
		}
	} else if saveSession {
		currentSession = sessions.NewSession(sessionName, modelName)
		printInfo(fmt.Sprintf("Starting new session '%s' (will auto-save)", sessionName))
		fmt.Println()
	}

	fmt.Println()
	fmt.Printf("%s┌─ Chat · %s%s\n", brandPrimary+colorBold, modelName, colorReset)
	fmt.Println("│")
	if currentSession != nil {
		fmt.Printf("│ %sSession:%s %s (auto-saving)\n", colorDim, colorReset, currentSession.Name)
	}
	fmt.Printf("│ Type %sexit%s to quit · %sclear%s to reset\n", brandSecondary, colorReset, brandSecondary, colorReset)
	fmt.Println()

	// Start chat session
	fmt.Print("Connecting...")

	// Import required packages for HTTP client
	client := &http.Client{
		Timeout: 300 * time.Second,
	}

	// Check if server is running
	healthURL := fmt.Sprintf("http://localhost:%d/health", cfg.ServerPort)
	resp, err := client.Get(healthURL)
	if err != nil {
		fmt.Printf(" %s✗%s\n", brandError, colorReset)
		fmt.Println()
		printError("Server not running")
		fmt.Println()
		printSection("Start Server")
		printItem("Direct start", "offgrid serve")
		printItem("System service", "sudo systemctl start offgrid-llm")
		fmt.Println()
		os.Exit(1)
	}
	resp.Body.Close()
	fmt.Printf(" %s✓%s\n", brandSuccess, colorReset)

	// Check which model is currently loaded in the server
	modelsURL := fmt.Sprintf("http://localhost:%d/v1/models", cfg.ServerPort)
	resp, err = client.Get(modelsURL)
	if err == nil {
		defer resp.Body.Close()
		var modelsResp struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err == nil {
			// Check if requested model is in the active models list
			foundActive := false
			var activeModel string
			for _, m := range modelsResp.Data {
				if m.ID == modelName {
					foundActive = true
					break
				}
				activeModel = m.ID
			}

			if !foundActive && len(modelsResp.Data) > 0 {
				fmt.Println()
				printWarning(fmt.Sprintf("Server is currently serving: %s", activeModel))
				fmt.Println()
				printInfo("To use a different model, restart llama-server:")
				fmt.Printf("  %s$%s sudo systemctl stop llama-server\n", brandMuted, colorReset)
				fmt.Printf("  %s$%s sudo systemctl start llama-server\n", brandMuted, colorReset)
				fmt.Println()
				printInfo("Or continue with the loaded model...")
				fmt.Println()
			}
		}
	}

	fmt.Println()

	// Interactive chat loop
	messages := []ChatMessage{}

	// Load messages from session if continuing
	if currentSession != nil && currentSession.MessageCount() > 0 {
		for _, msg := range currentSession.Messages {
			messages = append(messages, ChatMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("\n%s>%s ", brandPrimary, colorReset)
		input, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		input = strings.TrimSpace(input)

		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println()
			fmt.Printf("%sChat session ended%s\n\n", colorDim, colorReset)
			break
		}

		if input == "clear" {
			messages = []ChatMessage{}
			// Clear session too if active
			if currentSession != nil {
				currentSession.Messages = []sessions.Message{}
				if err := sessionMgr.Save(currentSession); err != nil {
					fmt.Printf("%sWarning: Failed to save cleared session: %v%s\n", colorYellow, err, colorReset)
				}
			}
			fmt.Printf("\n%sConversation cleared%s\n", colorDim, colorReset)
			continue
		}

		// Add user message
		messages = append(messages, ChatMessage{
			Role:    "user",
			Content: input,
		})

		// Save user message to session
		if currentSession != nil {
			currentSession.AddMessage("user", input)
		}

		// Make API request
		reqBody := ChatCompletionRequest{
			Model:    modelName,
			Messages: messages,
			Stream:   true,
		}

		jsonData, _ := json.Marshal(reqBody)
		apiURL := fmt.Sprintf("http://localhost:%d/v1/chat/completions", cfg.ServerPort)

		req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
		if err != nil {
			fmt.Println()
			fmt.Fprintf(os.Stderr, "✗ Error creating request: %v\n", err)
			fmt.Println()
			continue
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			fmt.Println()
			fmt.Fprintf(os.Stderr, "✗ Error: %v\n", err)
			fmt.Println()
			continue
		}

		// Handle streaming response
		fmt.Print("\n")
		scanner := bufio.NewScanner(resp.Body)
		var assistantMsg strings.Builder
		lineLength := 0

		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			var chunk ChatCompletionChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) > 0 {
				token := chunk.Choices[0].Delta.Content
				fmt.Print(token)
				assistantMsg.WriteString(token)
				lineLength += len(token)

				// Wrap long lines
				if lineLength > 80 && strings.Contains(token, " ") {
					lineLength = 0
				}
			}
		}

		fmt.Println()
		fmt.Println()

		resp.Body.Close()

		// Add assistant response to history
		messages = append(messages, ChatMessage{
			Role:    "assistant",
			Content: assistantMsg.String(),
		})

		// Save assistant message to session
		if currentSession != nil {
			currentSession.AddMessage("assistant", assistantMsg.String())
			// Auto-save after each exchange
			if err := sessionMgr.Save(currentSession); err != nil {
				// Don't interrupt the conversation, just log the error
				fmt.Printf("%s⚠ Failed to save session: %v%s\n", brandMuted, err, colorReset)
			}
		}
	}

	// Save session one final time on exit
	if currentSession != nil && (saveSession || loadSession) {
		if err := sessionMgr.Save(currentSession); err != nil {
			printWarning(fmt.Sprintf("Failed to save session: %v", err))
		} else {
			printSuccess(fmt.Sprintf("Session '%s' saved (%d messages)", currentSession.Name, currentSession.MessageCount()))
		}
		fmt.Println()
	}
}

// Helper types for handleRun
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type ChatCompletionChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

func formatNumber(n int64) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	} else if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatDuration formats a duration in human-readable form
func formatDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%dns", d.Nanoseconds())
	} else if d < time.Millisecond {
		return fmt.Sprintf("%.2fµs", float64(d.Microseconds()))
	} else if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Milliseconds()))
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

// formatModelSize formats model size in GB with appropriate precision
func formatModelSize(sizeGB float64) string {
	if sizeGB < 0.1 {
		// For sizes less than 0.1 GB, show in MB
		return fmt.Sprintf("%d MB", int(sizeGB*1024))
	} else if sizeGB < 1.0 {
		// For sizes less than 1 GB, use 2 decimal places
		return fmt.Sprintf("%.2f GB", sizeGB)
	}
	// For larger sizes, use 1 decimal place
	return fmt.Sprintf("%.1f GB", sizeGB)
}

// isServerHealthy checks if the server is responding
func isServerHealthy(baseURL string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/health", nil)
	if err != nil {
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// handleAlias manages model aliases
func handleAlias(args []string) {
	printBanner()
	printSection("Model Aliases")

	if len(args) == 0 {
		fmt.Println("Usage:")
		fmt.Println("  offgrid alias list                    - List all aliases")
		fmt.Println("  offgrid alias set <alias> <model>    - Create an alias")
		fmt.Println("  offgrid alias remove <alias>          - Remove an alias")
		fmt.Println()
		return
	}

	configDir := "/var/lib/offgrid"
	if home, err := os.UserHomeDir(); err == nil {
		configDir = filepath.Join(home, ".offgrid")
	}

	am := models.NewAliasManager(configDir)

	switch args[0] {
	case "list", "ls":
		aliases := am.ListAliases()
		if len(aliases) == 0 {
			printInfo("No aliases defined")
			return
		}

		for alias, modelID := range aliases {
			printItem(alias, modelID)
		}

	case "set", "create", "add":
		if len(args) < 3 {
			printError("Usage: offgrid alias set <alias> <model>")
			return
		}

		alias := args[1]
		modelID := args[2]

		if err := am.SetAlias(alias, modelID); err != nil {
			printError(fmt.Sprintf("Failed to set alias: %v", err))
			return
		}

		printSuccess(fmt.Sprintf("Alias '%s' → '%s' created", alias, modelID))

	case "remove", "rm", "delete":
		if len(args) < 2 {
			printError("Usage: offgrid alias remove <alias>")
			return
		}

		alias := args[1]
		if err := am.RemoveAlias(alias); err != nil {
			printError(fmt.Sprintf("Failed to remove alias: %v", err))
			return
		}

		printSuccess(fmt.Sprintf("Alias '%s' removed", alias))

	default:
		printError(fmt.Sprintf("Unknown alias command: %s", args[0]))
	}
}

// handleFavorite manages favorite models
func handleFavorite(args []string) {
	printBanner()
	printSection("Favorite Models")

	if len(args) == 0 {
		fmt.Println("Usage:")
		fmt.Println("  offgrid favorite list            - List favorite models")
		fmt.Println("  offgrid favorite add <model>     - Add to favorites")
		fmt.Println("  offgrid favorite remove <model>  - Remove from favorites")
		fmt.Println()
		return
	}

	configDir := "/var/lib/offgrid"
	if home, err := os.UserHomeDir(); err == nil {
		configDir = filepath.Join(home, ".offgrid")
	}

	am := models.NewAliasManager(configDir)

	switch args[0] {
	case "list", "ls":
		favorites := am.ListFavorites()
		if len(favorites) == 0 {
			printInfo("No favorite models")
			return
		}

		for _, modelID := range favorites {
			fmt.Printf("%s %s\n", iconStar, modelID)
		}

	case "add", "set":
		if len(args) < 2 {
			printError("Usage: offgrid favorite add <model>")
			return
		}

		modelID := args[1]
		if err := am.SetFavorite(modelID, true); err != nil {
			printError(fmt.Sprintf("Failed to add favorite: %v", err))
			return
		}

		printSuccess(fmt.Sprintf("'%s' added to favorites", modelID))

	case "remove", "rm", "delete":
		if len(args) < 2 {
			printError("Usage: offgrid favorite remove <model>")
			return
		}

		modelID := args[1]
		if err := am.SetFavorite(modelID, false); err != nil {
			printError(fmt.Sprintf("Failed to remove favorite: %v", err))
			return
		}

		printSuccess(fmt.Sprintf("'%s' removed from favorites", modelID))

	default:
		printError(fmt.Sprintf("Unknown favorite command: %s", args[0]))
	}
}

// handleTemplate manages prompt templates
func handleTemplate(args []string) {
	printBanner()
	printSection("Prompt Templates")

	if len(args) == 0 {
		fmt.Println("Usage:")
		fmt.Println("  offgrid template list              - List all templates")
		fmt.Println("  offgrid template show <name>       - Show template details")
		fmt.Println("  offgrid template apply <name>      - Apply template (interactive)")
		fmt.Println()
		return
	}

	switch args[0] {
	case "list", "ls":
		fmt.Println()
		templateList := templates.ListTemplates()
		for _, name := range templateList {
			tpl, _ := templates.GetTemplate(name)
			fmt.Printf("%s %-15s %s %s\n", iconDiamond, name, brandMuted+"|"+colorReset, tpl.Description)
		}
		fmt.Println()

	case "show", "info":
		if len(args) < 2 {
			printError("Usage: offgrid template show <name>")
			return
		}

		tpl, err := templates.GetTemplate(args[1])
		if err != nil {
			printError(fmt.Sprintf("Template not found: %s", args[1]))
			return
		}

		fmt.Println()
		printBox(tpl.Name, fmt.Sprintf("%s\n\n%sVariables:%s %s",
			tpl.Description,
			colorBold, colorReset,
			strings.Join(tpl.Variables, ", ")))
		fmt.Println()
		fmt.Println(colorDim + "Template:" + colorReset)
		fmt.Println(tpl.Template)
		fmt.Println()

		if len(tpl.Examples) > 0 {
			fmt.Println(colorDim + "Examples:" + colorReset)
			for key, value := range tpl.Examples {
				printItem(key, value)
			}
			fmt.Println()
		}

	case "apply", "use":
		if len(args) < 2 {
			printError("Usage: offgrid template apply <name>")
			return
		}

		tpl, err := templates.GetTemplate(args[1])
		if err != nil {
			printError(fmt.Sprintf("Template not found: %s", args[1]))
			return
		}

		fmt.Println()
		fmt.Printf("%sTemplate:%s %s - %s\n\n", colorBold, colorReset, tpl.Name, tpl.Description)

		// Collect variables interactively
		variables := make(map[string]string)
		scanner := bufio.NewScanner(os.Stdin)

		for _, varName := range tpl.Variables {
			fmt.Printf("%s%s:%s ", brandPrimary, varName, colorReset)
			if example, ok := tpl.Examples[varName]; ok {
				fmt.Printf("%s(%s)%s\n> ", brandMuted, example, colorReset)
			} else {
				fmt.Print("\n> ")
			}

			scanner.Scan()
			value := scanner.Text()
			if value != "" {
				variables[varName] = value
			}
		}

		prompt, err := tpl.Apply(variables)
		if err != nil {
			printError(fmt.Sprintf("Failed to apply template: %v", err))
			return
		}

		fmt.Println()
		printDivider()
		fmt.Println()
		fmt.Println(prompt)
		fmt.Println()
		printDivider()
		fmt.Println()

	default:
		printError(fmt.Sprintf("Unknown template command: %s", args[0]))
	}
}

// handleBatch processes requests in batch mode
func handleBatch(args []string) {
	printBanner()
	printSection("Batch Processing")

	if len(args) == 0 {
		fmt.Println("Usage:")
		fmt.Println("  offgrid batch process <input.jsonl> [output.jsonl] [--concurrency N]")
		fmt.Println()
		fmt.Println("Input format (JSONL):")
		fmt.Println(`  {"id": "1", "model": "model.gguf", "prompt": "Hello"}`)
		fmt.Println(`  {"id": "2", "model": "model.gguf", "prompt": "World"}`)
		fmt.Println()
		return
	}

	if args[0] != "process" {
		printError("Only 'process' subcommand is supported")
		return
	}

	if len(args) < 2 {
		printError("Usage: offgrid batch process <input.jsonl> [output.jsonl]")
		return
	}

	inputPath := args[1]
	outputPath := "batch-results.jsonl"
	concurrency := 4

	// Parse remaining args
	for i := 2; i < len(args); i++ {
		if args[i] == "--concurrency" || args[i] == "-c" {
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &concurrency)
				i++
			}
		} else if !strings.HasPrefix(args[i], "--") {
			outputPath = args[i]
		}
	}

	printInfo(fmt.Sprintf("Processing: %s → %s (concurrency=%d)", inputPath, outputPath, concurrency))
	fmt.Println()

	// Load config
	configPath := os.Getenv("OFFGRID_CONFIG")
	_, err := config.LoadWithPriority(configPath)
	if err != nil {
		printError(fmt.Sprintf("Failed to load config: %v", err))
		return
	}

	// Create inference engine
	engine := inference.NewMockEngine()

	// Create batch processor
	processor := batch.NewProcessor(engine, concurrency)

	// Process file
	ctx := context.Background()
	if err := processor.ProcessFile(ctx, inputPath, outputPath); err != nil {
		printError(fmt.Sprintf("Batch processing failed: %v", err))
		return
	}

	printSuccess(fmt.Sprintf("Results written to: %s", outputPath))
}

// handleSession handles session commands
func handleSession(args []string) {
	printBanner()
	printSection("Session Management")

	homeDir, _ := os.UserHomeDir()
	sessionsDir := filepath.Join(homeDir, ".offgrid", "sessions")
	sessionMgr := sessions.NewSessionManager(sessionsDir)

	if len(args) == 0 {
		fmt.Println("Usage:")
		fmt.Printf("  %soffgrid session%s list\n", colorBold, colorReset)
		fmt.Printf("  %soffgrid session%s show <name>\n", colorBold, colorReset)
		fmt.Printf("  %soffgrid session%s delete <name>\n", colorBold, colorReset)
		fmt.Printf("  %soffgrid session%s export <name> [output.md]\n", colorBold, colorReset)
		fmt.Println()
		fmt.Println("Manage conversation sessions for persistent chat history")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Printf("  %s$%s offgrid session list\n", brandMuted, colorReset)
		fmt.Printf("  %s$%s offgrid session show my-project\n", brandMuted, colorReset)
		fmt.Printf("  %s$%s offgrid session export my-project output.md\n", brandMuted, colorReset)
		fmt.Println()
		return
	}

	subcommand := args[0]

	switch subcommand {
	case "list", "ls":
		handleSessionList(sessionMgr)
	case "show", "view":
		if len(args) < 2 {
			printError("Usage: offgrid session show <name>")
			return
		}
		handleSessionShow(sessionMgr, args[1])
	case "delete", "del", "rm":
		if len(args) < 2 {
			printError("Usage: offgrid session delete <name>")
			return
		}
		handleSessionDelete(sessionMgr, args[1])
	case "export":
		if len(args) < 2 {
			printError("Usage: offgrid session export <name> [output.md]")
			return
		}
		outputPath := ""
		if len(args) >= 3 {
			outputPath = args[2]
		}
		handleSessionExport(sessionMgr, args[1], outputPath)
	default:
		printError(fmt.Sprintf("Unknown subcommand: %s", subcommand))
		fmt.Println("Available subcommands: list, show, delete, export")
	}
}

func handleSessionList(sessionMgr *sessions.SessionManager) {
	sessionList, err := sessionMgr.List()
	if err != nil {
		if output.JSONMode {
			output.Error("Failed to list sessions", err)
		}
		printError(fmt.Sprintf("Failed to list sessions: %v", err))
		return
	}

	// JSON output mode
	if output.JSONMode {
		var jsonSessions []output.SessionInfo
		for _, sess := range sessionList {
			jsonSessions = append(jsonSessions, output.SessionInfo{
				Name:      sess.Name,
				ModelID:   sess.ModelID,
				Messages:  sess.MessageCount(),
				CreatedAt: sess.CreatedAt.Format(time.RFC3339),
				UpdatedAt: sess.UpdatedAt.Format(time.RFC3339),
			})
		}
		output.PrintSessions(jsonSessions)
		return
	}

	// Human-readable output
	if len(sessionList) == 0 {
		printInfo("No saved sessions found")
		fmt.Println()
		printInfo("Sessions are automatically saved when using the chat command with --save flag")
		return
	}

	fmt.Printf("Found %d session(s):\n\n", len(sessionList))

	for i, sess := range sessionList {
		fmt.Printf("  %d. %s%s%s\n", i+1, brandPrimary, sess.Name, colorReset)
		fmt.Printf("     Model: %s · Messages: %d · Updated: %s\n",
			sess.ModelID, sess.MessageCount(), formatTimeAgo(sess.UpdatedAt))
		if i < len(sessionList)-1 {
			fmt.Println()
		}
	}
	fmt.Println()
}

func handleSessionShow(sessionMgr *sessions.SessionManager, name string) {
	sess, err := sessionMgr.Load(name)
	if err != nil {
		printError(fmt.Sprintf("Failed to load session: %v", err))
		return
	}

	fmt.Printf("%s%s%s\n", colorBold, sess.Name, colorReset)
	fmt.Printf("Model: %s\n", sess.ModelID)
	fmt.Printf("Created: %s\n", sess.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated: %s (%s)\n", sess.UpdatedAt.Format("2006-01-02 15:04:05"), formatTimeAgo(sess.UpdatedAt))
	fmt.Printf("Messages: %d\n", sess.MessageCount())
	fmt.Println()
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println()

	for i, msg := range sess.Messages {
		if msg.Role == "user" {
			fmt.Printf("%s● User%s (%s)\n", brandPrimary, colorReset, msg.Timestamp.Format("15:04:05"))
		} else {
			fmt.Printf("%s● Assistant%s (%s)\n", brandSuccess, colorReset, msg.Timestamp.Format("15:04:05"))
		}
		fmt.Println(msg.Content)
		if i < len(sess.Messages)-1 {
			fmt.Println()
			fmt.Println(strings.Repeat("─", 60))
			fmt.Println()
		}
	}
	fmt.Println()
}

func handleSessionDelete(sessionMgr *sessions.SessionManager, name string) {
	if err := sessionMgr.Delete(name); err != nil {
		printError(fmt.Sprintf("Failed to delete session: %v", err))
		return
	}

	printSuccess(fmt.Sprintf("Deleted session: %s", name))
}

func handleSessionExport(sessionMgr *sessions.SessionManager, name string, outputPath string) {
	sess, err := sessionMgr.Load(name)
	if err != nil {
		printError(fmt.Sprintf("Failed to load session: %v", err))
		return
	}

	markdown := sess.ExportMarkdown()

	if outputPath == "" {
		outputPath = sess.Name + ".md"
	}

	if err := os.WriteFile(outputPath, []byte(markdown), 0644); err != nil {
		printError(fmt.Sprintf("Failed to write file: %v", err))
		return
	}

	printSuccess(fmt.Sprintf("Exported to: %s", outputPath))
}

// formatTimeAgo formats a time as a human-readable "ago" string
func formatTimeAgo(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else if duration < 7*24*time.Hour {
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	} else {
		return t.Format("2006-01-02")
	}
}

// handleCompletions generates shell completion scripts
func handleCompletions(args []string) {
	// Only show banner/help when no args provided (help mode)
	// Don't show when generating actual completion scripts
	if len(args) == 0 {
		printBanner()
		printSection("Shell Completions")
		fmt.Println("Usage:")
		fmt.Printf("  %soffgrid completions%s <shell>\n", colorBold, colorReset)
		fmt.Println()
		fmt.Println("Supported shells:")
		fmt.Printf("  %sbash%s    Bash completion script\n", brandPrimary, colorReset)
		fmt.Printf("  %szsh%s     Zsh completion script\n", brandPrimary, colorReset)
		fmt.Printf("  %sfish%s    Fish completion script\n", brandPrimary, colorReset)
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Printf("  %s$%s offgrid completions bash > /etc/bash_completion.d/offgrid\n", brandMuted, colorReset)
		fmt.Printf("  %s$%s offgrid completions zsh > ~/.zsh/completions/_offgrid\n", brandMuted, colorReset)
		fmt.Printf("  %s$%s offgrid completions fish > ~/.config/fish/completions/offgrid.fish\n", brandMuted, colorReset)
		fmt.Println()
		return
	}

	shell := args[0]
	gen := completions.NewGenerator("offgrid")

	// Add all commands with their subcommands and flags
	gen.AddCommand(completions.Command{
		Name:        "list",
		Flags:       []string{"--json"},
		Description: "List available models",
	})
	gen.AddCommand(completions.Command{
		Name:        "search",
		Flags:       []string{"--author", "--limit", "--json"},
		Description: "Search for models",
	})
	gen.AddCommand(completions.Command{
		Name:        "download",
		Description: "Download a model",
	})
	gen.AddCommand(completions.Command{
		Name:        "download-hf",
		Flags:       []string{"--quant"},
		Description: "Download from HuggingFace",
	})
	gen.AddCommand(completions.Command{
		Name:        "run",
		Flags:       []string{"--save", "--load"},
		Description: "Start interactive chat",
	})
	gen.AddCommand(completions.Command{
		Name:        "import",
		Description: "Import models from USB",
	})
	gen.AddCommand(completions.Command{
		Name:        "export",
		Description: "Export model to USB",
	})
	gen.AddCommand(completions.Command{
		Name:        "remove",
		Description: "Remove a model",
	})
	gen.AddCommand(completions.Command{
		Name:        "benchmark",
		Description: "Benchmark a model",
	})
	gen.AddCommand(completions.Command{
		Name:        "quantization",
		Description: "Show quantization info",
	})
	gen.AddCommand(completions.Command{
		Name:        "alias",
		Subcommands: []string{"list", "set", "remove"},
		Description: "Manage model aliases",
	})
	gen.AddCommand(completions.Command{
		Name:        "favorite",
		Subcommands: []string{"list", "add", "remove"},
		Description: "Manage favorites",
	})
	gen.AddCommand(completions.Command{
		Name:        "template",
		Subcommands: []string{"list", "show", "apply"},
		Description: "Manage templates",
	})
	gen.AddCommand(completions.Command{
		Name:        "batch",
		Subcommands: []string{"process"},
		Flags:       []string{"--concurrency"},
		Description: "Batch processing",
	})
	gen.AddCommand(completions.Command{
		Name:        "session",
		Subcommands: []string{"list", "show", "export", "delete"},
		Flags:       []string{"--json"},
		Description: "Manage sessions",
	})
	gen.AddCommand(completions.Command{
		Name:        "completions",
		Subcommands: []string{"bash", "zsh", "fish"},
		Description: "Generate completions",
	})
	gen.AddCommand(completions.Command{
		Name:        "config",
		Subcommands: []string{"init", "show", "validate"},
		Description: "Manage configuration",
	})
	gen.AddCommand(completions.Command{
		Name:        "info",
		Flags:       []string{"--json"},
		Description: "System information",
	})
	gen.AddCommand(completions.Command{
		Name:        "serve",
		Description: "Start API server",
	})
	gen.AddCommand(completions.Command{
		Name:        "help",
		Description: "Show help",
	})

	var script string
	switch shell {
	case "bash":
		script = gen.GenerateBash()
	case "zsh":
		script = gen.GenerateZsh()
	case "fish":
		script = gen.GenerateFish()
	default:
		printError(fmt.Sprintf("Unsupported shell: %s", shell))
		fmt.Println()
		fmt.Println("Supported shells: bash, zsh, fish")
		fmt.Println()
		return
	}

	fmt.Println(script)
}
