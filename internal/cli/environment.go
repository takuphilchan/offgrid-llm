package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// CommandRequest is a reviewed local process invocation. The OS-backed runner
// must resolve the named capability rather than execute an arbitrary shell line.
type CommandRequest struct {
	Capability string
	Args       []string
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
}

// CommandRunner is supplied by the OS entry point, never by model output.
type CommandRunner interface {
	Run(context.Context, CommandRequest) error
}

// InvocationOptions are derived from a copy of the incoming arguments.
// Parsing and propagation replace process-global output flags in a later step.
type InvocationOptions struct {
	JSON    bool
	NoColor bool
}

// Environment contains process-owned dependencies for one CLI invocation.
// Execute never falls back to ambient OS state when a dependency is absent.
type Environment struct {
	Stdin         io.Reader
	Stdout        io.Writer
	Stderr        io.Writer
	Getenv        func(string) string
	Setenv        func(string, string) error
	HTTP          *http.Client
	LongHTTP      *http.Client
	Exec          CommandRunner
	Options       InvocationOptions
	Version       string
	ReadFile      func(string) ([]byte, error)
	Device        func() DeviceInfo
	Style         PresentationStyle
	ServerURL     string
	Doctor        func(context.Context) (DoctorSnapshot, error)
	Info          func(context.Context) (InfoSnapshot, error)
	Computer      ComputerDependencies
	Service       ServiceDependencies
	LocalAdmin    LocalAdminDependencies
	ModelVerify   ModelVerifyDependencies
	Batch         BatchDependencies
	ModelPackage  ModelPackageDependencies
	Quantize      QuantizeDependencies
	ModelTransfer ModelTransferDependencies
	Interactive   InteractiveDependencies
	Logs          LogDependencies
	Audio         AudioDependencies
	Integrations  IntegrationDependencies
	LegacyAgent   LegacyAgentDependencies
}

type DeviceInfo struct {
	GPUName     string
	GPUMemoryMB int64
}

func (env Environment) validate() error {
	switch {
	case env.Stdin == nil:
		return errors.New("missing CLI stdin")
	case env.Stdout == nil:
		return errors.New("missing CLI stdout")
	case env.Stderr == nil:
		return errors.New("missing CLI stderr")
	case env.Getenv == nil:
		return errors.New("missing CLI getenv")
	case env.Setenv == nil:
		return errors.New("missing CLI setenv")
	case env.HTTP == nil:
		return errors.New("missing CLI HTTP client")
	case env.LongHTTP == nil:
		return errors.New("missing CLI long HTTP client")
	case env.Exec == nil:
		return errors.New("missing CLI command runner")
	default:
		return nil
	}
}

type Command func(context.Context, []string, Environment) error

// Execute is the internal command boundary. Commands join the registry only
// after their individual migration checks pass; cmd/offgrid retains the
// existing process path until the registry is complete.
func Execute(ctx context.Context, args []string, env Environment) int {
	filtered, options := ParseInvocation(args)
	env.Options = options
	return executeWithCommands(ctx, filtered, env, commandRegistry)
}

var commandRegistry = map[string]Command{
	"version":        versionCommand,
	"help":           helpCommand,
	"search":         searchCommand,
	"list":           listCommand,
	"doctor":         doctorCommand,
	"info":           infoCommand,
	"computer":       computerCommand,
	"session":        sessionServiceCommand,
	"export-session": sessionExportCommand,
	"kb":             knowledgeServiceCommand,
	"agent":          agentServiceCommand,
	"metrics":        metricsCommand,
	"config":         configCommand,
	"users":          usersCommand,
	"audit":          auditCommand,
	"verify":         verifyModelCommand,
	"batch":          batchCommand,
	"model":          modelPackageCommand,
	"quantization":   quantizationCommand,
	"quantize":       quantizeCommand,
	"remove":         removeModelCommand,
	"export":         exportModelCommand,
	"import":         importModelCommand,
	"download":       modelDownloadCommand,
	"download-hf":    modelDownloadCommand,
	"chat":           chatPlaceholderCommand,
	"run":            runInteractiveCommand,
	"benchmark":      benchmarkCommand,
	"compare":        benchmarkCompareCommand,
	"audio":          audioCommand,
	"integration":    integrationCommand,
	"logs":           logsCommand,
}

// HasCommand identifies command families whose execution is owned by this
// package. The OS entry point uses this during the legacy-root transition so
// it cannot maintain a second list of migrated commands.
func HasCommand(name string) bool {
	canonical, ok := CanonicalCommand(name)
	if !ok {
		return false
	}
	return commandRegistry[canonical] != nil
}

func versionCommand(ctx context.Context, args []string, env Environment) error {
	presenter := Presenter{Stdout: env.Stdout, Stderr: env.Stderr, JSON: env.Options.JSON, Style: env.Style}
	return RunVersion(ctx, args, env, presenter)
}

func helpCommand(_ context.Context, _ []string, env Environment) error {
	presenter := Presenter{Stdout: env.Stdout, Stderr: env.Stderr, JSON: env.Options.JSON, Style: env.Style}
	presenter.Help(ResolveVersion(env.Version, env.ReadFile))
	return nil
}

func executeWithCommands(ctx context.Context, args []string, env Environment, commands map[string]Command) int {
	if err := env.validate(); err != nil {
		if env.Stderr != nil {
			_, _ = fmt.Fprintln(env.Stderr, err)
		}
		return 1
	}
	if ctx == nil {
		_, _ = fmt.Fprintln(env.Stderr, "missing CLI context")
		return 1
	}
	if err := ctx.Err(); err != nil {
		return ReportError(err, env.Options.JSON, env.Stdout, env.Stderr)
	}
	if len(args) == 0 {
		_, _ = fmt.Fprintln(env.Stderr, "CLI command unavailable")
		return 1
	}
	name := args[0]
	if canonical, ok := CanonicalCommand(name); ok {
		name = canonical
	}
	command := commands[name]
	if command == nil {
		_, _ = fmt.Fprintln(env.Stderr, "CLI command unavailable")
		return 1
	}
	if err := command(ctx, append([]string(nil), args[1:]...), env); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) && ctx.Err() != nil {
			return ReportError(context.Canceled, env.Options.JSON, env.Stdout, env.Stderr)
		}
		return ReportError(err, env.Options.JSON, env.Stdout, env.Stderr)
	}
	return 0
}
