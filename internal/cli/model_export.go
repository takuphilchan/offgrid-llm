package cli

import (
	"context"
	"fmt"
	"path/filepath"
)

func exportModelCommand(_ context.Context, args []string, env Environment) error {
	// Check for help flag
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = []string{} // Trigger help display
	}

	if len(args) < 2 {
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %s◈ Export Model%s\n", env.Style.Primary+env.Style.Bold, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "  %sExport a model to USB/SD card or external storage%s\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sUsage%s  offgrid export <model-id> <destination>\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sExamples%s\n", env.Style.Primary, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %s$%s offgrid export tinyllama-1.1b-chat.Q4_K_M /media/usb\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %s$%s offgrid export llama-2-7b-chat.Q5_K_M D:\\backup\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sTip:%s Use %soffgrid list%s to see available models\n", env.Style.Muted, env.Style.Reset, env.Style.Bold, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		return ReportedError{}
	}

	cfg, err := localConfig(env)
	if err != nil {
		return err
	}
	if env.ModelVerify.NewRegistry == nil || env.ModelTransfer.MkdirAll == nil || env.ModelTransfer.Open == nil || env.ModelTransfer.Create == nil {
		return fmt.Errorf("local model export adapters unavailable")
	}
	registry := env.ModelVerify.NewRegistry(cfg.ModelsDir)

	if err := registry.ScanModels(); err != nil {
		fmt.Fprintf(env.Stderr, "  ✗ Error scanning models: %v\n", err)
		return ReportedError{}
	}

	modelID := args[0]
	destPath := args[1]

	// Check if model exists
	meta, err := registry.GetModel(modelID)
	if err != nil {
		fmt.Fprintf(env.Stderr, "✗ Model not found: %s\n\n", modelID)

		// Show available models
		modelList := registry.ListModels()
		if len(modelList) > 0 {
			fmt.Fprintln(env.Stderr, "Available models:")
			for _, m := range modelList {
				fmt.Fprintf(env.Stderr, "  • %s\n", m.ID)
			}
			fmt.Fprintln(env.Stderr, "")
		} else {
			fmt.Fprintln(env.Stderr, "No models installed. Use 'offgrid download' to add models.")
			fmt.Fprintln(env.Stderr, "")
		}
		return ReportedError{}
	}

	if meta.Path == "" {
		fmt.Fprintf(env.Stderr, "✗ Model path not found for: %s\n\n", modelID)
		return ReportedError{}
	}

	// Ensure destination directory exists
	if err := env.ModelTransfer.MkdirAll(destPath, 0755); err != nil {
		fmt.Fprintf(env.Stderr, "✗ Cannot create destination directory: %v\n\n", err)
		fmt.Fprintf(env.Stderr, "Make sure:\n")
		fmt.Fprintf(env.Stderr, "  • The USB/SD card is mounted\n")
		fmt.Fprintf(env.Stderr, "  • You have write permissions\n")
		fmt.Fprintf(env.Stderr, "  • The device has enough space\n\n")
		return ReportedError{}
	}

	// Construct destination file path
	fileName := filepath.Base(meta.Path)
	destFile := filepath.Join(destPath, fileName)

	fmt.Fprintf(env.Stdout, "%sExport Model%s\n", env.Style.Primary+env.Style.Bold, env.Style.Reset)
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintf(env.Stdout, "Model: %s%s%s\n", env.Style.Primary, modelID, env.Style.Reset)
	fmt.Fprintf(env.Stdout, "From:  %s%s%s\n", env.Style.Muted, meta.Path, env.Style.Reset)
	fmt.Fprintf(env.Stdout, "To:    %s%s%s\n", env.Style.Muted, destFile, env.Style.Reset)
	fmt.Fprintf(env.Stdout, "Size:  %s\n", FormatBytes(meta.Size))
	fmt.Fprintln(env.Stdout, "")

	// Copy file
	sourceFile, err := env.ModelTransfer.Open(meta.Path)
	if err != nil {
		fmt.Fprintf(env.Stderr, "  ✗ Failed to open source: %v\n", err)
		return ReportedError{}
	}
	defer sourceFile.Close()

	destFileHandle, err := env.ModelTransfer.Create(destFile)
	if err != nil {
		fmt.Fprintf(env.Stderr, "  ✗ Failed to create destination: %v\n", err)
		return ReportedError{}
	}
	defer destFileHandle.Close()

	// Copy with progress
	buffer := make([]byte, 1024*1024) // 1MB buffer
	var totalCopied int64

	for {
		n, err := sourceFile.Read(buffer)
		if n > 0 {
			if _, err := destFileHandle.Write(buffer[:n]); err != nil {
				fmt.Fprintf(env.Stderr, "\n  ✗ Write error: %v\n", err)
				return ReportedError{}
			}
			totalCopied += int64(n)
			percent := float64(totalCopied) / float64(meta.Size) * 100
			fmt.Fprintf(env.Stdout, "\r  Progress: %.1f%% · %s / %s",
				percent, FormatBytes(totalCopied), FormatBytes(meta.Size))
		}
		if err != nil {
			break
		}
	}

	fmt.Fprintf(env.Stdout, "\n\n✓ Export complete\n")
	fmt.Fprintf(env.Stdout, "  Location: %s\n\n", destFile)
	return nil
}

func RunExportModelCommand(ctx context.Context, args []string, env Environment) error {
	return exportModelCommand(ctx, args, env)
}
