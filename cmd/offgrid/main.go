package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/server"
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

	// Wait a moment for service to start
	time.Sleep(2 * time.Second)

	// Check if service is active
	cmd = exec.Command("systemctl", "is-active", "llama-server")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("llama-server failed to start - check logs with: sudo journalctl -u llama-server -n 50")
	}

	printSuccess("Inference server reloaded")

	// Note: Currently llama-server loads the first model found or default
	// This will be improved in a future version to support dynamic model loading

	return nil
}

func main() {
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

	// Reload llama-server with the new model
	if err := reloadLlamaServer(); err != nil {
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
		fmt.Println("Importing models...\n")
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

		// Reload llama-server with the new model
		if err := reloadLlamaServer(); err != nil {
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
		printError(fmt.Sprintf("Error scanning models: %v", err))
		os.Exit(1)
	}

	modelList := registry.ListModels()

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

	fmt.Println("📚 Model Catalog")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	for _, entry := range catalog.Models {
		recommended := ""
		if entry.Recommended {
			recommended = " ★"
		}

		fmt.Printf("%s%s\n", entry.ID, recommended)
		fmt.Printf("  %s · %s parameters · %d GB RAM minimum\n",
			entry.Name, entry.Parameters, entry.MinRAM)
		fmt.Printf("  %s\n", entry.Description)
		fmt.Printf("  Variants: ")

		for i, v := range entry.Variants {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Printf("%s (%.1f GB)", v.Quantization, float64(v.Size)/(1024*1024*1024))
		}
		fmt.Println()
		fmt.Println()
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  offgrid download <model-id> [quantization]")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  offgrid download tinyllama-1.1b-chat Q4_K_M")
	fmt.Println("  offgrid quantization  # Learn about quantization levels")
	fmt.Println()
	fmt.Println("Or search HuggingFace for more models:")
	fmt.Println("  offgrid search llama --author TheBloke")
	fmt.Println()
}

func handleQuantization() {
	fmt.Println()
	fmt.Println("Quantization Levels")
	fmt.Println()
	fmt.Println("Quantization reduces model size by using fewer bits per weight.")
	fmt.Println("Lower bits = smaller file + faster loading - slight quality reduction")
	fmt.Println()

	// Show all quantization levels in order of quality
	quantLevels := []string{
		"Q2_K", "Q3_K_S", "Q3_K_M", "Q3_K_L",
		"Q4_0", "Q4_K_S", "Q4_K_M",
		"Q5_0", "Q5_K_S", "Q5_K_M",
		"Q6_K", "Q8_0",
	}

	for _, quant := range quantLevels {
		info := models.GetQuantizationInfo(quant)
		marker := "   "
		if quant == "Q4_K_M" || quant == "Q5_K_M" {
			marker = " ★ "
		}

		fmt.Printf("%s %s · %s\n", marker, info.Name, info.QualityLevel)
		fmt.Printf("     %.1f bits/weight · %s\n", info.BitsPerWeight, info.Description)
		fmt.Printf("     %s\n", info.UseCases)
		fmt.Println()
	}

	fmt.Println("Recommendations")
	fmt.Println()
	fmt.Println("  ★ Most users:       Q4_K_M  Best quality/size balance")
	fmt.Println("  ★ Production:       Q5_K_M  Higher quality (~25% larger)")
	fmt.Println("    Limited RAM:      Q3_K_M  Acceptable quality (3-4 GB)")
	fmt.Println("    Research:         Q8_0    Near-original quality")
	fmt.Println()
	fmt.Println("Size comparison (7B parameter model):")
	fmt.Println("  Q4_K_M: ~4.0 GB  |  Q5_K_M: ~4.8 GB  |  Q8_0: ~7.2 GB")
	fmt.Println()
}

func printHelp() {
	printDivider()
	fmt.Println()

	printSection("Usage")
	fmt.Printf("  %soffgrid%s [command]\n", colorBold, colorReset)
	fmt.Println()

	printSection("Commands")
	fmt.Printf("  %sserve%s              Start HTTP inference server (default)\n", brandPrimary, colorReset)
	fmt.Printf("  %ssearch%s <query>     Search HuggingFace for models\n", brandPrimary, colorReset)
	fmt.Printf("  %sdownload%s <id>      Download a model from catalog\n", brandPrimary, colorReset)
	fmt.Printf("  %sdownload-hf%s <id>   Download from HuggingFace Hub\n", brandPrimary, colorReset)
	fmt.Printf("  %srun%s <model>        Interactive chat with a model\n", brandPrimary, colorReset)
	fmt.Printf("  %simport%s <path>      Import model(s) from USB/SD card\n", brandPrimary, colorReset)
	fmt.Printf("  %sexport%s <id> <path> Export a model to USB/SD card\n", brandPrimary, colorReset)
	fmt.Printf("  %sremove%s <id>        Remove an installed model\n", brandPrimary, colorReset)
	fmt.Printf("  %slist%s               List installed models\n", brandPrimary, colorReset)
	fmt.Printf("  %scatalog%s            Show available models\n", brandPrimary, colorReset)
	fmt.Printf("  %sbenchmark%s <id>     Benchmark model performance\n", brandPrimary, colorReset)
	fmt.Printf("  %squantization%s       Explain quantization levels\n", brandPrimary, colorReset)
	fmt.Printf("  %sconfig%s <action>    Manage configuration (init, show, validate)\n", brandPrimary, colorReset)
	fmt.Printf("  %sinfo%s               Show system information\n", brandPrimary, colorReset)
	fmt.Printf("  %shelp%s               Show this help\n", brandPrimary, colorReset)
	fmt.Println()

	printSection("Examples")
	fmt.Printf("  %s$%s offgrid search llama --author TheBloke\n", brandMuted, colorReset)
	fmt.Printf("  %s$%s offgrid download tinyllama-1.1b-chat\n", brandMuted, colorReset)
	fmt.Printf("  %s$%s offgrid download-hf TheBloke/Llama-2-7B-Chat-GGUF\n", brandMuted, colorReset)
	fmt.Printf("  %s$%s offgrid run tinyllama-1.1b-chat.Q4_K_M\n", brandMuted, colorReset)
	fmt.Printf("  %s$%s offgrid import /media/usb\n", brandMuted, colorReset)
	fmt.Printf("  %s$%s offgrid benchmark tinyllama-1.1b-chat.Q4_K_M\n", brandMuted, colorReset)
	fmt.Println()

	printSection("Environment Variables")
	printItem("OFFGRID_CONFIG", "Configuration file path (YAML/JSON)")
	printItem("OFFGRID_PORT", "Server port (default: 11611)")
	printItem("OFFGRID_MODELS_DIR", "Models directory")
	printItem("OFFGRID_NUM_THREADS", "CPU threads")
	fmt.Println()

	printDivider()
	fmt.Println()
}

func handleInfo() {
	cfg := config.LoadConfig()
	registry := models.NewRegistry(cfg.ModelsDir)

	if err := registry.ScanModels(); err != nil {
		fmt.Fprintf(os.Stderr, "  ✗ Model scan error: %v\n", err)
	}

	fmt.Println()
	fmt.Println("OffGrid LLM v0.1.0-alpha")
	fmt.Println()

	// Configuration
	fmt.Println("Configuration")
	fmt.Printf("  Port:        %d\n", cfg.ServerPort)
	fmt.Printf("  Models:      %s\n", cfg.ModelsDir)
	fmt.Printf("  Context:     %d tokens\n", cfg.MaxContextSize)
	fmt.Printf("  Threads:     %d\n", cfg.NumThreads)
	fmt.Printf("  Memory:      %d MB\n", cfg.MaxMemoryMB)
	if cfg.EnableP2P {
		fmt.Printf("  P2P:         enabled (port %d)\n", cfg.P2PPort)
	}
	fmt.Println()

	// Installed Models
	modelList := registry.ListModels()
	fmt.Printf("Installed Models (%d)\n", len(modelList))
	if len(modelList) > 0 {
		for _, model := range modelList {
			meta, err := registry.GetModel(model.ID)
			if err == nil {
				status := "idle"
				if meta.IsLoaded {
					status = "loaded"
				}
				fmt.Printf("  • %s", model.ID)
				if meta.Size > 0 {
					fmt.Printf(" · %s", formatBytes(meta.Size))
				}
				if meta.Quantization != "" && meta.Quantization != "unknown" {
					fmt.Printf(" · %s", meta.Quantization)
				}
				fmt.Printf(" (%s)", status)
				fmt.Println()
			}
		}
	} else {
		fmt.Println("  No models installed")
	}
	fmt.Println()

	// Available Models
	catalog := models.DefaultCatalog()
	fmt.Printf("Available Models (%d)\n", len(catalog.Models))
	recommended := 0
	for _, entry := range catalog.Models {
		if entry.Recommended {
			recommended++
		}
	}
	fmt.Printf("  %d recommended\n", recommended)
	fmt.Println()

	// Quick Start
	if len(modelList) == 0 {
		fmt.Println("Quick Start")
		fmt.Println("  1. Download a model:  offgrid download tinyllama-1.1b-chat")
		fmt.Println("  2. Start server:      offgrid")
		fmt.Println("  3. Test endpoint:     curl http://localhost:11611/health")
	} else {
		fmt.Println("Server")
		fmt.Println("  Start:      offgrid")
		fmt.Printf("  Endpoint:   http://localhost:%d\n", cfg.ServerPort)
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

	fmt.Printf("\n%s%s%s Searching HuggingFace Hub%s\n", brandPrimary, iconSearch, colorBold, colorReset)
	printDivider()
	fmt.Println()

	hf := models.NewHuggingFaceClient()
	results, err := hf.SearchModels(filters)
	if err != nil {
		printError(fmt.Sprintf("Search failed: %v", err))
		fmt.Println()
		os.Exit(1)
	}

	if len(results) == 0 {
		printWarning("No models found matching your criteria")
		fmt.Println()
		printInfo("Try broadening your search or adjusting filters")
		fmt.Println()
		return
	}

	fmt.Printf("%s%d%s models found\n\n", colorBold, len(results), colorReset)

	for i, result := range results {
		model := result.Model

		// Model name with number
		fmt.Printf("%s%2d%s %s%s%s%s\n",
			brandMuted, i+1, colorReset,
			brandPrimary, iconModel, colorReset,
			colorBold+model.ID+colorReset)

		// Stats line
		fmt.Printf("     %s%s%s %s  %s❤%s %s",
			brandAccent, iconDownload, colorReset, formatNumber(model.Downloads),
			brandError, colorReset, formatNumber(int64(model.Likes)))

		if result.BestVariant != nil {
			if result.BestVariant.SizeGB > 0 {
				fmt.Printf("  %s%s%s Recommended: %s%s%s (%.1f GB)\n",
					brandMuted, boxV, colorReset,
					brandSuccess, result.BestVariant.Quantization, colorReset,
					result.BestVariant.SizeGB)
			} else {
				fmt.Printf("  %s%s%s Recommended: %s%s%s\n",
					brandMuted, boxV, colorReset,
					brandSuccess, result.BestVariant.Quantization, colorReset)
			}
		} else {
			fmt.Println()
		}

		// Show available variants
		if len(result.GGUFFiles) > 0 {
			fmt.Printf("     %sVariants:%s ", brandMuted, colorReset)
			shown := 0
			for _, file := range result.GGUFFiles {
				if shown >= 5 {
					fmt.Printf("%s... (+%d more)%s", brandMuted, len(result.GGUFFiles)-shown, colorReset)
					break
				}
				if shown > 0 {
					fmt.Printf("%s,%s ", brandMuted, colorReset)
				}
				fmt.Printf("%s", file.Quantization)
				shown++
			}
			fmt.Println()
		}

		// Download command
		if result.BestVariant != nil {
			fmt.Printf("     %s%s%s %soffgrid download-hf %s --file %s%s\n",
				brandPrimary, iconArrow, colorReset,
				brandMuted, model.ID, result.BestVariant.Filename, colorReset)
		}

		if i < len(results)-1 {
			fmt.Println()
		}
	}

	fmt.Println()
	printDivider()
	printInfo("Use 'offgrid download-hf <model-id> --file <filename>' to download")
	fmt.Println()
}

func handleDownloadHF(args []string) {
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
	var lastProgress int64

	err = hf.DownloadGGUF(modelID, selectedFile.Filename, destPath, func(current, total int64) {
		percent := float64(current) / float64(total) * 100
		speed := float64(current-lastProgress) / time.Since(startTime).Seconds() / (1024 * 1024)

		fmt.Printf("\r  ⏬ Progress: %.1f%% (%.1f / %.1f GB) · %.1f MB/s  ",
			percent,
			float64(current)/(1024*1024*1024),
			float64(total)/(1024*1024*1024),
			speed)

		lastProgress = current
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

	printInfo("Note: llama-server currently loads a fixed model")
	printInfo("To use this model, restart llama-server:")
	printItem("Restart", "sudo systemctl restart llama-server")
	fmt.Println()

	fmt.Println("Run it:")
	fmt.Printf("  offgrid run %s\n", selectedFile.Filename)
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
	if len(args) < 1 {
		printDivider()
		fmt.Println()
		printSection("Usage")
		fmt.Printf("  %soffgrid run%s <model-name>\n", colorBold, colorReset)
		fmt.Println()
		printSection("Description")
		fmt.Println("  Start an interactive chat session with a model")
		fmt.Println()
		printSection("Examples")
		fmt.Printf("  %s$%s offgrid run tinyllama-1.1b-chat.Q4_K_M\n", brandMuted, colorReset)
		fmt.Printf("  %s$%s offgrid run llama-2-7b-chat.Q4_K_M\n", brandMuted, colorReset)
		fmt.Println()
		printInfo("Use 'offgrid list' to see available models")
		fmt.Println()
		printDivider()
		fmt.Println()
		os.Exit(1)
	}

	modelName := args[0]

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

	printDivider()
	fmt.Println()
	printSection(fmt.Sprintf("Interactive Chat · %s", modelName))
	fmt.Println()
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
