package cli

import (
	"context"
	"fmt"
)

func removeModelCommand(_ context.Context, args []string, env Environment) error {
	p := Presenter{Stdout: env.Stdout, Stderr: env.Stderr, JSON: env.Options.JSON, Style: env.Style}
	// Check for help flag first
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = []string{} // Trigger help display
	}

	if len(args) < 1 {
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %s◈ Remove Model%s\n", env.Style.Primary+env.Style.Bold, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "  %sRemove an installed model from your system%s\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sUsage%s  offgrid remove <model-id> [options]\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sOptions%s\n", env.Style.Primary, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-20s %sSkip confirmation prompt%s\n", "--yes, -y", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sExamples%s\n", env.Style.Primary, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %s$%s offgrid remove tinyllama-1.1b-chat.Q4_K_M\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %s$%s offgrid remove llama-2-7b-chat.Q5_K_M --yes\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sTip:%s Use %soffgrid list%s to see installed models\n", env.Style.Muted, env.Style.Reset, env.Style.Bold, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		return ReportedError{}
	}

	cfg, err := localConfig(env)
	if err != nil {
		return err
	}
	if env.ModelVerify.NewRegistry == nil || env.ModelTransfer.Remove == nil {
		return fmt.Errorf("local model transfer adapters unavailable")
	}
	registry := env.ModelVerify.NewRegistry(cfg.ModelsDir)

	if err := registry.ScanModels(); err != nil {
		fmt.Fprintf(env.Stderr, "  ✗ Error scanning models: %v\n", err)
		return ReportedError{}
	}

	modelID := args[0]
	var skipConfirm bool

	// Parse options
	for i := 1; i < len(args); i++ {
		if args[i] == "--yes" || args[i] == "-y" {
			skipConfirm = true
		}
	}

	// Check if model exists
	meta, err := registry.GetModel(modelID)
	if err != nil {
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "%s✗ Model not found: %s%s\n", env.Style.Error, modelID, env.Style.Reset)
		fmt.Fprintln(env.Stdout)

		// Show available models
		modelList := registry.ListModels()
		if len(modelList) > 0 {
			fmt.Fprintf(env.Stdout, "%sAvailable Models%s\n", env.Style.Primary+env.Style.Bold, env.Style.Reset)
			for _, m := range modelList {
				modelMeta, _ := registry.GetModel(m.ID)
				fmt.Fprintf(env.Stdout, "%s", m.ID)
				if modelMeta != nil && modelMeta.Size > 0 {
					fmt.Fprintf(env.Stdout, " · %s", FormatBytes(modelMeta.Size))
				}
				if modelMeta != nil && modelMeta.Quantization != "" {
					fmt.Fprintf(env.Stdout, " · %s", modelMeta.Quantization)
				}
				fmt.Fprintln(env.Stdout)
			}
			fmt.Fprintln(env.Stdout)
		} else {
			fmt.Fprintf(env.Stdout, "%sℹ No models installed%s\n", env.Style.Dim, env.Style.Reset)
			fmt.Fprintln(env.Stdout)
			fmt.Fprintln(env.Stdout, "Download models:")
			fmt.Fprintf(env.Stdout, "  %sFrom HuggingFace:%s offgrid download-hf <repo> --file <file>.gguf\n", env.Style.Secondary, env.Style.Reset)
			fmt.Fprintln(env.Stdout)
		}
		return ReportedError{}
	}

	// Confirm deletion
	fmt.Fprintln(env.Stdout)
	fmt.Fprintf(env.Stdout, "%sRemove Model%s\n", env.Style.Primary+env.Style.Bold, env.Style.Reset)
	fmt.Fprintln(env.Stdout, "")

	fmt.Fprintf(env.Stdout, "%sModel Information%s\n", env.Style.Primary, env.Style.Reset)
	fmt.Fprintf(env.Stdout, "Name: %s%s%s\n", env.Style.Bold, modelID, env.Style.Reset)
	if meta.Path != "" {
		fmt.Fprintf(env.Stdout, "Path: %s%s%s\n", env.Style.Muted, meta.Path, env.Style.Reset)
	}
	if meta.Size > 0 {
		fmt.Fprintf(env.Stdout, "Size: %s%s%s will be freed\n", env.Style.Success, FormatBytes(meta.Size), env.Style.Reset)
	}
	fmt.Fprintln(env.Stdout, "")

	if !skipConfirm {
		fmt.Fprintf(env.Stdout, "%s⚠  This action cannot be undone%s\n", env.Style.Error, env.Style.Reset)
		fmt.Fprintln(env.Stdout, "")
		fmt.Fprintf(env.Stdout, "%sContinue?%s (y/N): ", env.Style.Muted, env.Style.Reset)

		var response string
		fmt.Fscanln(env.Stdin, &response)

		if response != "y" && response != "Y" {
			fmt.Fprintln(env.Stdout)
			p.Info("Cancelled - model preserved")
			fmt.Fprintln(env.Stdout)
			return nil
		}
	} else {
		fmt.Fprintf(env.Stdout, "%sRemoving model (--yes flag)...%s\n", env.Style.Muted, env.Style.Reset)
		fmt.Fprintln(env.Stdout, "")
	}

	// Delete the model file
	if meta.Path != "" {
		if err := env.ModelTransfer.Remove(meta.Path); err != nil {
			fmt.Fprintln(env.Stdout)
			p.Error(fmt.Sprintf("Failed to remove file: %v", err))
			fmt.Fprintln(env.Stdout)
			return ReportedError{}
		}
	}

	fmt.Fprintln(env.Stdout)
	fmt.Fprintf(env.Stdout, "%s✓ Model removed: %s%s%s\n", env.Style.Success, env.Style.Primary, modelID, env.Style.Reset)

	// Rescan to update registry after file deletion
	if err := registry.ScanModels(); err != nil {
		fmt.Fprintln(env.Stdout)
		p.Warning(fmt.Sprintf("Failed to refresh model list: %v", err))
	}

	// Show remaining models
	remaining := registry.ListModels()
	fmt.Fprintf(env.Stdout, "%s%d model(s) remaining%s\n\n", env.Style.Muted, len(remaining), env.Style.Reset)
	return nil
}

func RunRemoveModelCommand(ctx context.Context, args []string, env Environment) error {
	return removeModelCommand(ctx, args, env)
}
