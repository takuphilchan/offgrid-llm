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
	"github.com/takuphilchan/offgrid-llm/internal/server"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/internal/templates"
)

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
	fmt.Println("    ║      OFFGRID LLM  v0.1.0α        ║")
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
	// Check if systemd is available
	cmd := exec.Command("systemctl", "--version")
	if err := cmd.Run(); err != nil {
		// Systemd not available
		return fmt.Errorf("systemd not available - manual restart required")
	}

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

// waitForLlamaServerReady polls llama-server until it's ready or timeout
func waitForLlamaServerReady(timeoutSec int) error {
	// Read llama-server port
	portBytes, err := os.ReadFile("/etc/offgrid/llama-port")
	if err != nil {
		return fmt.Errorf("could not read llama-server port: %w", err)
	}
	port := strings.TrimSpace(string(portBytes))

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
		printDivider()
		fmt.Println()
		printSection("Usage")
		fmt.Printf("  %soffgrid download%s <model-id> [quantization]\n", colorBold, colorReset)
		fmt.Println()
		printSection("Description")
		fmt.Println("  Download a model from the built-in catalog")
		fmt.Println()
		printSection("Examples")
		fmt.Printf("  %s$%s offgrid download tinyllama-1.1b-chat Q4_K_M\n", brandMuted, colorReset)
		fmt.Printf("  %s$%s offgrid download llama-2-7b-chat\n", brandMuted, colorReset)
		fmt.Println()
		printInfo("Use 'offgrid catalog' to see available models")
		fmt.Println()
		printDivider()
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
		printDivider()
		fmt.Println()
		printSection("Usage")
		fmt.Printf("  %soffgrid import%s <usb-path> [model-file]\n", colorBold, colorReset)
		fmt.Println()
		printSection("Description")
		fmt.Println("  Import GGUF models from USB/SD card or external storage")
		fmt.Println()
		printSection("Examples")
		fmt.Printf("  %s$%s offgrid import /media/usb              # Import all .gguf files\n", brandMuted, colorReset)
		fmt.Printf("  %s$%s offgrid import /media/usb/model.gguf  # Import specific file\n", brandMuted, colorReset)
		fmt.Printf("  %s$%s offgrid import D:\\models              # Windows directory\n", brandMuted, colorReset)
		fmt.Println()
		printInfo("Use 'offgrid list' to view imported models")
		fmt.Println()
		printDivider()
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
		printDivider()
		fmt.Println()
		printSection("Usage")
		fmt.Printf("  %soffgrid remove%s <model-id>\n", colorBold, colorReset)
		fmt.Println()
		printSection("Description")
		fmt.Println("  Remove an installed model from your system")
		fmt.Println()
		printSection("Examples")
		fmt.Printf("  %s$%s offgrid remove tinyllama-1.1b-chat.Q4_K_M\n", brandMuted, colorReset)
		fmt.Printf("  %s$%s offgrid remove llama-2-7b-chat.Q5_K_M\n", brandMuted, colorReset)
		fmt.Println()
		printInfo("Use 'offgrid list' to see installed models")
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
	fmt.Println("🗑️  Remove Model")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Printf("Model:  %s\n", modelID)
	if meta.Path != "" {
		fmt.Printf("Path:   %s\n", meta.Path)
	}
	if meta.Size > 0 {
		fmt.Printf("Size:   %s will be freed\n", formatBytes(meta.Size))
	}
	fmt.Println()
	fmt.Print("⚠️  This action cannot be undone. Continue? (y/N): ")

	var response string
	fmt.Scanln(&response)

	if response != "y" && response != "Y" {
		fmt.Println()
		fmt.Println("✓ Cancelled - model preserved")
		fmt.Println()
		return
	}

	// Delete the model file
	if meta.Path != "" {
		if err := os.Remove(meta.Path); err != nil {
			fmt.Fprintf(os.Stderr, "\n✗ Failed to remove file: %v\n\n", err)
			os.Exit(1)
		}
	}

	fmt.Println()
	fmt.Printf("✓ Removed %s\n", modelID)

	// Rescan to update registry after file deletion
	if err := registry.ScanModels(); err != nil {
		fmt.Fprintf(os.Stderr, "\n⚠️  Warning: Failed to refresh model list: %v\n", err)
	}

	// Show remaining models
	remaining := registry.ListModels()
	fmt.Printf("\n%d model(s) remaining\n\n", len(remaining))
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

func handleBenchmark(args []string) {
	if len(args) < 1 {
		printDivider()
		fmt.Println()
		printSection("Usage")
		fmt.Printf("  %soffgrid benchmark%s <model-id>\n", colorBold, colorReset)
		fmt.Println()
		printSection("Description")
		fmt.Println("  Benchmark model performance and resource usage")
		fmt.Println()
		printSection("Examples")
		fmt.Printf("  %s$%s offgrid benchmark tinyllama-1.1b-chat.Q4_K_M\n", brandMuted, colorReset)
		fmt.Printf("  %s$%s offgrid benchmark llama-2-7b-chat.Q5_K_M\n", brandMuted, colorReset)
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
		fmt.Fprintf(os.Stderr, "✗ Error scanning models: %v\n\n", err)
		os.Exit(1)
	}

	modelID := args[0]

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

	fmt.Println("⚡ Benchmark Model")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("Model Information")
	fmt.Printf("  Name:          %s\n", modelID)
	fmt.Printf("  Path:          %s\n", meta.Path)
	fmt.Printf("  Size:          %s\n", formatBytes(meta.Size))
	if meta.Quantization != "" {
		fmt.Printf("  Quantization:  %s\n", meta.Quantization)
	}
	fmt.Println()

	fmt.Println("Performance Metrics")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("  ⏳ This feature requires llama.cpp integration")
	fmt.Println()
	fmt.Println("  Metrics will include:")
	fmt.Println("    • Model load time")
	fmt.Println("    • Tokens per second (inference speed)")
	fmt.Println("    • Memory usage (RAM/VRAM)")
	fmt.Println("    • First token latency")
	fmt.Println("    • Context processing speed")
	fmt.Println()
	fmt.Println("  Next steps:")
	fmt.Println("    1. Ensure server is running: offgrid serve")
	fmt.Println("    2. Use API endpoint: curl http://localhost:11611/v1/benchmark")
	fmt.Println()

	// TODO: Implement actual benchmarking
	// This would require:
	// 1. Load model with inference engine
	// 2. Run test prompts
	// 3. Measure timing and resource usage
	// 4. Display results
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

	// Human-readable output
	printDivider()
	fmt.Println()
	printSection("Installed Models")

	if len(modelList) == 0 {
		fmt.Println()
		printInfo(fmt.Sprintf("No models found in %s", cfg.ModelsDir))
		fmt.Println()
		printSection("Get Started")
		printItem("Search HuggingFace", "offgrid search llama")
		printItem("Download model", "offgrid download-hf <model-id>")
		printItem("Browse catalog", "offgrid catalog")
		fmt.Println()
		printDivider()
		fmt.Println()
		return
	}

	fmt.Println()
	fmt.Printf("Found %s%d%s model(s):\n\n", brandPrimary, len(modelList), colorReset)

	var totalSize int64
	for _, model := range modelList {
		meta, err := registry.GetModel(model.ID)
		if err == nil {
			fmt.Printf("  %s◭%s %s", brandSecondary, colorReset, model.ID)
			if meta.Size > 0 {
				fmt.Printf(" %s·%s %s", brandMuted, colorReset, formatBytes(meta.Size))
				totalSize += meta.Size
			}
			if meta.Quantization != "" && meta.Quantization != "unknown" {
				fmt.Printf(" %s·%s %s", brandMuted, colorReset, meta.Quantization)
			}
			fmt.Println()
		} else {
			fmt.Printf("  %s◭%s %s\n", brandSecondary, colorReset, model.ID)
		}
	}

	fmt.Println()
	if totalSize > 0 {
		fmt.Printf("Total size: %s%s%s\n", brandPrimary, formatBytes(totalSize), colorReset)
		fmt.Println()
	}

	printSection("Next Steps")
	printItem("Start chat", "offgrid run <model-name>")
	printItem("Start server", "offgrid serve")
	printItem("Benchmark model", "offgrid benchmark <model-name>")
	fmt.Println()
	printDivider()
	fmt.Println()
}

func handleCatalog() {
	catalog := models.DefaultCatalog()

	fmt.Println()
	printSection("Model Catalog")
	fmt.Println()

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
	fmt.Printf("%sLanguage Models%s (%d)\n", colorBold, colorReset, len(llms))
	printDivider()
	fmt.Println()

	for i, entry := range llms {
		star := ""
		if entry.Recommended {
			star = fmt.Sprintf(" %s★%s", brandSuccess, colorReset)
		}

		fmt.Printf("%s%s%s%s %s· %s · %d GB RAM%s\n",
			brandPrimary, entry.ID, colorReset, star,
			brandMuted, entry.Parameters, entry.MinRAM, colorReset)

		// Variants on same line
		fmt.Printf("   %s", brandMuted)
		for i, v := range entry.Variants {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Printf("%s (%.1f GB)", v.Quantization, float64(v.Size)/(1024*1024*1024))
		}
		fmt.Printf("%s\n", colorReset)

		if i < len(llms)-1 {
			fmt.Println()
		}
	}

	// Show embeddings if any
	if len(embeddings) > 0 {
		fmt.Println()
		fmt.Println()
		fmt.Printf("%sEmbedding Models%s (%d)\n", colorBold, colorReset, len(embeddings))
		printDivider()
		fmt.Println()

		for i, entry := range embeddings {
			star := ""
			if entry.Recommended {
				star = fmt.Sprintf(" %s★%s", brandSuccess, colorReset)
			}

			fmt.Printf("%s%s%s%s\n",
				brandPrimary, entry.ID, colorReset, star)

			// Variants on same line
			fmt.Printf("   %s", brandMuted)
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

			if i < len(embeddings)-1 {
				fmt.Println()
			}
		}
	}

	fmt.Println()
	fmt.Println()
	printSection("Usage")
	printItem("Download model", "offgrid download <model-id> [quantization]")
	printItem("Search HuggingFace", "offgrid search llama --author TheBloke")
	printItem("Learn quantization", "offgrid quantization")
	fmt.Println()
}

func handleQuantization() {
	fmt.Println()
	printSection("Quantization Levels")
	fmt.Println()
	fmt.Printf("%sLower bits = smaller size + faster speed - slight quality loss%s\n", brandMuted, colorReset)
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
		fmt.Printf("%s%s%s\n", colorBold, tier.name, colorReset)
		for _, quant := range tier.quants {
			info := models.GetQuantizationInfo(quant)
			star := "  "
			starColor := ""
			if quant == "Q4_K_M" || quant == "Q5_K_M" {
				star = "★ "
				starColor = brandSuccess
			}

			fmt.Printf("  %s%s%s%-8s%s %.1f bits %s· %s%s\n",
				starColor, star, colorReset,
				info.Name, brandMuted,
				info.BitsPerWeight,
				colorReset, info.Description, colorReset)
		}
		fmt.Println()
	}

	printSection("Quick Guide")
	fmt.Printf("  %s★%s %sQ4_K_M%s  Best for most users (4.0 GB for 7B model)\n", brandSuccess, colorReset, brandPrimary, colorReset)
	fmt.Printf("  %s★%s %sQ5_K_M%s  Production quality (4.8 GB for 7B model)\n", brandSuccess, colorReset, brandPrimary, colorReset)
	fmt.Printf("     %sQ3_K_M%s  Limited RAM (3.0 GB for 7B model)\n", brandPrimary, colorReset)
	fmt.Printf("     %sQ8_0%s    Maximum quality (7.2 GB for 7B model)\n", brandPrimary, colorReset)
	fmt.Println()
}

func printHelp() {
	printDivider()
	fmt.Println()

	printSection("Usage")
	fmt.Printf("  %soffgrid%s [command] [options]\n", colorBold, colorReset)
	fmt.Println()

	// Model Management
	fmt.Printf("%sModel Management%s\n", colorBold, colorReset)
	printDivider()
	fmt.Printf("  %slist%s               List installed models\n", brandPrimary, colorReset)
	fmt.Printf("  %scatalog%s            Browse available models\n", brandPrimary, colorReset)
	fmt.Printf("  %ssearch%s <query>     Search HuggingFace\n", brandPrimary, colorReset)
	fmt.Printf("  %sdownload%s <id>      Download from catalog\n", brandPrimary, colorReset)
	fmt.Printf("  %sdownload-hf%s <id>   Download from HuggingFace\n", brandPrimary, colorReset)
	fmt.Printf("  %simport%s <path>      Import from USB/SD card\n", brandPrimary, colorReset)
	fmt.Printf("  %sexport%s <id> <dst>  Export to USB/SD card\n", brandPrimary, colorReset)
	fmt.Printf("  %sremove%s <id>        Remove installed model\n", brandPrimary, colorReset)
	fmt.Println()

	// Inference & Chat
	fmt.Printf("%sInference & Chat%s\n", colorBold, colorReset)
	printDivider()
	fmt.Printf("  %sserve%s              Start API server (default)\n", brandPrimary, colorReset)
	fmt.Printf("  %srun%s <model>        Interactive chat session\n", brandPrimary, colorReset)
	fmt.Printf("  %ssession%s <cmd>      Manage chat sessions\n", brandPrimary, colorReset)
	fmt.Printf("  %stemplate%s <cmd>     Manage prompt templates\n", brandPrimary, colorReset)
	fmt.Printf("  %sbatch%s <file>       Batch process prompts\n", brandPrimary, colorReset)
	fmt.Println()

	// Configuration
	fmt.Printf("%sConfiguration & Tools%s\n", colorBold, colorReset)
	printDivider()
	fmt.Printf("  %sinfo%s               System information\n", brandPrimary, colorReset)
	fmt.Printf("  %sconfig%s <action>    Manage configuration\n", brandPrimary, colorReset)
	fmt.Printf("  %squantization%s       Quantization guide\n", brandPrimary, colorReset)
	fmt.Printf("  %salias%s <cmd>        Model aliases\n", brandPrimary, colorReset)
	fmt.Printf("  %sfavorite%s <cmd>     Favorite models\n", brandPrimary, colorReset)
	fmt.Printf("  %sbenchmark%s <id>     Performance testing\n", brandPrimary, colorReset)
	fmt.Printf("  %scompletions%s <shell> Shell completions\n", brandPrimary, colorReset)
	fmt.Printf("  %shelp%s               Show this help\n", brandPrimary, colorReset)
	fmt.Println()

	printSection("Examples")
	fmt.Printf("  %s$%s offgrid search llama --author TheBloke\n", brandMuted, colorReset)
	fmt.Printf("  %s$%s offgrid download tinyllama-1.1b-chat Q4_K_M\n", brandMuted, colorReset)
	fmt.Printf("  %s$%s offgrid download-hf TheBloke/Llama-2-7B-Chat-GGUF\n", brandMuted, colorReset)
	fmt.Printf("  %s$%s offgrid run tinyllama-1.1b-chat.Q4_K_M --save project\n", brandMuted, colorReset)
	fmt.Printf("  %s$%s offgrid import /media/usb\n", brandMuted, colorReset)
	fmt.Printf("  %s$%s offgrid session list\n", brandMuted, colorReset)
	fmt.Println()

	printSection("Environment Variables")
	printItem("OFFGRID_CONFIG", "Configuration file path (YAML/JSON)")
	printItem("OFFGRID_PORT", "Server port (default: 11611)")
	printItem("OFFGRID_MODELS_DIR", "Models directory")
	printItem("OFFGRID_NUM_THREADS", "CPU threads")
	fmt.Println()

	printSection("Global Flags")
	printItem("--json", "Output in JSON format (for scripting)")
	fmt.Println()

	printDivider()
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
	printSection(fmt.Sprintf("OffGrid LLM %sv0.1.0-alpha%s", brandMuted, colorReset))
	fmt.Println()

	// Configuration - more compact
	fmt.Printf("%sConfiguration%s\n", colorBold, colorReset)
	printDivider()
	fmt.Printf("  %sPort:%s %d  %s│%s  %sModels:%s %s\n",
		brandMuted, colorReset, cfg.ServerPort,
		brandMuted, colorReset,
		brandMuted, colorReset, cfg.ModelsDir)
	fmt.Printf("  %sThreads:%s %d  %s│%s  %sContext:%s %d tokens  %s│%s  %sMemory:%s %d MB\n",
		brandMuted, colorReset, cfg.NumThreads,
		brandMuted, colorReset,
		brandMuted, colorReset, cfg.MaxContextSize,
		brandMuted, colorReset,
		brandMuted, colorReset, cfg.MaxMemoryMB)
	if cfg.EnableP2P {
		fmt.Printf("  %sP2P:%s enabled (port %d)\n", brandMuted, colorReset, cfg.P2PPort)
	}
	fmt.Println()

	// Installed Models - more visual
	var totalSize int64
	fmt.Printf("%sInstalled Models%s (%s%d%s)\n", colorBold, colorReset, brandPrimary, len(modelList), colorReset)
	printDivider()
	if len(modelList) > 0 {
		for _, model := range modelList {
			meta, err := registry.GetModel(model.ID)
			if err == nil {
				statusIcon := "○"
				statusColor := brandMuted
				if meta.IsLoaded {
					statusIcon = "●"
					statusColor = brandSuccess
				}

				fmt.Printf("  %s%s%s %s", statusColor, statusIcon, colorReset, model.ID)
				if meta.Size > 0 {
					fmt.Printf(" %s·%s %s", brandMuted, colorReset, formatBytes(meta.Size))
					totalSize += meta.Size
				}
				if meta.Quantization != "" && meta.Quantization != "unknown" {
					fmt.Printf(" %s·%s %s", brandMuted, colorReset, meta.Quantization)
				}
				fmt.Println()
			}
		}
		if totalSize > 0 {
			fmt.Printf("  %sTotal:%s %s\n", brandMuted, colorReset, formatBytes(totalSize))
		}
	} else {
		fmt.Printf("  %sNo models installed%s\n", brandMuted, colorReset)
	}
	fmt.Println()

	// Available Models
	catalog := models.DefaultCatalog()
	recommended := 0
	for _, entry := range catalog.Models {
		if entry.Recommended {
			recommended++
		}
	}
	fmt.Printf("%sAvailable in Catalog%s (%s%d%s total, %s%d%s recommended)\n",
		colorBold, colorReset,
		brandPrimary, len(catalog.Models), colorReset,
		brandSuccess, recommended, colorReset)
	printDivider()
	fmt.Println()

	// Quick Start
	if len(modelList) == 0 {
		printSection("Quick Start")
		printItem("1. Download model", "offgrid download tinyllama-1.1b-chat Q4_K_M")
		printItem("2. Start server", "offgrid serve")
		printItem("3. Test endpoint", fmt.Sprintf("curl http://localhost:%d/health", cfg.ServerPort))
	} else {
		printSection("Server")
		printItem("Start server", "offgrid serve")
		printItem("API endpoint", fmt.Sprintf("http://localhost:%d", cfg.ServerPort))
		printItem("Health check", fmt.Sprintf("http://localhost:%d/health", cfg.ServerPort))
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
			printDivider()
			fmt.Println()
			printSection("Usage")
			fmt.Printf("  %soffgrid search%s [query] [options]\n", colorBold, colorReset)
			fmt.Println()
			printSection("Options")
			printItem("-a, --author <name>", "Filter by author (e.g., 'TheBloke')")
			printItem("-q, --quant <type>", "Filter by quantization (e.g., 'Q4_K_M')")
			printItem("-s, --sort <field>", "Sort by: downloads, likes, created, modified")
			printItem("-l, --limit <n>", "Limit results (default: 20)")
			printItem("--all", "Include gated models")
			fmt.Println()
			printSection("Examples")
			fmt.Printf("  %s$%s offgrid search llama\n", brandMuted, colorReset)
			fmt.Printf("  %s$%s offgrid search mistral --author TheBloke --quant Q4_K_M\n", brandMuted, colorReset)
			fmt.Printf("  %s$%s offgrid search --sort likes --limit 10\n", brandMuted, colorReset)
			fmt.Println()
			printDivider()
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
	fmt.Printf("📦 Fetching model info: %s\n", modelID)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	model, err := hf.GetModelInfo(modelID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintf(os.Stderr, "✗ Failed to fetch model: %v\n", err)
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Make sure:")
		fmt.Fprintln(os.Stderr, "  • The model ID is correct")
		fmt.Fprintln(os.Stderr, "  • You have internet connectivity")
		fmt.Fprintln(os.Stderr, "")
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
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "✗ No matching GGUF files found")
		fmt.Fprintln(os.Stderr, "")
		if quantFilter != "" {
			fmt.Fprintf(os.Stderr, "No files with quantization '%s' found.\n", quantFilter)
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Try without --quant filter or use 'offgrid search' to see available quantizations")
		} else {
			fmt.Fprintln(os.Stderr, "This model may not have GGUF format files.")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Search for GGUF models:")
			fmt.Fprintln(os.Stderr, "  offgrid search <query> --author TheBloke")
		}
		fmt.Fprintln(os.Stderr, "")
		os.Exit(1)
	}

	// If multiple files, let user choose
	var selectedFile models.GGUFFileInfo
	if len(ggufFiles) == 1 {
		selectedFile = ggufFiles[0]
	} else {
		fmt.Println("Available files:")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		for i, file := range ggufFiles {
			fmt.Printf("  %d. %s (%s)\n", i+1, file.Filename, file.Quantization)
		}
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Print("\nSelect file (1-", len(ggufFiles), "): ")

		var choice int
		fmt.Scanf("%d", &choice)
		if choice < 1 || choice > len(ggufFiles) {
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "✗ Invalid choice")
			fmt.Fprintln(os.Stderr, "")
			os.Exit(1)
		}
		selectedFile = ggufFiles[choice-1]
	}

	fmt.Println()
	fmt.Printf("📥 Downloading: %s\n", selectedFile.Filename)
	if selectedFile.SizeGB > 0 {
		fmt.Printf("   Size: %.1f GB\n", selectedFile.SizeGB)
	}
	fmt.Printf("   Quantization: %s\n", selectedFile.Quantization)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// Create destination path
	destPath := filepath.Join(cfg.ModelsDir, selectedFile.Filename)

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

		fmt.Printf("\r  ⏬ Progress: %.1f%% (%.1f / %.1f GB) · %.1f MB/s  ",
			percent,
			float64(current)/(1024*1024*1024),
			float64(total)/(1024*1024*1024),
			speed)
	})

	if err != nil {
		fmt.Println()
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintf(os.Stderr, "✗ Download failed: %v\n", err)
		fmt.Fprintln(os.Stderr, "")
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("✓ Download complete!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  Model: %s\n", selectedFile.Filename)
	fmt.Printf("  Location: %s\n", destPath)
	fmt.Println()

	// Reload llama-server with the downloaded model
	if err := reloadLlamaServerWithModel(destPath); err != nil {
		fmt.Println()
		printWarning(fmt.Sprintf("Could not auto-reload server: %v", err))
		fmt.Println()
		printInfo("Manually restart the server:")
		printItem("Restart service", "sudo systemctl restart llama-server")
		fmt.Println()
	}

	// Extract model name (filename without .gguf extension for CLI)
	modelName := selectedFile.Filename
	if strings.HasSuffix(modelName, ".gguf") {
		modelName = modelName[:len(modelName)-5]
	}

	fmt.Println("Run it:")
	fmt.Printf("  offgrid run %s\n", modelName)
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

	// Check if model exists locally
	registry := models.NewRegistry(cfg.ModelsDir)
	if err := registry.ScanModels(); err != nil {
		fmt.Fprintf(os.Stderr, "\n✗ Error: Failed to scan models directory\n")
		fmt.Fprintf(os.Stderr, "  %v\n\n", err)
		os.Exit(1)
	}

	// Try to find the model
	modelInfo, err := registry.GetModel(modelName)
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

	// Switch to the requested model and reload llama-server
	modelPath := modelInfo.Path
	if err := reloadLlamaServerWithModel(modelPath); err != nil {
		fmt.Println()
		printWarning(fmt.Sprintf("Could not switch to model: %v", err))
		printInfo("You may need to manually restart llama-server:")
		printItem("Restart", "sudo systemctl restart llama-server")
		fmt.Println()
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

	printDivider()
	fmt.Println()
	printSection(fmt.Sprintf("Interactive Chat · %s", modelName))
	fmt.Println()
	if currentSession != nil {
		printInfo(fmt.Sprintf("Session: %s (auto-saving)", currentSession.Name))
	}
	printInfo("Type 'exit' to quit, 'clear' to reset conversation")
	fmt.Println()
	printDivider()
	fmt.Println()

	// Start chat session
	fmt.Printf("%s⚡%s Connecting to inference engine...", brandAccent, colorReset)

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
		fmt.Print("┌─ You\n└─ ")
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
			printDivider()
			fmt.Println()
			printSuccess("Chat session ended")
			fmt.Println()
			break
		}

		if input == "clear" {
			messages = []ChatMessage{}
			// Clear session too if active
			if currentSession != nil {
				currentSession.Messages = []sessions.Message{}
				if err := sessionMgr.Save(currentSession); err != nil {
					printWarning(fmt.Sprintf("Failed to save cleared session: %v", err))
				}
			}
			fmt.Println()
			printDivider()
			fmt.Println()
			printSuccess("Conversation cleared")
			fmt.Println()
			printDivider()
			fmt.Println()
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
		fmt.Print("┌─ Assistant\n└─ ")
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
