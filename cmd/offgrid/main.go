package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/server"
)

func main() {
	fmt.Println()
	fmt.Println("OFFGRID-LLM v0.1.0α")
	fmt.Println("Edge Inference Orchestrator")
	fmt.Println()

	// Parse command
	if len(os.Args) > 1 {
		command := os.Args[1]

		switch command {
		case "download":
			handleDownload(os.Args[2:])
			return
		case "import":
			handleImport(os.Args[2:])
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
		fmt.Fprintln(os.Stderr, "Usage: offgrid download <model-id> [quantization]")
		fmt.Fprintln(os.Stderr, "Example: offgrid download tinyllama-1.1b-chat Q4_K_M")
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
		fmt.Printf("\r  Downloading %s (%s): %.1f%% · %s · %.1f MB/s",
			p.ModelID, p.Variant, p.Percent,
			formatBytes(p.BytesDone), float64(p.Speed)/(1024*1024))

		if p.Status == "complete" {
			fmt.Println("\n  ✓ Download complete")
		} else if p.Status == "verifying" {
			fmt.Print("\n  Verifying checksum...")
		}
	})

	fmt.Printf("Downloading %s (%s)\n\n", modelID, quantization)

	if err := downloader.Download(modelID, quantization); err != nil {
		fmt.Fprintf(os.Stderr, "\n  ✗ Download failed: %v\n", err)
		os.Exit(1)
	}
}

func handleImport(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: offgrid import <usb-path> [model-file]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  offgrid import /media/usb                    # Import all .gguf files from USB")
		fmt.Fprintln(os.Stderr, "  offgrid import /media/usb/model.gguf         # Import specific file")
		fmt.Fprintln(os.Stderr, "  offgrid import D:\\                           # Windows USB drive")
		os.Exit(1)
	}

	cfg := config.LoadConfig()
	registry := models.NewRegistry(cfg.ModelsDir)
	importer := models.NewUSBImporter(cfg.ModelsDir, registry)

	usbPath := args[0]

	// Check if path is a specific file or directory
	info, err := os.Stat(usbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  ✗ Error: %v\n", err)
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
			fmt.Println("  No model files found")
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
	}
}

func getFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func handleList(args []string) {
	cfg := config.LoadConfig()
	registry := models.NewRegistry(cfg.ModelsDir)

	if err := registry.ScanModels(); err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning models: %v\n", err)
		os.Exit(1)
	}

	modelList := registry.ListModels()

	if len(modelList) == 0 {
		fmt.Println("No models installed in", cfg.ModelsDir)
		fmt.Println()
		fmt.Println("  Download a model:  offgrid download <model-id>")
		fmt.Println("  Browse catalog:    offgrid catalog")
		return
	}

	fmt.Printf("Models (%d)\n", len(modelList))
	fmt.Println()
	for _, model := range modelList {
		fmt.Printf("  • %s\n", model.ID)
	}
	fmt.Println()
}

func handleCatalog() {
	catalog := models.DefaultCatalog()

	fmt.Println()
	fmt.Println("Available Models")
	fmt.Println()

	for _, entry := range catalog.Models {
		recommended := ""
		if entry.Recommended {
			recommended = " (recommended)"
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

	fmt.Println("Usage:")
	fmt.Println("  offgrid download <model-id> [quantization]")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  offgrid download tinyllama-1.1b-chat Q4_K_M")
	fmt.Println("  offgrid quantization  # Learn about quantization levels")
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
	fmt.Println()
	fmt.Println("Usage")
	fmt.Println("  offgrid [command]")
	fmt.Println()
	fmt.Println("Commands")
	fmt.Println("  serve              Start HTTP inference server (default)")
	fmt.Println("  download <id>      Download a model from catalog")
	fmt.Println("  import <path>      Import model(s) from USB/SD card")
	fmt.Println("  list               List installed models")
	fmt.Println("  catalog            Show available models")
	fmt.Println("  quantization       Explain quantization levels")
	fmt.Println("  config <action>    Manage configuration (init, show, validate)")
	fmt.Println("  info               Show system information")
	fmt.Println("  help               Show this help")
	fmt.Println()
	fmt.Println("Examples")
	fmt.Println("  offgrid")
	fmt.Println("  offgrid catalog")
	fmt.Println("  offgrid download tinyllama-1.1b-chat")
	fmt.Println("  offgrid import /media/usb")
	fmt.Println("  offgrid config init")
	fmt.Println()
	fmt.Println("Environment Variables")
	fmt.Println("  OFFGRID_CONFIG       Configuration file path (YAML/JSON)")
	fmt.Println("  OFFGRID_PORT         Server port (default: 8080)")
	fmt.Println("  OFFGRID_MODELS_DIR   Models directory")
	fmt.Println("  OFFGRID_NUM_THREADS  CPU threads")
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
		fmt.Println("  3. Test endpoint:     curl http://localhost:8080/health")
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
