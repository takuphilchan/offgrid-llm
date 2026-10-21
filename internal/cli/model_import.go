package cli

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func importModelCommand(_ context.Context, args []string, env Environment) error {
	p := Presenter{Stdout: env.Stdout, Stderr: env.Stderr, JSON: env.Options.JSON, Style: env.Style}
	// Check for help flag
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = []string{} // Trigger help display
	}

	if len(args) < 1 {
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %s◈ Import Model%s\n", env.Style.Primary+env.Style.Bold, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "  %sImport GGUF models from USB/SD card or external storage%s\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sUsage%s  offgrid import <path> [model-file]\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sExamples%s\n", env.Style.Primary, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %s$%s offgrid import /media/usb              %s# Import all .gguf files%s\n", env.Style.Dim, env.Style.Reset, env.Style.Muted, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %s$%s offgrid import /media/usb/model.gguf  %s# Import specific file%s\n", env.Style.Dim, env.Style.Reset, env.Style.Muted, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %s$%s offgrid import D:\\models              %s# Windows path%s\n", env.Style.Dim, env.Style.Reset, env.Style.Muted, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sTip:%s Use %soffgrid list%s to view imported models\n", env.Style.Muted, env.Style.Reset, env.Style.Bold, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		return ReportedError{}
	}

	cfg, err := localConfig(env)
	if err != nil {
		return err
	}
	if env.ModelVerify.NewRegistry == nil || env.ModelTransfer.NewUSBImporter == nil || env.ModelTransfer.Stat == nil || env.ModelTransfer.RenderProgress == nil || env.ModelTransfer.ReloadServer == nil || env.ModelTransfer.ReloadModel == nil {
		return fmt.Errorf("local model import adapters unavailable")
	}
	registry := env.ModelVerify.NewRegistry(cfg.ModelsDir)
	importer := env.ModelTransfer.NewUSBImporter(cfg.ModelsDir, registry)

	usbPath := args[0]

	// Check if path is a specific file or directory
	info, err := env.ModelTransfer.Stat(usbPath)
	if err != nil {
		fmt.Fprintf(env.Stderr, "%s%s%s Path not found: %s\n\n", env.Style.Error, env.Style.Cross, env.Style.Reset, usbPath)
		fmt.Fprintf(env.Stderr, "Common USB/SD mount points:\n")
		fmt.Fprintf(env.Stderr, "  - Linux:   /media/<username>/<device>\n")
		fmt.Fprintf(env.Stderr, "  - macOS:   /Volumes/<device>\n")
		fmt.Fprintf(env.Stderr, "  - Windows: D:\\ E:\\ F:\\\n")
		fmt.Fprintf(env.Stderr, "\nTip: Use 'ls /media' or 'mount' to find your device\n\n")
		return ReportedError{}
	}

	if info.IsDir() {
		// Import all models from directory
		fmt.Fprintf(env.Stdout, "Scanning %s\n\n", usbPath)

		modelFiles, err := importer.ScanUSBDrive(usbPath)
		if err != nil {
			fmt.Fprintf(env.Stderr, "  %s%s%s Scan error: %v\n", env.Style.Error, env.Style.Cross, env.Style.Reset, err)
			return ReportedError{}
		}

		if len(modelFiles) == 0 {
			fmt.Fprintf(env.Stdout, "%s%s%s No GGUF model files found in %s\n", env.Style.Error, env.Style.Cross, env.Style.Reset, usbPath)
			fmt.Fprintln(env.Stdout)
			fmt.Fprintln(env.Stdout, "Looking for files matching: *.gguf")
			fmt.Fprintln(env.Stdout)
			fmt.Fprintln(env.Stdout, "Make sure your models:")
			fmt.Fprintln(env.Stdout, "  - Have .gguf file extension")
			fmt.Fprintln(env.Stdout, "  - Are in GGUF format (not safetensors or PyTorch)")
			fmt.Fprintln(env.Stdout, "  - Are readable (check permissions)")
			fmt.Fprintln(env.Stdout)
			return nil
		}

		fmt.Fprintf(env.Stdout, "Found %d model file(s):\n\n", len(modelFiles))
		for i, file := range modelFiles {
			modelID, quant := importer.GetModelInfo(filepath.Base(file))
			size := getTransferFileSize(file, env)
			fmt.Fprintf(env.Stdout, "  %d. %s (%s) - %s\n", i+1, modelID, quant, FormatBytes(size))
		}
		fmt.Fprintln(env.Stdout)

		// Import all
		fmt.Fprintln(env.Stdout, "Importing models...")
		fmt.Fprintln(env.Stdout)
		imported, err := importer.ImportAll(usbPath, func(p models.ImportProgress) {
			if p.Status == "copying" {
				env.ModelTransfer.RenderProgress(p.BytesDone, p.BytesTotal, 0, 25)
			} else if p.Status == "verifying" {
				fmt.Fprintf(env.Stdout, "\r  Verifying %s...          ", p.FileName)
			} else if p.Status == "complete" {
				fmt.Fprintf(env.Stdout, "\r  %s%s%s %s\n", env.Style.Success, env.Style.Check, env.Style.Reset, p.FileName)
			}
		})

		if err != nil {
			fmt.Fprintf(env.Stderr, "\n  %s%s%s Import failed: %v\n", env.Style.Error, env.Style.Cross, env.Style.Reset, err)
			return ReportedError{}
		}

		fmt.Fprintf(env.Stdout, "\n  %s%s%s Imported %d model(s) to %s\n", env.Style.Success, env.Style.Check, env.Style.Reset, imported, cfg.ModelsDir)

		// Reload llama-server with imported models
		if imported > 0 {
			if err := env.ModelTransfer.ReloadServer(); err != nil {
				fmt.Fprintln(env.Stdout)
				p.Warning(fmt.Sprintf("Could not auto-reload server: %v", err))
				fmt.Fprintln(env.Stdout)
				p.Info("Manually restart the server:")
				p.Item("Restart service", "sudo systemctl restart llama-server")
				fmt.Fprintln(env.Stdout)
			}
		}
	} else {
		// Import single file
		fmt.Fprintf(env.Stdout, "Importing %s\n\n", filepath.Base(usbPath))

		err := importer.ImportModel(usbPath, "", func(p models.ImportProgress) {
			if p.Status == "copying" {
				env.ModelTransfer.RenderProgress(p.BytesDone, p.BytesTotal, 0, 25)
			} else if p.Status == "verifying" {
				fmt.Fprint(env.Stdout, "\r  Verifying integrity...          ")
			} else if p.Status == "complete" {
				fmt.Fprintf(env.Stdout, "\r  %s%s%s Import complete                \n", env.Style.Success, env.Style.Check, env.Style.Reset)
			}
		})

		if err != nil {
			fmt.Fprintf(env.Stderr, "\n  %s%s%s Import failed: %v\n", env.Style.Error, env.Style.Cross, env.Style.Reset, err)
			return ReportedError{}
		}

		fmt.Fprintf(env.Stdout, "\n  %s%s%s Model imported to %s\n", env.Style.Success, env.Style.Check, env.Style.Reset, cfg.ModelsDir)

		// Reload llama-server with the imported model
		// Construct the destination path where the model was imported
		importedModelPath := filepath.Join(cfg.ModelsDir, filepath.Base(usbPath))
		if err := env.ModelTransfer.ReloadModel(importedModelPath); err != nil {
			fmt.Fprintln(env.Stdout)
			p.Warning(fmt.Sprintf("Could not auto-reload server: %v", err))
			fmt.Fprintln(env.Stdout)
			p.Info("Manually restart the server:")
			p.Item("Restart service", "sudo systemctl restart llama-server")
			fmt.Fprintln(env.Stdout)
		}
	}
	return nil
}

func getTransferFileSize(path string, env Environment) int64 {
	info, err := env.ModelTransfer.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}
func RunImportModelCommand(ctx context.Context, args []string, env Environment) error {
	return importModelCommand(ctx, args, env)
}
