package main

import (
	"context"
	"fmt"
	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/output"
	"time"
)

func handleList(ctx context.Context, args []string) int {
	return executeCommand(ctx, func(ctx context.Context) error { return runModelList(ctx, args) })
}

func handleQuantization(ctx context.Context) int {
	return executeCommand(ctx, func(ctx context.Context) error {
		return cli.RunQuantizationCommand(ctx, nil, modelQuantizeEnvironment())
	})
}

func handleQuantize(ctx context.Context, args []string) int {
	return executeCommand(ctx, func(ctx context.Context) error {
		return cli.RunQuantizeCommand(ctx, args, modelQuantizeEnvironment())
	})
}

func estimateRAMFromModel(modelID string, quant string) float64 {
	return cli.EstimateRAMFromModel(modelID, quant)
}

func handleVerify(ctx context.Context, args []string) int {
	return executeCommand(ctx, func(ctx context.Context) error {
		return cli.RunVerifyModelCommand(ctx, args, modelVerifyEnvironment())
	})
}

func handleLoRA(args []string) {
	cfg := config.LoadConfig()
	manager := inference.NewLoRAManager(cfg.DataDir, nil)

	// Check for help flag
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = []string{} // Trigger help display
	}

	if len(args) < 1 {
		fmt.Println()
		fmt.Printf("  %sLoRA Adapters%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("  %sExperimental adapter registry%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sUsage%s  offgrid lora <command>\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sCommands%s\n", brandPrimary, colorReset)
		fmt.Printf("    %-24s %sList registered adapters%s\n", "list", colorDim, colorReset)
		fmt.Printf("    %-24s %sRegister an adapter%s\n", "register <name> <path>", colorDim, colorReset)
		fmt.Printf("    %-24s %sRemove an adapter%s\n", "remove <id>", colorDim, colorReset)
		fmt.Printf("    %-24s %sShow adapter info%s\n", "info <id>", colorDim, colorReset)
		fmt.Printf("    %-24s %sSet scale (0.0-1.0)%s\n", "scale <id> <value>", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sMaturity%s  Experimental. Registration works; runtime hot-loading is not available yet.\n", colorDim, colorReset)
		fmt.Println()
		return
	}

	subCmd := args[0]
	switch subCmd {
	case "list":
		adapterList := manager.ListAdapters()
		if output.JSONMode {
			output.PrintJSON(manager.GetStatus())
			return
		}
		fmt.Println()
		fmt.Printf("%sLoRA Adapters%s\n", brandPrimary+colorBold, colorReset)
		fmt.Println()
		if len(adapterList) == 0 {
			fmt.Println("  No adapters registered")
			fmt.Println()
			fmt.Printf("  %sRegister an adapter:%s offgrid lora register <name> <path>\n", colorDim, colorReset)
		} else {
			fmt.Printf("  %sMaturity:%s Experimental registry; runtime loading is not available yet.\n\n", colorDim, colorReset)
			fmt.Printf("  %s%-16s  %-20s  %-8s  %s%s\n", colorDim, "ID", "Name", "Scale", "Path", colorReset)
			for _, a := range adapterList {
				fmt.Printf("  %-16s  %-20s  %-8.2f  %s\n", a.ID[:16], a.Name, a.Scale, a.Path)
			}
		}
		fmt.Println()

	case "register":
		if len(args) < 3 {
			printError("Usage: offgrid lora register <name> <path> [scale]")
			return
		}
		name := args[1]
		path := args[2]
		scale := float32(1.0)
		if len(args) > 3 {
			fmt.Sscanf(args[3], "%f", &scale)
		}

		adapter, err := manager.RegisterAdapter("", name, path, scale, "")
		if err != nil {
			printError(fmt.Sprintf("Failed to register adapter: %v", err))
			return
		}

		if output.JSONMode {
			output.PrintJSON(adapter)
			return
		}
		printSuccess(fmt.Sprintf("Registered adapter: %s (%s)", adapter.Name, adapter.ID))

	case "remove":
		if len(args) < 2 {
			printError("Usage: offgrid lora remove <id>")
			return
		}
		if err := manager.DeleteAdapter(args[1]); err != nil {
			printError(fmt.Sprintf("Failed to remove adapter: %v", err))
			return
		}
		printSuccess("Adapter removed")

	case "info":
		if len(args) < 2 {
			printError("Usage: offgrid lora info <id>")
			return
		}
		adapter, ok := manager.GetAdapter(args[1])
		if !ok {
			printError("Adapter not found")
			return
		}
		if output.JSONMode {
			output.PrintJSON(adapter)
			return
		}
		fmt.Println()
		fmt.Printf("%sAdapter Info%s\n", brandPrimary+colorBold, colorReset)
		fmt.Println()
		fmt.Printf("  %sID:%s          %s\n", colorDim, colorReset, adapter.ID)
		fmt.Printf("  %sName:%s        %s\n", colorDim, colorReset, adapter.Name)
		fmt.Printf("  %sPath:%s        %s\n", colorDim, colorReset, adapter.Path)
		fmt.Printf("  %sScale:%s       %.2f\n", colorDim, colorReset, adapter.Scale)
		fmt.Printf("  %sCreated:%s     %s\n", colorDim, colorReset, adapter.CreatedAt.Format(time.RFC3339))
		fmt.Println()

	case "scale":
		if len(args) < 3 {
			printError("Usage: offgrid lora scale <id> <value>")
			return
		}
		var scale float32
		fmt.Sscanf(args[2], "%f", &scale)
		if err := manager.SetAdapterScale(context.Background(), args[1], scale); err != nil {
			printError(fmt.Sprintf("Failed to set scale: %v", err))
			return
		}
		printSuccess(fmt.Sprintf("Scale updated to %.2f", scale))

	default:
		printError(fmt.Sprintf("Unknown lora command: %s", subCmd))
	}
}

// handleAgent handles AI agent commands
