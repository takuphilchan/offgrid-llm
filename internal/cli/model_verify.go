package cli

import (
	"context"
	"fmt"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

// ModelVerifyDependencies keep model-directory access inside the selected
// workspace; direct tests can point both adapters at a disposable directory.
type ModelVerifyDependencies struct {
	NewRegistry  func(string) *models.Registry
	NewValidator func(string) *models.Validator
}

func verifyModelCommand(_ context.Context, args []string, env Environment) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = nil
	}
	s := env.Style
	p := Presenter{Stdout: env.Stdout, Stderr: env.Stderr, JSON: env.Options.JSON, Style: s}
	if len(args) == 0 {
		_, _ = fmt.Fprintln(env.Stdout)
		_, _ = fmt.Fprintf(env.Stdout, "  %s◈ Verify Model%s\n", s.Primary+s.Bold, s.Reset)
		_, _ = fmt.Fprintf(env.Stdout, "  %sVerify model file integrity using SHA256 checksum%s\n\n", s.Dim, s.Reset)
		_, _ = fmt.Fprintf(env.Stdout, "  %sUsage%s  offgrid verify <model-name>\n\n", s.Dim, s.Reset)
		_, _ = fmt.Fprintf(env.Stdout, "  %sExamples%s\n", s.Primary, s.Reset)
		_, _ = fmt.Fprintf(env.Stdout, "    %s$%s offgrid verify llama-2-7b-chat.Q4_K_M\n\n", s.Dim, s.Reset)
		return nil
	}
	cfg, err := localConfig(env)
	if err != nil {
		return err
	}
	if env.ModelVerify.NewRegistry == nil || env.ModelVerify.NewValidator == nil {
		return fmt.Errorf("local model-verification adapters unavailable")
	}
	registry := env.ModelVerify.NewRegistry(cfg.ModelsDir)
	if err := registry.ScanModels(); err != nil {
		p.HelpfulError(err, "Scanning models")
		return LegacyReportedError{}
	}
	meta, err := registry.GetModel(args[0])
	if err != nil {
		p.Error(fmt.Sprintf("Model not found: %s", args[0]))
		_, _ = fmt.Fprintln(env.Stdout)
		p.Info("Use 'offgrid list' to see available models")
		_, _ = fmt.Fprintln(env.Stdout)
		return LegacyReportedError{}
	}
	_, _ = fmt.Fprintln(env.Stdout)
	_, _ = fmt.Fprintf(env.Stdout, "%sVerifying Model%s\n", s.Primary+s.Bold, s.Reset)
	_, _ = fmt.Fprintf(env.Stdout, "Model: %s%s%s\n", s.Bold, meta.ID, s.Reset)
	_, _ = fmt.Fprintf(env.Stdout, "Path:  %s%s%s\n\n", s.Dim, meta.Path, s.Reset)
	validator := env.ModelVerify.NewValidator(cfg.ModelsDir)
	result, err := validator.ValidateModel(meta.Path)
	if err != nil {
		p.HelpfulError(err, "Validation")
		return LegacyReportedError{}
	}
	if result.Valid {
		_, _ = fmt.Fprintf(env.Stdout, "%s✓ Valid GGUF Model%s\n", s.Success, s.Reset)
	} else {
		_, _ = fmt.Fprintf(env.Stdout, "%s✗ Invalid Model%s\n", s.Error, s.Reset)
	}
	_, _ = fmt.Fprintf(env.Stdout, "Size: %s\n", FormatBytes(result.FileSize))
	if result.SHA256Hash != "" {
		_, _ = fmt.Fprintf(env.Stdout, "SHA256: %s%s%s\n", s.Dim, result.SHA256Hash, s.Reset)
	}
	if len(result.Errors) > 0 {
		_, _ = fmt.Fprintln(env.Stdout)
		_, _ = fmt.Fprintf(env.Stdout, "%sErrors:%s\n", s.Error, s.Reset)
		for _, issue := range result.Errors {
			_, _ = fmt.Fprintf(env.Stdout, "• %s\n", issue)
		}
	}
	if len(result.Warnings) > 0 {
		_, _ = fmt.Fprintln(env.Stdout)
		_, _ = fmt.Fprintf(env.Stdout, "%sWarnings:%s\n", s.Accent, s.Reset)
		for _, warning := range result.Warnings {
			_, _ = fmt.Fprintf(env.Stdout, "• %s\n", warning)
		}
	}
	_, _ = fmt.Fprintln(env.Stdout)
	_, _ = fmt.Fprintln(env.Stdout)
	return nil
}

func RunVerifyModelCommand(ctx context.Context, args []string, env Environment) error {
	return verifyModelCommand(ctx, args, env)
}
