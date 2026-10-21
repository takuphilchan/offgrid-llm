package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/takuphilchan/offgrid-llm/internal/resource"
)

// InteractiveDependencies expose only host-owned model/process and terminal
// capabilities to the legacy run command. The parser, request loop and
// persistence policy live in internal/cli.
type InteractiveDependencies struct {
	DownloadHF         func(context.Context, []string) error
	EnsureServer       func() error
	StartServer        func() error
	EnsureLlama        func() error
	StartLlama         func(string) error
	ReadLlamaPort      func(string) string
	WaitModelReady     func(string, int) error
	Wait               func(context.Context, int) error
	ReadDir            func(string) ([]os.DirEntry, error)
	ShouldUseTUI       func() bool
	RunTUI             func(context.Context, *InteractiveTUIState) error
	IsEmbeddingModel   func(...string) bool
	EmbeddingChatError func(string) string
	SyncOutput         func() error
	DetectResources    func() (*resource.SystemResources, error)
}

func runInteractiveCommand(ctx context.Context, args []string, env Environment) error {
	if env.Interactive.DownloadHF == nil || env.Interactive.EnsureServer == nil || env.Interactive.StartServer == nil || env.Interactive.EnsureLlama == nil || env.Interactive.StartLlama == nil || env.Interactive.ReadLlamaPort == nil || env.Interactive.WaitModelReady == nil || env.Interactive.Wait == nil || env.Interactive.ReadDir == nil || env.Interactive.ShouldUseTUI == nil || env.Interactive.RunTUI == nil || env.Interactive.IsEmbeddingModel == nil || env.Interactive.EmbeddingChatError == nil || env.Interactive.DetectResources == nil || env.ReadFile == nil {
		return fmt.Errorf("interactive run adapter unavailable")
	}
	return runInteractive(ctx, args, env)
}

func RunInteractiveCommand(ctx context.Context, args []string, env Environment) error {
	return runInteractiveCommand(ctx, args, env)
}
