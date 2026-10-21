package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

type QuantizeDependencies struct {
	Stat     func(string) (os.FileInfo, error)
	LookPath func(string) (string, error)
	Run      func(context.Context, string, string, string, string, io.Writer, io.Writer) error
	Now      func() time.Time
}

func quantizationCommand(_ context.Context, _ []string, env Environment) error {
	fmt.Fprintln(env.Stdout)
	fmt.Fprintf(env.Stdout, "  %s◈ Quantization Guide%s\n", env.Style.Primary+env.Style.Bold, env.Style.Reset)
	fmt.Fprintf(env.Stdout, "  %sLower bits = smaller size + faster speed - slight quality loss%s\n", env.Style.Dim, env.Style.Reset)
	fmt.Fprintln(env.Stdout)

	// Group by quality tier
	tiers := []struct {
		name   string
		quants []string
	}{
		{"Compact (2-3 bit)", []string{"Q2_K", "Q3_K_S", "Q3_K_M"}},
		{"Balanced (4 bit) — Recommended", []string{"Q4_K_S", "Q4_K_M"}},
		{"High Quality (5-6 bit)", []string{"Q5_K_S", "Q5_K_M", "Q6_K"}},
		{"Maximum Quality (8 bit)", []string{"Q8_0"}},
	}

	for _, tier := range tiers {
		fmt.Fprintf(env.Stdout, "  %s%s%s\n", env.Style.Primary, tier.name, env.Style.Reset)
		for _, quant := range tier.quants {
			info := models.GetQuantizationInfo(quant)
			star := "  "
			starColor := ""
			if quant == "Q4_K_M" || quant == "Q5_K_M" {
				star = "★ "
				starColor = env.Style.Success
			}

			fmt.Fprintf(env.Stdout, "    %s%s%s%-8s%s %.1f bits · %s%s\n",
				starColor, star, env.Style.Reset,
				info.Name, env.Style.Muted,
				info.BitsPerWeight,
				info.Description, env.Style.Reset)
		}
		fmt.Fprintln(env.Stdout)
	}

	fmt.Fprintf(env.Stdout, "  %sRecommendations%s\n", env.Style.Primary, env.Style.Reset)
	fmt.Fprintf(env.Stdout, "    %s★%s Q4_K_M   %sBest for most users (4.0 GB for 7B)%s\n", env.Style.Success, env.Style.Reset, env.Style.Dim, env.Style.Reset)
	fmt.Fprintf(env.Stdout, "    %s★%s Q5_K_M   %sProduction quality (4.8 GB for 7B)%s\n", env.Style.Success, env.Style.Reset, env.Style.Dim, env.Style.Reset)
	fmt.Fprintf(env.Stdout, "      Q3_K_M   %sLimited RAM (3.0 GB for 7B)%s\n", env.Style.Dim, env.Style.Reset)
	fmt.Fprintf(env.Stdout, "      Q8_0     %sMaximum quality (7.2 GB for 7B)%s\n", env.Style.Dim, env.Style.Reset)
	fmt.Fprintln(env.Stdout)
	return nil
}

func quantizeCommand(ctx context.Context, args []string, env Environment) error {
	p := Presenter{Stdout: env.Stdout, Stderr: env.Stderr, JSON: env.Options.JSON, Style: env.Style}
	if len(args) < 2 {
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %s◈ Quantize Model%s\n", env.Style.Primary+env.Style.Bold, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "  %sConvert a model to a different precision level%s\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sUsage%s  offgrid quantize <model-id> <quant> [options]\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sQuantization Types%s\n", env.Style.Primary, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    Q2_K     %s2-bit (smallest, lowest quality)%s\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    Q3_K_M   %s3-bit medium%s\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %s★%s Q4_K_M %s4-bit medium (recommended)%s\n", env.Style.Success, env.Style.Reset, env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %s★%s Q5_K_M %s5-bit medium (high quality)%s\n", env.Style.Success, env.Style.Reset, env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    Q6_K     %s6-bit%s\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    Q8_0     %s8-bit (largest, highest quality)%s\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sOptions%s\n", env.Style.Primary, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-20s %sOutput model name%s\n", "--output <name>", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sExamples%s\n", env.Style.Primary, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %s$%s offgrid quantize llama-2-7b.F16 Q4_K_M\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %s$%s offgrid quantize phi-2.F16 Q5_K_M --output phi-2-hq\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sTip:%s Use %soffgrid quantization%s for quality comparisons\n", env.Style.Muted, env.Style.Reset, env.Style.Bold, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		return ReportedError{}
	}

	cfg, err := localConfig(env)
	if err != nil {
		return err
	}
	if env.Quantize.Stat == nil || env.Quantize.LookPath == nil || env.Quantize.Run == nil || env.Quantize.Now == nil {
		return fmt.Errorf("quantization process adapters unavailable")
	}
	if env.ModelVerify.NewRegistry == nil {
		return fmt.Errorf("local model registry unavailable")
	}
	registry := env.ModelVerify.NewRegistry(cfg.ModelsDir)

	if err := registry.ScanModels(); err != nil {
		p.Error(fmt.Sprintf("Error scanning models: %v", err))
		return ReportedError{}
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
		p.Error(fmt.Sprintf("Model not found: %s", modelID))
		fmt.Fprintln(env.Stdout)
		modelList := registry.ListModels()
		if len(modelList) > 0 {
			fmt.Fprintln(env.Stdout, "Available models:")
			for _, m := range modelList {
				fmt.Fprintf(env.Stdout, "  • %s\n", m.ID)
			}
			fmt.Fprintln(env.Stdout)
		}
		return ReportedError{}
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
		p.Error(fmt.Sprintf("Invalid quantization type: %s", targetQuant))
		fmt.Fprintf(env.Stdout, "\n%sValid types:%s Q2_K, Q3_K_S, Q3_K_M, Q4_K_S, Q4_K_M, Q5_K_S, Q5_K_M, Q6_K, Q8_0\n\n", env.Style.Muted, env.Style.Reset)
		return ReportedError{}
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
	if _, err := env.Quantize.Stat(outputPath); err == nil {
		p.Error(fmt.Sprintf("Output file already exists: %s", outputName+".gguf"))
		fmt.Fprintf(env.Stdout, "\n%sUse --output to specify a different name%s\n\n", env.Style.Muted, env.Style.Reset)
		return ReportedError{}
	}

	// Print quantization header
	fmt.Fprintf(env.Stdout, "\n%sQuantize Model%s\n", env.Style.Primary+env.Style.Bold, env.Style.Reset)
	fmt.Fprintln(env.Stdout, "")

	fmt.Fprintf(env.Stdout, "%sSource Model%s\n", env.Style.Primary, env.Style.Reset)
	fmt.Fprintf(env.Stdout, "Name: %s%s%s\n", env.Style.Bold, modelID, env.Style.Reset)
	fmt.Fprintf(env.Stdout, "Path: %s%s%s\n", env.Style.Muted, meta.Path, env.Style.Reset)
	fmt.Fprintf(env.Stdout, "Size: %s%s%s", env.Style.Primary, FormatBytes(meta.Size), env.Style.Reset)
	if meta.Quantization != "" {
		fmt.Fprintf(env.Stdout, " · %s%s%s", env.Style.Muted, meta.Quantization, env.Style.Reset)
	}
	fmt.Fprintln(env.Stdout)
	fmt.Fprintln(env.Stdout, "")

	quantInfo := models.GetQuantizationInfo(targetQuant)
	fmt.Fprintf(env.Stdout, "%sTarget Quantization%s\n", env.Style.Primary, env.Style.Reset)
	fmt.Fprintf(env.Stdout, "Type:    %s%s%s\n", env.Style.Primary, targetQuant, env.Style.Reset)
	fmt.Fprintf(env.Stdout, "Bits:    %.1f bits per weight\n", quantInfo.BitsPerWeight)
	fmt.Fprintf(env.Stdout, "Quality: %s\n", quantInfo.Description)
	fmt.Fprintf(env.Stdout, "Output:  %s%s.gguf%s\n", env.Style.Muted, outputName, env.Style.Reset)
	fmt.Fprintln(env.Stdout, "")

	// Check if llama-quantize is available
	llamaQuantize := "llama-quantize"
	if _, err := env.Quantize.LookPath(llamaQuantize); err != nil {
		p.Error("llama-quantize tool not found")
		fmt.Fprintf(env.Stdout, "\n%sInstall llama.cpp first:%s\n", env.Style.Muted, env.Style.Reset)
		fmt.Fprintln(env.Stdout, "  cd /tmp && git clone https://github.com/ggerganov/llama.cpp")
		fmt.Fprintln(env.Stdout, "  cd llama.cpp && make")
		fmt.Fprintln(env.Stdout, "  sudo cp llama-quantize /usr/local/bin/")
		fmt.Fprintln(env.Stdout)
		return ReportedError{}
	}

	// Run quantization
	fmt.Fprintln(env.Stdout, "Starting quantization...")
	fmt.Fprintln(env.Stdout)

	startTime := env.Quantize.Now()
	if err := env.Quantize.Run(ctx, llamaQuantize, meta.Path, outputPath, targetQuant, env.Stdout, env.Stderr); err != nil {
		fmt.Fprintln(env.Stdout)
		p.Error(fmt.Sprintf("Quantization failed: %v", err))
		return ReportedError{}
	}
	duration := env.Quantize.Now().Sub(startTime)

	// Get output file size
	outputStat, err := env.Quantize.Stat(outputPath)
	if err != nil {
		p.Error(fmt.Sprintf("Failed to stat output file: %v", err))
		return ReportedError{}
	}

	// Calculate compression ratio
	compressionRatio := float64(meta.Size) / float64(outputStat.Size())
	sizeSaved := meta.Size - outputStat.Size()

	// Display results
	fmt.Fprintln(env.Stdout)
	fmt.Fprintf(env.Stdout, "%sQuantization Complete%s\n", env.Style.Success+env.Style.Bold, env.Style.Reset)
	fmt.Fprintln(env.Stdout, "")

	fmt.Fprintf(env.Stdout, "%sResults%s\n", env.Style.Success, env.Style.Reset)
	fmt.Fprintf(env.Stdout, "Original:  %s\n", FormatBytes(meta.Size))
	fmt.Fprintf(env.Stdout, "Quantized: %s%s%s\n", env.Style.Success, FormatBytes(outputStat.Size()), env.Style.Reset)
	fmt.Fprintf(env.Stdout, "Saved:     %s%s%s (%.1fx smaller)\n", env.Style.Primary, FormatBytes(sizeSaved), env.Style.Reset, compressionRatio)
	fmt.Fprintf(env.Stdout, "Time:      %s\n", FormatDuration(duration))
	fmt.Fprintln(env.Stdout, "")

	fmt.Fprintf(env.Stdout, "%sModel Ready%s\n", env.Style.Success, env.Style.Reset)
	fmt.Fprintf(env.Stdout, "Name:     %s%s%s\n", env.Style.Primary, outputName, env.Style.Reset)
	fmt.Fprintf(env.Stdout, "Location: %s%s%s\n", env.Style.Muted, outputPath, env.Style.Reset)
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout)

	fmt.Fprintf(env.Stdout, "%sNext Steps%s\n", env.Style.Muted, env.Style.Reset)
	fmt.Fprintf(env.Stdout, "  Test model:  %soffgrid run %s%s\n", env.Style.Muted, outputName, env.Style.Reset)
	fmt.Fprintf(env.Stdout, "  Benchmark:   %soffgrid benchmark %s%s\n", env.Style.Muted, outputName, env.Style.Reset)
	fmt.Fprintln(env.Stdout)
	return nil
}

func RunQuantizationCommand(ctx context.Context, args []string, env Environment) error {
	return quantizationCommand(ctx, args, env)
}

func RunQuantizeCommand(ctx context.Context, args []string, env Environment) error {
	return quantizeCommand(ctx, args, env)
}

func FormatDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%dns", d.Nanoseconds())
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%.2fµs", float64(d.Microseconds()))
	}
	if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Milliseconds()))
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}
