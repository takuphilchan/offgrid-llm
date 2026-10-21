package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/audit"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

// LocalAdminDependencies preserve the preexisting local administration path.
// The OS adapter supplies the stores and paths; tests supply disposable ones.
type LocalAdminDependencies struct {
	LoadConfig       func() *config.Config
	LoadWithPriority func(string) (*config.Config, error)
	LoadFromFile     func(string) (*config.Config, error)
	SaveConfig       func(*config.Config, string) error
	HomeDir          func() (string, error)
	MkdirAll         func(string, uint32) error
	NewUserStore     func(string) *users.UserStore
	NewQuotaManager  func(string) *users.QuotaManager
	NewAuditLogger   func(audit.AuditConfig) (*audit.AuditLogger, error)
	GlobAuditFiles   func(string) ([]string, error)
	Entropy          io.Reader
}

func localJSON(env Environment, value any) error {
	encoder := json.NewEncoder(env.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func localConfig(env Environment) (*config.Config, error) {
	if env.LocalAdmin.LoadConfig == nil {
		return nil, fmt.Errorf("local configuration adapter unavailable")
	}
	cfg := env.LocalAdmin.LoadConfig()
	if cfg == nil {
		return nil, fmt.Errorf("local configuration unavailable")
	}
	return cfg, nil
}

func metricsCommand(ctx context.Context, _ []string, env Environment) error {
	cfg, err := localConfig(env)
	if err != nil {
		return err
	}
	serverURL := fmt.Sprintf("http://localhost:%d/metrics", cfg.ServerPort)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, serverURL, nil)
	if err != nil {
		return err
	}
	resp, err := env.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if env.Options.JSON {
			encoder := json.NewEncoder(env.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(map[string]string{"status": "server_not_running", "message": "Start server to view live metrics"})
		}
		s := env.Style
		_, _ = fmt.Fprintln(env.Stdout)
		_, _ = fmt.Fprintf(env.Stdout, "%sMetrics%s\n\n", s.Primary+s.Bold, s.Reset)
		_, _ = fmt.Fprintf(env.Stdout, "  %sServer not running%s\n\n", s.Accent, s.Reset)
		_, _ = fmt.Fprintln(env.Stdout, "  Start the server to view live metrics:")
		_, _ = fmt.Fprintf(env.Stdout, "    %soffgrid serve%s\n\n", s.Dim, s.Reset)
		_, _ = fmt.Fprintln(env.Stdout, "  Then access metrics at:")
		_, _ = fmt.Fprintf(env.Stdout, "    %s%s%s\n\n", s.Dim, serverURL, s.Reset)
		return nil
	}
	defer resp.Body.Close()
	// The legacy command displays the body even for non-2xx responses and
	// ignores a partial read error. Preserve that behavior during migration.
	body, _ := io.ReadAll(resp.Body)
	if env.Options.JSON {
		metricsData := make(map[string]string)
		for _, line := range strings.Split(string(body), "\n") {
			if strings.HasPrefix(line, "#") || line == "" {
				continue
			}
			parts := strings.SplitN(line, " ", 2)
			if len(parts) == 2 {
				metricsData[parts[0]] = parts[1]
			}
		}
		encoder := json.NewEncoder(env.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(metricsData)
	}
	s := env.Style
	_, _ = fmt.Fprintln(env.Stdout)
	_, _ = fmt.Fprintf(env.Stdout, "%sPrometheus Metrics%s\n\n", s.Primary+s.Bold, s.Reset)
	_, _ = fmt.Fprintln(env.Stdout, string(body))
	return nil
}

func RunMetricsCommand(ctx context.Context, args []string, env Environment) error {
	return metricsCommand(ctx, args, env)
}

func configCommand(_ context.Context, args []string, env Environment) error {
	s := env.Style
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = nil
	}
	if len(args) == 0 {
		_, _ = fmt.Fprintln(env.Stdout)
		_, _ = fmt.Fprintf(env.Stdout, "  %s◈ Configuration%s\n", s.Primary+s.Bold, s.Reset)
		_, _ = fmt.Fprintf(env.Stdout, "  %sManage settings and preferences%s\n\n", s.Dim, s.Reset)
		_, _ = fmt.Fprintf(env.Stdout, "  %sUsage%s  offgrid config <action>\n\n", s.Dim, s.Reset)
		_, _ = fmt.Fprintf(env.Stdout, "  %sActions%s\n", s.Primary, s.Reset)
		_, _ = fmt.Fprintf(env.Stdout, "    %-20s %sCreate config file%s\n", "init [path]", s.Dim, s.Reset)
		_, _ = fmt.Fprintf(env.Stdout, "    %-20s %sDisplay current config%s\n", "show", s.Dim, s.Reset)
		_, _ = fmt.Fprintf(env.Stdout, "    %-20s %sValidate config file%s\n\n", "validate [path]", s.Dim, s.Reset)
		_, _ = fmt.Fprintf(env.Stdout, "  %sExamples%s\n", s.Primary, s.Reset)
		_, _ = fmt.Fprintf(env.Stdout, "    %s$%s offgrid config init\n", s.Dim, s.Reset)
		_, _ = fmt.Fprintf(env.Stdout, "    %s$%s offgrid config show\n", s.Dim, s.Reset)
		_, _ = fmt.Fprintf(env.Stdout, "    %s$%s offgrid config validate config.yaml\n\n", s.Dim, s.Reset)
		return ReportedError{}
	}
	switch args[0] {
	case "init":
		path := ""
		if len(args) > 1 {
			path = args[1]
		} else {
			if env.LocalAdmin.HomeDir == nil || env.LocalAdmin.MkdirAll == nil {
				return fmt.Errorf("local home-directory adapter unavailable")
			}
			home, _ := env.LocalAdmin.HomeDir()
			dir := filepath.Join(home, ".offgrid-llm")
			_ = env.LocalAdmin.MkdirAll(dir, 0755)
			path = filepath.Join(dir, "config.yaml")
		}
		cfg, err := localConfig(env)
		if err != nil {
			return err
		}
		if env.LocalAdmin.SaveConfig == nil {
			return fmt.Errorf("local configuration writer unavailable")
		}
		if err := env.LocalAdmin.SaveConfig(cfg, path); err != nil {
			_, _ = fmt.Fprintf(env.Stderr, "  ✗ Failed to create config: %v\n", err)
			return ReportedError{}
		}
		_, _ = fmt.Fprintf(env.Stdout, "  ✓ Created config: %s\n\n", path)
		_, _ = fmt.Fprintln(env.Stdout, "To use:")
		_, _ = fmt.Fprintf(env.Stdout, "  export OFFGRID_CONFIG=%s\n", path)
		_, _ = fmt.Fprintln(env.Stdout, "  offgrid")
		_, _ = fmt.Fprintln(env.Stdout)
		return nil
	case "show":
		if env.LocalAdmin.LoadWithPriority == nil {
			return fmt.Errorf("local configuration reader unavailable")
		}
		cfg, err := env.LocalAdmin.LoadWithPriority(env.Getenv("OFFGRID_CONFIG"))
		if err != nil {
			_, _ = fmt.Fprintf(env.Stderr, "  ✗ Failed to load config: %v\n", err)
			return ReportedError{}
		}
		_, _ = fmt.Fprintln(env.Stdout, "\nConfiguration\n\nServer")
		_, _ = fmt.Fprintf(env.Stdout, "  Host:             %s\n  Port:             %d\n\n", cfg.ServerHost, cfg.ServerPort)
		_, _ = fmt.Fprintln(env.Stdout, "Models")
		_, _ = fmt.Fprintf(env.Stdout, "  Directory:        %s\n  Default:          %s\n  Max context:      %d\n  Threads:          %d\n\n", cfg.ModelsDir, cfg.DefaultModel, cfg.MaxContextSize, cfg.NumThreads)
		_, _ = fmt.Fprintln(env.Stdout, "Resources")
		_, _ = fmt.Fprintf(env.Stdout, "  Max memory:       %d MB\n  Max models:       %d\n  GPU:              %v\n", cfg.MaxMemoryMB, cfg.MaxModels, cfg.EnableGPU)
		if cfg.EnableGPU {
			_, _ = fmt.Fprintf(env.Stdout, "  GPU layers:       %d\n", cfg.NumGPULayers)
		}
		_, _ = fmt.Fprintln(env.Stdout)
		if cfg.EnableP2P {
			_, _ = fmt.Fprintln(env.Stdout, "P2P")
			_, _ = fmt.Fprintf(env.Stdout, "  Enabled:          %v\n  Port:             %d\n  Discovery:        %d\n\n", cfg.EnableP2P, cfg.P2PPort, cfg.DiscoveryPort)
		}
		_, _ = fmt.Fprintln(env.Stdout, "Logging")
		_, _ = fmt.Fprintf(env.Stdout, "  Level:            %s\n", cfg.LogLevel)
		if cfg.LogFile != "" {
			_, _ = fmt.Fprintf(env.Stdout, "  File:             %s\n", cfg.LogFile)
		}
		_, _ = fmt.Fprintln(env.Stdout)
		return nil
	case "validate":
		if len(args) < 2 {
			_, _ = fmt.Fprintln(env.Stderr, "Usage: offgrid config validate <path>")
			return ReportedError{}
		}
		if env.LocalAdmin.LoadFromFile == nil {
			return fmt.Errorf("local configuration reader unavailable")
		}
		cfg, err := env.LocalAdmin.LoadFromFile(args[1])
		if err != nil {
			_, _ = fmt.Fprintf(env.Stderr, "  ✗ Invalid config: %v\n", err)
			return ReportedError{}
		}
		if err := cfg.Validate(); err != nil {
			_, _ = fmt.Fprintf(env.Stderr, "  ✗ Validation failed: %v\n", err)
			return ReportedError{}
		}
		_, _ = fmt.Fprintf(env.Stdout, "  ✓ Config valid: %s\n\n", args[1])
		return nil
	default:
		_, _ = fmt.Fprintf(env.Stderr, "Unknown action: %s\n", args[0])
		return ReportedError{}
	}
}

func RunConfigCommand(ctx context.Context, args []string, env Environment) error {
	return configCommand(ctx, args, env)
}
