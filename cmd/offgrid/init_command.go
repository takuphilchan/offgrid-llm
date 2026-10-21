package main

import (
	"bufio"
	"fmt"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
	"os"
	"path/filepath"
	"strings"
)

func handleInit(args []string) int {
	fmt.Println()

	// Check if already initialized
	cfg := config.LoadConfig()
	registry := models.NewRegistry(cfg.ModelsDir)
	registry.ScanModels()
	installedModels := registry.ListModels()

	if len(installedModels) > 0 {
		fmt.Printf("  %s%s Already initialized%s — %d model(s) installed\n\n", brandSuccess, iconCheck, colorReset, len(installedModels))
		fmt.Printf("  %sNext steps:%s\n", colorBold, colorReset)
		fmt.Printf("    %s$%s offgrid list          %s# Show installed models%s\n", brandMuted, colorReset, brandMuted, colorReset)
		fmt.Printf("    %s$%s offgrid run <model>   %s# Start chatting%s\n", brandMuted, colorReset, brandMuted, colorReset)
		fmt.Printf("    %s$%s offgrid doctor        %s# Check system health%s\n", brandMuted, colorReset, brandMuted, colorReset)
		fmt.Println()
		return 0
	}

	// Step 1: Detect system
	fmt.Printf("  %s[1/4]%s %sDetecting system...%s\n", brandPrimary, colorReset, colorBold, colorReset)
	res, err := resource.DetectResources()
	if err != nil {
		printError(fmt.Sprintf("Failed to detect system: %v", err))
		return 1
	}

	fmt.Printf("        %sOS%s       %s/%s\n", brandMuted, colorReset, res.OS, res.Arch)
	fmt.Printf("        %sCPU%s      %d cores\n", brandMuted, colorReset, res.CPUCores)
	fmt.Printf("        %sRAM%s      %s\n", brandMuted, colorReset, formatBytes(res.AvailableRAM*1024*1024))
	if res.GPUAvailable {
		fmt.Printf("        %sGPU%s      %s\n", brandMuted, colorReset, res.GPUName)
	} else {
		fmt.Printf("        %sGPU%s      None (CPU mode)\n", brandMuted, colorReset)
	}
	fmt.Println()

	// Step 2: Get recommendations
	fmt.Printf("  %s[2/4]%s %sFinding compatible models...%s\n", brandPrimary, colorReset, colorBold, colorReset)
	recommendations := res.RecommendedModels()

	primary := []resource.ModelRecommendation{}
	for _, rec := range recommendations {
		if rec.Priority == 1 {
			primary = append(primary, rec)
		}
	}

	if len(primary) == 0 {
		fmt.Println()
		printWarning("Your system has limited resources")
		fmt.Printf("\n  %sMinimum requirements:%s\n", colorBold, colorReset)
		fmt.Printf("    %s•%s 2GB RAM available\n", brandMuted, colorReset)
		fmt.Printf("    %s•%s 2GB free disk space\n", brandMuted, colorReset)
		fmt.Println()
		return 1
	}

	fmt.Println()
	fmt.Printf("        %sRecommended for your system:%s\n", colorBold, colorReset)
	maxModels := 3
	if len(primary) < maxModels {
		maxModels = len(primary)
	}
	for i := 0; i < maxModels; i++ {
		rec := primary[i]
		fmt.Printf("        %s%d.%s %s%s%s %s(%s · %.1fGB)%s\n",
			brandPrimary, i+1, colorReset,
			colorBold, rec.ModelID, colorReset,
			brandMuted, rec.Quantization, rec.SizeGB, colorReset)
		fmt.Printf("           %s%s%s\n", brandMuted, rec.Reason, colorReset)
	}
	fmt.Println()

	// Step 3: Choose model
	fmt.Printf("  %s[3/4]%s %sSelect a model%s\n", brandPrimary, colorReset, colorBold, colorReset)
	fmt.Printf("        Enter %s1-%d%s or %ss%s to skip: ", brandPrimary, maxModels, colorReset, brandMuted, colorReset)

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "s" || input == "S" || input == "" {
		fmt.Println()
		fmt.Printf("  %sSkipped.%s Search models anytime:\n", brandMuted, colorReset)
		fmt.Printf("    %s$%s offgrid search llama\n", brandMuted, colorReset)
		fmt.Printf("    %s$%s offgrid recommend\n", brandMuted, colorReset)
		fmt.Println()
		return 0
	}

	var choice int
	fmt.Sscanf(input, "%d", &choice)
	if choice < 1 || choice > len(primary) {
		printError("Invalid choice")
		return 1
	}

	selected := primary[choice-1]
	fmt.Println()

	// Step 4: Download
	fmt.Printf("  %s[4/4]%s %sDownloading %s...%s\n\n", brandPrimary, colorReset, colorBold, selected.ModelID, colorReset)

	// Search for the model on HuggingFace
	hf := models.NewHuggingFaceClient()
	filters := models.SearchFilter{
		Query:        selected.ModelID,
		Quantization: selected.Quantization,
		OnlyGGUF:     true,
		Limit:        1,
	}

	results, err := hf.SearchModels(filters)
	if err != nil || len(results) == 0 {
		printError("Failed to find model on HuggingFace")
		fmt.Println()
		fmt.Println("Search manually:")
		fmt.Printf("  offgrid search %s\n", selected.ModelID)
		fmt.Println()
		return 1
	}

	result := results[0]
	if result.BestVariant == nil {
		printError("No suitable variant found")
		return 1
	}

	var wizardProjector *models.ProjectorSource
	if projector, err := hf.ResolveProjectorSource(result.Model.ID, nil, result.BestVariant.Filename); err != nil {
		fmt.Println()
		printWarning(fmt.Sprintf("Vision adapter lookup failed: %v", err))
		fmt.Println()
	} else {
		wizardProjector = projector
	}
	if wizardProjector != nil {
		sourceLabel := wizardProjector.ModelID
		if wizardProjector.Source == "fallback" {
			sourceLabel = fmt.Sprintf("%s (fallback)", sourceLabel)
		}
		fmt.Printf("Vision adapter detected: %s%s%s", brandPrimary, wizardProjector.File.Filename, colorReset)
		if wizardProjector.File.Size > 0 {
			fmt.Printf(" · %s", formatBytes(wizardProjector.File.Size))
		}
		fmt.Println()
		fmt.Printf("Source: %s%s%s\n", brandMuted, sourceLabel, colorReset)
		if wizardProjector.Reason != "" {
			fmt.Printf("  %s%s%s\n", brandMuted, wizardProjector.Reason, colorReset)
		}
		fmt.Printf("%sIt will download automatically after the main model.%s\n\n", brandMuted, colorReset)
	}

	// Download the model
	destPath := filepath.Join(cfg.ModelsDir, result.BestVariant.Filename)

	fmt.Printf("Downloading from: %s\n", result.Model.ID)
	fmt.Printf("File: %s (%.1fGB)\n\n", result.BestVariant.Filename, result.BestVariant.SizeGB)

	err = hf.DownloadGGUF(result.Model.ID, result.BestVariant.Filename, destPath, func(current, total int64) {
		percent := float64(current) / float64(total) * 100
		fmt.Printf("\r  Progress: %.1f%% (%s / %s) · %.1f MB/s",
			percent,
			formatBytes(current),
			formatBytes(total),
			float64(current)/1024/1024)
	})

	if err != nil {
		fmt.Println()
		printHelpfulError(err, "Download")
		return 1
	}

	fmt.Println()
	fmt.Println()
	fmt.Printf("%s✓ Setup complete!%s\n\n", brandSuccess+colorBold, colorReset)

	modelName := strings.TrimSuffix(result.BestVariant.Filename, ".gguf")
	fmt.Println("Next steps:")
	fmt.Printf("  offgrid run %s         # Start chatting\n", modelName)
	fmt.Printf("  offgrid list                 # View installed models\n")
	fmt.Printf("  offgrid search llama         # Find more models\n")
	fmt.Println()

	if wizardProjector != nil {
		fmt.Printf("%sVision Adapter%s\n", brandPrimary+colorBold, colorReset)
		if err := downloadVisionProjector(hf, cfg.ModelsDir, wizardProjector); err != nil {
			printError(fmt.Sprintf("Vision adapter download failed: %v", err))
			return 1
		}
		fmt.Println()
	}
	return 0
}
