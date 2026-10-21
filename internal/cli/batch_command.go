package cli

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// BatchDependencies are owned by the OS adapter. Tests can exercise parsing
// and presentation without starting a server, opening model files, or waiting.
type BatchDependencies struct {
	EnsureServer func() error
	StartServer  func() error
	Sleep        func(time.Duration)
	ProcessFile  func(context.Context, string, string, int) error
}

func batchCommand(ctx context.Context, args []string, env Environment) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = nil
	}
	s := env.Style
	p := Presenter{Stdout: env.Stdout, Stderr: env.Stderr, JSON: env.Options.JSON, Style: s}
	if len(args) == 0 {
		_, _ = fmt.Fprintln(env.Stdout)
		_, _ = fmt.Fprintf(env.Stdout, "  %s◈ Batch Processing%s\n", s.Primary+s.Bold, s.Reset)
		_, _ = fmt.Fprintf(env.Stdout, "  %sProcess multiple prompts from JSONL files%s\n\n", s.Dim, s.Reset)
		_, _ = fmt.Fprintf(env.Stdout, "  %sUsage%s  offgrid batch process <input.jsonl> [output.jsonl]\n\n", s.Dim, s.Reset)
		_, _ = fmt.Fprintf(env.Stdout, "  %sOptions%s\n", s.Primary, s.Reset)
		_, _ = fmt.Fprintf(env.Stdout, "    %-20s %sParallel requests (default: 4)%s\n\n", "--concurrency <n>", s.Dim, s.Reset)
		_, _ = fmt.Fprintf(env.Stdout, "  %sInput Format%s  JSONL with id, model, prompt fields\n", s.Dim, s.Reset)
		_, _ = fmt.Fprintf(env.Stdout, "    %s{\"id\": \"1\", \"model\": \"llama3\", \"prompt\": \"Hello\"}%s\n\n", s.Dim, s.Reset)
		_, _ = fmt.Fprintf(env.Stdout, "  %sExamples%s\n", s.Primary, s.Reset)
		_, _ = fmt.Fprintf(env.Stdout, "    %s$%s offgrid batch process prompts.jsonl\n", s.Dim, s.Reset)
		_, _ = fmt.Fprintf(env.Stdout, "    %s$%s offgrid batch process in.jsonl out.jsonl --concurrency 8\n\n", s.Dim, s.Reset)
		return nil
	}
	if args[0] != "process" {
		p.Error("Usage: offgrid batch process <input.jsonl>")
		_, _ = fmt.Fprintf(env.Stdout, "  %sSee:%s offgrid batch --help\n", s.Muted, s.Reset)
		return LegacyReportedError{}
	}
	if len(args) < 2 {
		p.Error("Usage: offgrid batch process <input.jsonl> [output.jsonl]")
		return LegacyReportedError{}
	}
	inputPath, outputPath, concurrency := args[1], "batch-results.jsonl", 4
	for i := 2; i < len(args); i++ {
		if args[i] == "--concurrency" || args[i] == "-c" {
			if i+1 < len(args) {
				_, _ = fmt.Sscanf(args[i+1], "%d", &concurrency)
				i++
			}
		} else if !strings.HasPrefix(args[i], "--") {
			outputPath = args[i]
		}
	}
	if env.Batch.EnsureServer == nil || env.Batch.StartServer == nil || env.Batch.Sleep == nil || env.Batch.ProcessFile == nil {
		return fmt.Errorf("batch execution adapters unavailable")
	}
	p.Info(fmt.Sprintf("Processing: %s to %s (concurrency=%d)", inputPath, outputPath, concurrency))
	_, _ = fmt.Fprintln(env.Stdout)
	if err := env.Batch.EnsureServer(); err != nil {
		p.Warning("Server not running. Starting server...")
		if err := env.Batch.StartServer(); err != nil {
			p.Error("Failed to start server. Please run 'offgrid serve' first.")
			return LegacyReportedError{}
		}
		env.Batch.Sleep(2 * time.Second)
	}
	if err := env.Batch.ProcessFile(ctx, inputPath, outputPath, concurrency); err != nil {
		p.Error(fmt.Sprintf("Batch processing failed: %v", err))
		return LegacyReportedError{}
	}
	p.Success(fmt.Sprintf("Results written to: %s", outputPath))
	return nil
}

func RunBatchCommand(ctx context.Context, args []string, env Environment) error {
	return batchCommand(ctx, args, env)
}
