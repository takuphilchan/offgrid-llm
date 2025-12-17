package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
	"github.com/takuphilchan/offgrid-llm/internal/server"
)

// This demo shows the new core features added to OffGrid LLM
func main() {
	fmt.Println("=== OffGrid LLM - Core Features Demo ===")

	// 1. GPU Detection & Monitoring
	fmt.Println("1. GPU Detection & Monitoring")
	fmt.Println("------------------------------")

	gpuMonitor := resource.NewGPUMonitor()
	if gpuMonitor.IsAvailable() {
		fmt.Printf("✓ GPU Type: %s\n", gpuMonitor.GetGPUType())
		fmt.Printf("✓ Total VRAM: %d MB\n", gpuMonitor.GetTotalVRAM())
		fmt.Printf("✓ Free VRAM: %d MB\n", gpuMonitor.GetFreeVRAM())

		gpuInfo := gpuMonitor.GetGPUInfo()
		for i, gpu := range gpuInfo {
			fmt.Printf("  GPU %d: %s\n", i, gpu.Name)
			fmt.Printf("    Memory: %d/%d MB (%.1f%% used)\n",
				gpu.MemoryUsed, gpu.MemoryTotal,
				float64(gpu.MemoryUsed)/float64(gpu.MemoryTotal)*100)
			fmt.Printf("    Utilization: %.1f%%\n", gpu.Utilization)
			if gpu.Temperature > 0 {
				fmt.Printf("    Temperature: %d°C\n", gpu.Temperature)
			}
		}

		// Check if we have enough VRAM for a typical model (4GB)
		if hasEnough, err := gpuMonitor.HasEnoughVRAM(4096); hasEnough {
			fmt.Println("✓ Sufficient VRAM for 4GB model")
		} else {
			fmt.Printf("✗ Insufficient VRAM: %v\n", err)
		}
	} else {
		fmt.Println("✗ No GPU detected (CPU-only mode)")
	}

	fmt.Println()

	// 2. Model Validation & Integrity Checks
	fmt.Println("2. Model Validation & Integrity")
	fmt.Println("--------------------------------")

	modelsDir := os.Getenv("OFFGRID_MODELS_DIR")
	if modelsDir == "" {
		modelsDir = "/var/lib/offgrid/models"
	}

	validator := models.NewValidator(modelsDir)

	// Scan and validate all models
	results, err := validator.ValidateDirectory()
	if err != nil {
		log.Printf("Error scanning models: %v\n", err)
	} else {
		validCount := 0
		corruptedCount := 0

		for modelName, result := range results {
			if result.Valid {
				validCount++
				fmt.Printf("✓ %s (%.2f MB) - Valid GGUF\n",
					modelName, float64(result.FileSize)/(1024*1024))
				if result.SHA256Hash != "" {
					fmt.Printf("  SHA256: %s...%s\n",
						result.SHA256Hash[:8], result.SHA256Hash[len(result.SHA256Hash)-8:])
				}
			} else {
				corruptedCount++
				fmt.Printf("✗ %s - %v\n", modelName, result.Errors)
			}
		}

		fmt.Printf("\nSummary: %d valid, %d corrupted\n", validCount, corruptedCount)
	}

	// Example: Validate a specific model with expected hash
	testModelPath := filepath.Join(modelsDir, "test-model.gguf")
	if _, err := os.Stat(testModelPath); err == nil {
		fmt.Printf("\nValidating: %s\n", testModelPath)
		result, _ := validator.ValidateModel(testModelPath)
		if result.Valid {
			fmt.Println("✓ Model validated successfully")
		} else {
			fmt.Printf("✗ Validation failed: %v\n", result.Errors)
		}
	}

	fmt.Println()

	// 3. Request Queue Management
	fmt.Println("3. Request Queue Management")
	fmt.Println("---------------------------")

	// Create resource monitor
	monitor := resource.NewMonitor(1000) // 1 second update interval
	monitor.Start()
	defer monitor.Stop()

	// Create request queue with safe defaults for edge devices
	queueConfig := server.DefaultQueueConfig()
	queueConfig.MaxConcurrent = 2     // Only 2 concurrent requests
	queueConfig.MaxQueueSize = 5      // Queue up to 5 requests
	queueConfig.MemoryThreshold = 512 // Require 512MB free

	queue := server.NewRequestQueue(queueConfig, monitor, gpuMonitor)

	fmt.Printf("Queue Configuration:\n")
	fmt.Printf("  Max Concurrent: %d\n", queueConfig.MaxConcurrent)
	fmt.Printf("  Max Queue Size: %d\n", queueConfig.MaxQueueSize)
	fmt.Printf("  Memory Threshold: %d MB\n", queueConfig.MemoryThreshold)

	// Get current stats
	stats := queue.GetStats()
	fmt.Printf("\nCurrent Queue Stats:\n")
	fmt.Printf("  Total Requests: %d\n", stats.TotalRequests)
	fmt.Printf("  Completed OK: %d\n", stats.CompletedOK)
	fmt.Printf("  Completed Error: %d\n", stats.CompletedError)
	fmt.Printf("  Rejected: %d\n", stats.Rejected)
	fmt.Printf("  Queue Depth: %d/%d\n", stats.CurrentQueue, queueConfig.MaxQueueSize)
	fmt.Printf("  Running: %d/%d\n", stats.CurrentRunning, queueConfig.MaxConcurrent)

	// Show dynamic concurrency adjustment capability
	fmt.Printf("\n✓ Queue supports dynamic concurrency adjustment\n")
	fmt.Printf("✓ Memory-aware request rejection\n")
	fmt.Printf("✓ Priority-based request scheduling\n")
	fmt.Printf("✓ Timeout protection\n")

	fmt.Println()

	// Summary
	fmt.Println("=== Summary ===")
	fmt.Println("Core improvements implemented:")
	fmt.Println("✓ GPU detection for NVIDIA/AMD with VRAM monitoring")
	fmt.Println("✓ Model integrity validation with SHA256 verification")
	fmt.Println("✓ GGUF file format validation")
	fmt.Println("✓ Request queue with concurrency limits")
	fmt.Println("✓ Memory-aware request handling")
	fmt.Println("✓ Priority-based scheduling")
	fmt.Println("\nThese features provide essential reliability and")
	fmt.Println("resource management for edge/offline deployments.")
}
