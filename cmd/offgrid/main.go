package main

import (
	"fmt"
	"os"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/server"
)

func main() {
	fmt.Println("🌐 OffGrid LLM - AI for Edge & Offline Environments")
	fmt.Println("Version: 0.1.0-alpha")
	fmt.Println()

	// Parse command
	if len(os.Args) > 1 {
		command := os.Args[1]

		switch command {
		case "download":
			handleDownload(os.Args[2:])
			return
		case "list":
			handleList(os.Args[2:])
			return
		case "catalog":
			handleCatalog()
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

	// Start the HTTP server (default command)
	srv := server.New()
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
		fmt.Printf("\r📥 Downloading %s (%s): %.1f%% [%s] %.2f MB/s",
			p.ModelID, p.Variant, p.Percent,
			formatBytes(p.BytesDone), float64(p.Speed)/(1024*1024))

		if p.Status == "complete" {
			fmt.Println("\n✅ Download complete!")
		} else if p.Status == "verifying" {
			fmt.Print("\n🔍 Verifying...")
		}
	})

	fmt.Printf("Downloading %s (%s)...\n", modelID, quantization)

	if err := downloader.Download(modelID, quantization); err != nil {
		fmt.Fprintf(os.Stderr, "\n❌ Download failed: %v\n", err)
		os.Exit(1)
	}
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
		fmt.Println("No models found in", cfg.ModelsDir)
		fmt.Println("\nDownload models with: offgrid download <model-id>")
		fmt.Println("See available models: offgrid catalog")
		return
	}

	fmt.Printf("📦 Found %d model(s) in %s:\n\n", len(modelList), cfg.ModelsDir)
	for _, model := range modelList {
		fmt.Printf("  • %s\n", model.ID)
	}
}

func handleCatalog() {
	catalog := models.DefaultCatalog()

	fmt.Println("📚 Available Models:\n")

	for _, entry := range catalog.Models {
		recommended := ""
		if entry.Recommended {
			recommended = " ⭐"
		}

		fmt.Printf("  %s%s\n", entry.ID, recommended)
		fmt.Printf("    Name: %s\n", entry.Name)
		fmt.Printf("    Size: %s parameters\n", entry.Parameters)
		fmt.Printf("    RAM:  %d GB minimum\n", entry.MinRAM)
		fmt.Printf("    Info: %s\n", entry.Description)
		fmt.Printf("    Variants: ")

		for i, v := range entry.Variants {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Printf("%s (%.1f GB)", v.Quantization, float64(v.Size)/(1024*1024*1024))
		}
		fmt.Println("\n")
	}

	fmt.Println("Download: offgrid download <model-id> [quantization]")
	fmt.Println("Example:  offgrid download tinyllama-1.1b-chat Q4_K_M")
}

func printHelp() {
	fmt.Println("Usage: offgrid [command]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  serve, server    Start the HTTP server (default)")
	fmt.Println("  download <id>    Download a model from catalog")
	fmt.Println("  list             List installed models")
	fmt.Println("  catalog          Show available models in catalog")
	fmt.Println("  help             Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  offgrid                                    # Start server")
	fmt.Println("  offgrid catalog                            # Browse models")
	fmt.Println("  offgrid download tinyllama-1.1b-chat       # Download model")
	fmt.Println("  offgrid list                               # List local models")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  OFFGRID_PORT         Server port (default: 8080)")
	fmt.Println("  OFFGRID_MODELS_DIR   Models directory")
	fmt.Println("  OFFGRID_NUM_THREADS  CPU threads to use")
	fmt.Println()
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
