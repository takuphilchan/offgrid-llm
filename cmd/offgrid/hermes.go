package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/integrations"
	"github.com/takuphilchan/offgrid-llm/internal/output"
)

const (
	hermesUnixInstallerURL    = "https://hermes-agent.nousresearch.com/install.sh"
	hermesWindowsInstallerURL = "https://hermes-agent.nousresearch.com/install.ps1"
	maxHermesInstallerSize    = 4 << 20
)

var (
	hermesLookPath    = exec.LookPath
	hermesExecCommand = exec.Command
	hermesRuntimeOS   = runtime.GOOS
	hermesRuntimeArch = runtime.GOARCH
)

type hermesInstallOptions struct {
	ServerURL   string
	BaseURL     string
	ModelID     string
	APIKey      string
	Force       bool
	Yes         bool
	WithBrowser bool
	RunDoctor   bool
}

type hermesInstallResult struct {
	Installed     bool     `json:"installed"`
	Executable    string   `json:"executable"`
	PluginPath    string   `json:"plugin_path"`
	Provider      string   `json:"provider"`
	ModelID       string   `json:"model_id"`
	BaseURL       string   `json:"base_url"`
	DoctorChecked bool     `json:"doctor_checked"`
	DoctorOK      bool     `json:"doctor_ok"`
	Warnings      []string `json:"warnings,omitempty"`
}

type hermesStatusResult struct {
	Installed  bool                  `json:"installed"`
	Executable string                `json:"executable,omitempty"`
	Plugin     bool                  `json:"plugin"`
	PluginPath string                `json:"plugin_path"`
	Service    bool                  `json:"service"`
	Runtime    *cliIntegrationStatus `json:"runtime,omitempty"`
	Error      string                `json:"error,omitempty"`
}

func handleHermes(args []string) error {
	if len(args) == 0 {
		return runHermesChat(nil)
	}
	switch strings.ToLower(args[0]) {
	case "help", "-h", "--help":
		printHermesHelp()
		return nil
	case "install", "setup":
		if containsHelp(args[1:]) {
			printHermesHelp()
			return nil
		}
		return handleHermesInstall(args[1:])
	case "status", "check":
		return handleHermesStatus(args[1:])
	case "doctor":
		return runHermesRaw(append([]string{"doctor"}, args[1:]...))
	case "test", "verify":
		return handleHermesTest(args[1:])
	case "run":
		return runHermesChat(args[1:])
	case "chat":
		return runHermesChat(args[1:])
	default:
		if strings.HasPrefix(args[0], "-") {
			return runHermesChat(args)
		}
		// Preserve access to Hermes' own commands such as model, sessions,
		// update, and gateway without reimplementing its CLI in OffGrid.
		return runHermesRaw(args)
	}
}

func printHermesHelp() {
	fmt.Println()
	fmt.Printf("  %sHermes Agent on OffGrid%s\n", brandPrimary+colorBold, colorReset)
	fmt.Printf("  %sInstall, configure, verify, and run Hermes with private OffGrid inference%s\n\n", brandMuted, colorReset)
	fmt.Printf("  %-38s %sInstall Hermes and configure OffGrid%s\n", "offgrid hermes install", colorDim, colorReset)
	fmt.Printf("  %-38s %sInspect setup and model-context preflight%s\n", "offgrid hermes status", colorDim, colorReset)
	fmt.Printf("  %-38s %sRun a real inference smoke test%s\n", "offgrid hermes test", colorDim, colorReset)
	fmt.Printf("  %-38s %sOpen an interactive Hermes session%s\n", "offgrid hermes", colorDim, colorReset)
	fmt.Printf("  %-38s %sAsk Hermes using OffGrid%s\n", "offgrid hermes -q \"...\"", colorDim, colorReset)
	fmt.Printf("  %-38s %sRun Hermes diagnostics%s\n", "offgrid hermes doctor", colorDim, colorReset)
	fmt.Printf("\n  %sInstall options%s\n", brandMuted, colorReset)
	fmt.Printf("    %-24s %sChoose an installed OffGrid model%s\n", "--model <id>", colorDim, colorReset)
	fmt.Printf("    %-24s %sAddress Hermes uses for OffGrid%s\n", "--base-url <url>", colorDim, colorReset)
	fmt.Printf("    %-24s %sAddress this CLI uses for OffGrid%s\n", "--server <url>", colorDim, colorReset)
	fmt.Printf("    %-24s %sAuthentication token (default: local token)%s\n", "--api-key <token>", colorDim, colorReset)
	fmt.Printf("    %-24s %sReplace a locally modified provider bundle%s\n", "--force", colorDim, colorReset)
	fmt.Printf("    %-24s %sAccept the official external installer prompt%s\n", "--yes, -y", colorDim, colorReset)
	fmt.Printf("    %-24s %sAlso install optional npm/browser tools (may take time)%s\n", "--with-browser", colorDim, colorReset)
	fmt.Printf("    %-24s %sAlso run full Hermes diagnostics (may check remote services)%s\n\n", "--doctor", colorDim, colorReset)
}

func handleHermesInstall(args []string) error {
	options, err := parseHermesInstallOptions(args)
	if err != nil {
		return err
	}
	publicURL := options.BaseURL
	if publicURL == "" {
		publicURL = options.ServerURL
	}
	setup, err := fetchIntegrationSetup(options.ServerURL, "hermes", options.ModelID, publicURL)
	if err != nil {
		return fmt.Errorf("cannot reach a compatible OffGrid service: %w; start OffGrid, then retry offgrid hermes install", err)
	}
	if !setup.Integration.Ready {
		for _, warning := range setup.Integration.Warnings {
			printWarning(warning)
		}
		return fmt.Errorf("OffGrid is reachable, but Hermes is not ready; %s", hermesReadinessAdvice(setup.Integration))
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot resolve user home: %w", err)
	}
	executable, findErr := findHermesExecutable(home)
	if findErr != nil {
		accepted, confirmErr := confirmHermesInstall(options.Yes)
		if confirmErr != nil {
			return confirmErr
		}
		if !accepted {
			printInfo("Installation cancelled; no external software was changed")
			return nil
		}
		hermesProgress("Installing Hermes Agent with its official installer")
		if err := installHermesRuntime(); err != nil {
			return fmt.Errorf("Hermes installation failed: %w", err)
		}
		executable, err = findHermesExecutable(home)
		if err != nil {
			return fmt.Errorf("Hermes installed, but its command could not be located; open a new terminal and run offgrid hermes install again")
		}
	} else {
		hermesProgress("Hermes Agent is already installed")
	}

	hermesProgress("Installing the native OffGrid provider")
	plugin, err := integrations.InstallPluginBundle("hermes", home, options.Force)
	if err != nil {
		return err
	}

	baseURL := setup.Setup.Environment["OFFGRID_BASE_URL"]
	if baseURL == "" {
		baseURL = strings.TrimRight(publicURL, "/") + "/v1"
	}
	apiKey := options.APIKey
	if apiKey == "" {
		apiKey = setup.Setup.Environment["OFFGRID_API_KEY"]
	}
	if apiKey == "" {
		apiKey = "offgrid-local"
	}
	environment := mergedEnvironment(os.Environ(), map[string]string{
		"OFFGRID_API_KEY":  apiKey,
		"OFFGRID_BASE_URL": baseURL,
	})

	hermesProgress("Saving the OffGrid provider and model in Hermes")
	for _, command := range hermesConfigurationCommands(setup, apiKey, baseURL) {
		if err := runHermesProcess(executable, command, environment, output.JSONMode); err != nil {
			return fmt.Errorf("cannot configure Hermes setting %s: %w", command[2], err)
		}
	}

	runtimeWarnings := append([]string(nil), setup.Integration.Warnings...)
	if options.WithBrowser {
		hermesProgress("Installing optional browser and computer-use dependencies")
		if err := installHermesOptionalTools(); err != nil {
			warning := "Hermes core is configured; optional browser tools failed; retry with offgrid hermes install --with-browser"
			runtimeWarnings = append(runtimeWarnings, warning)
			printWarning(fmt.Sprintf("%s (%v)", warning, err))
		}
	}

	doctorOK := false
	if options.RunDoctor {
		hermesProgress("Checking the Hermes provider connection")
		if err := runHermesProcess(executable, []string{"doctor"}, environment, output.JSONMode); err != nil {
			printWarning(fmt.Sprintf("Hermes is configured, but doctor reported a problem: %v", err))
		} else {
			doctorOK = true
		}
	}

	result := hermesInstallResult{
		Installed:     true,
		Executable:    executable,
		PluginPath:    plugin.Path,
		Provider:      "offgrid",
		ModelID:       setup.Integration.ModelID,
		BaseURL:       baseURL,
		DoctorChecked: options.RunDoctor,
		DoctorOK:      doctorOK,
		Warnings:      runtimeWarnings,
	}
	if output.JSONMode {
		outputJSON(result)
		return nil
	}
	printSuccess("Hermes Agent is configured to use OffGrid (core runtime)")
	fmt.Printf("  %sModel%s     %s\n", brandMuted, colorReset, result.ModelID)
	fmt.Printf("  %sEndpoint%s  %s\n", brandMuted, colorReset, result.BaseURL)
	fmt.Printf("  %sVerify%s    offgrid hermes test\n", brandMuted, colorReset)
	fmt.Printf("  %sStart%s     offgrid hermes\n", brandMuted, colorReset)
	if !options.WithBrowser {
		fmt.Printf("  %sOptional%s  offgrid hermes install --with-browser (browser/computer use)\n", brandMuted, colorReset)
	}
	fmt.Println()
	return nil
}

func handleHermesStatus(args []string) error {
	serverURL, _, _, err := integrationFlags(args)
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot resolve user home: %w", err)
	}
	result := hermesStatusResult{PluginPath: integrations.HermesPluginPath(home)}
	if executable, findErr := findHermesExecutable(home); findErr == nil {
		result.Installed = true
		result.Executable = executable
	}
	if info, statErr := os.Stat(filepath.Join(result.PluginPath, "__init__.py")); statErr == nil && !info.IsDir() {
		result.Plugin = true
	}
	var response cliIntegrationsResponse
	if err := getIntegrationJSON(serverURL+"/v1/integrations", &response); err != nil {
		result.Error = err.Error()
	} else {
		result.Service = true
		for index := range response.Integrations {
			if response.Integrations[index].ID == "hermes" {
				item := response.Integrations[index]
				result.Runtime = &item
				break
			}
		}
	}
	if outputJSON(result) {
		if result.Installed && result.Plugin && result.Service && result.Runtime != nil && result.Runtime.Ready {
			return nil
		}
		return fmt.Errorf("Hermes is not ready; %s", hermesStatusAdvice(result))
	}
	fmt.Println()
	fmt.Printf("  %sHermes Agent readiness%s\n\n", brandPrimary+colorBold, colorReset)
	printHermesStatusLine("Hermes runtime", result.Installed, result.Executable)
	printHermesStatusLine("OffGrid provider", result.Plugin, result.PluginPath)
	printHermesStatusLine("OffGrid service", result.Service, serverURL)
	if result.Runtime != nil {
		printHermesStatusLine("Model/context", result.Runtime.Ready, emptyFallback(result.Runtime.ModelID, "no model"))
		for _, warning := range result.Runtime.Warnings {
			printWarning(warning)
		}
	}
	if result.Error != "" {
		printWarning(result.Error)
	}
	if result.Installed && result.Plugin && result.Service && result.Runtime != nil && result.Runtime.Ready {
		fmt.Printf("\n  %sPreflight ready.%s Verify a model reply: offgrid hermes test\n\n", brandSuccess+colorBold, colorReset)
		return nil
	} else {
		fmt.Printf("\n  %sNext:%s %s\n\n", brandAccent, colorReset, hermesStatusAdvice(result))
		return fmt.Errorf("Hermes is not ready")
	}
}

func hermesStatusAdvice(status hermesStatusResult) string {
	if !status.Installed || !status.Plugin || !status.Service || status.Runtime == nil {
		return "run offgrid hermes install after starting the OffGrid service"
	}
	return hermesReadinessAdvice(*status.Runtime)
}

func hermesReadinessAdvice(status cliIntegrationStatus) string {
	if status.ContextWindow < status.MinimumContext {
		return fmt.Sprintf("Hermes requires an effective OffGrid context of at least %d tokens (65536 recommended). Increase the service context only if RAM/VRAM allows it, then retry. Do not claim a larger Hermes context than OffGrid actually allocates", status.MinimumContext)
	}
	if status.ModelID == "" {
		return "install a suitable chat model in OffGrid, then retry offgrid hermes install"
	}
	return "inspect offgrid integrations list, then retry offgrid hermes install"
}

func handleHermesTest(args []string) error {
	serverURL, modelID, publicURL, err := integrationFlags(args)
	if err != nil {
		return err
	}
	if publicURL == "" {
		publicURL = serverURL
	}
	setup, err := fetchIntegrationSetup(serverURL, "hermes", modelID, publicURL)
	if err != nil {
		return fmt.Errorf("cannot inspect OffGrid: %w", err)
	}
	if !setup.Integration.Ready {
		return fmt.Errorf("Hermes cannot be tested yet; %s", hermesReadinessAdvice(setup.Integration))
	}
	home, _ := os.UserHomeDir()
	executable, err := findHermesExecutable(home)
	if err != nil {
		return fmt.Errorf("Hermes is not installed; run offgrid hermes install")
	}
	baseURL := setup.Setup.Environment["OFFGRID_BASE_URL"]
	apiKey := setup.Setup.Environment["OFFGRID_API_KEY"]
	environment := mergedEnvironment(os.Environ(), map[string]string{
		"OFFGRID_API_KEY":  apiKey,
		"OFFGRID_BASE_URL": baseURL,
	})
	const expected = "HERMES_OFFGRID_OK"
	if err := runHermesSmokeTest(executable, []string{
		"chat", "--oneshot", "--provider", "offgrid", "--model", setup.Integration.ModelID,
		"-q", "Reply with exactly: " + expected,
	}, environment, expected); err != nil {
		return fmt.Errorf("Hermes smoke test failed: %w", err)
	}
	printSuccess(fmt.Sprintf("Hermes reached OffGrid at %s", baseURL))
	return nil
}

func runHermesChat(args []string) error {
	chatArgs := []string{"chat", "--provider", "offgrid"}
	chatArgs = append(chatArgs, args...)
	return runHermesRaw(chatArgs)
}

func runHermesRaw(args []string) error {
	home, _ := os.UserHomeDir()
	executable, err := findHermesExecutable(home)
	if err != nil {
		return fmt.Errorf("Hermes Agent is not installed or is not on PATH; run offgrid hermes install")
	}
	if err := runHermesProcess(executable, args, os.Environ(), false); err != nil {
		return fmt.Errorf("Hermes exited with an error: %w", err)
	}
	return nil
}

func parseHermesInstallOptions(args []string) (hermesInstallOptions, error) {
	cfg, err := config.LoadWithPriority(os.Getenv("OFFGRID_CONFIG"))
	if err != nil {
		return hermesInstallOptions{}, err
	}
	options := hermesInstallOptions{
		ServerURL: fmt.Sprintf("http://127.0.0.1:%d", cfg.ServerPort),
		APIKey:    strings.TrimSpace(os.Getenv("OFFGRID_API_KEY")),
	}
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--server", "--base-url", "--model", "--api-key":
			name := args[index]
			index++
			if index >= len(args) {
				return hermesInstallOptions{}, fmt.Errorf("%s requires a value", name)
			}
			value := strings.TrimSpace(args[index])
			if value == "" {
				return hermesInstallOptions{}, fmt.Errorf("%s cannot be empty", name)
			}
			switch name {
			case "--server":
				options.ServerURL = strings.TrimRight(value, "/")
			case "--base-url":
				options.BaseURL = strings.TrimRight(value, "/")
			case "--model":
				options.ModelID = value
			case "--api-key":
				options.APIKey = value
			}
		case "--force":
			options.Force = true
		case "--yes", "-y":
			options.Yes = true
		case "--with-browser":
			options.WithBrowser = true
		case "--skip-browser":
			options.WithBrowser = false
		case "--doctor":
			options.RunDoctor = true
		case "--skip-doctor":
			options.RunDoctor = false
		default:
			return hermesInstallOptions{}, fmt.Errorf("unknown option: %s", args[index])
		}
	}
	return options, nil
}

func hermesConfigurationCommands(setup cliIntegrationSetupResponse, apiKey, baseURL string) [][]string {
	return [][]string{
		{"config", "set", "OFFGRID_API_KEY", apiKey},
		// Hermes bridges custom config keys to the environment, but warns about
		// unknown top-level keys unless --force is supplied.
		{"config", "set", "OFFGRID_BASE_URL", baseURL, "--force"},
		{"config", "set", "model.default", setup.Integration.ModelID},
		{"config", "set", "model.provider", "offgrid"},
		{"config", "set", "model.base_url", baseURL},
		{"config", "set", "model.context_length", strconv.Itoa(setup.Integration.ContextWindow)},
	}
}

func findHermesExecutable(userHome string) (string, error) {
	if executable, err := hermesLookPath("hermes"); err == nil {
		return executable, nil
	}
	hermesHome := integrations.HermesHome(userHome)
	installDir := hermesInstallDir(userHome)
	candidates := []string{}
	if hermesRuntimeOS == "windows" {
		candidates = append(candidates,
			filepath.Join(hermesHome, "bin", "hermes.exe"),
			filepath.Join(hermesHome, "bin", "hermes.cmd"),
			filepath.Join(hermesHome, "hermes-agent", "venv", "Scripts", "hermes.exe"),
		)
	} else {
		candidates = append(candidates,
			filepath.Join(userHome, ".local", "bin", "hermes"),
			filepath.Join(installDir, "venv", "bin", "hermes"),
		)
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("hermes executable not found")
}

func confirmHermesInstall(assumeYes bool) (bool, error) {
	if assumeYes {
		return true, nil
	}
	if output.JSONMode {
		return false, fmt.Errorf("Hermes Agent is not installed; pass --yes to approve its official installer in JSON mode")
	}
	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return false, fmt.Errorf("Hermes Agent is not installed; rerun interactively or pass --yes")
	}
	fmt.Println()
	fmt.Printf("  %sHermes Agent is external software maintained by Nous Research.%s\n", brandPrimary+colorBold, colorReset)
	fmt.Printf("  OffGrid will download and run the official installer from:\n  %s%s%s\n", brandMuted, hermesInstallerURL(), colorReset)
	fmt.Print("\n  Continue? [Y/n] ")
	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "" || answer == "y" || answer == "yes", nil
}

func hermesInstallerURL() string {
	if hermesRuntimeOS == "windows" {
		return hermesWindowsInstallerURL
	}
	return hermesUnixInstallerURL
}

func installHermesRuntime() error {
	if err := validateHermesPlatform(hermesRuntimeOS, hermesRuntimeArch); err != nil {
		return err
	}
	installer, cleanup, err := downloadHermesInstaller(hermesInstallerURL())
	if err != nil {
		return err
	}
	defer cleanup()

	// Hermes exposes a supported stage protocol for managed installers. Keep
	// the required runtime independent from the much larger npm/browser stage,
	// so a registry timeout cannot invalidate a working agent installation.
	// The Unix prerequisites stage also tries to install optional Node/ffmpeg;
	// the required stages provision their own prerequisites on demand.
	for _, stage := range hermesCoreStages(hermesRuntimeOS) {
		hermesProgress("Hermes setup: " + stage)
		if err := runHermesStage(installer, stage); err != nil {
			return fmt.Errorf("official installer stage %s: %w", stage, err)
		}
	}
	return nil
}

func hermesCoreStages(goos string) []string {
	if goos == "windows" {
		return []string{"uv", "git", "repository", "python", "venv", "dependencies", "path", "config-templates", "bootstrap-marker"}
	}
	return []string{"repository", "venv", "python-deps", "path", "config", "complete"}
}

func installHermesOptionalTools() error {
	installer, cleanup, err := downloadHermesInstaller(hermesInstallerURL())
	if err != nil {
		return err
	}
	defer cleanup()
	if hermesRuntimeOS == "windows" {
		if err := runHermesStage(installer, "node"); err != nil {
			return fmt.Errorf("official installer optional node stage: %w", err)
		}
	}
	if err := runHermesStage(installer, "node-deps"); err != nil {
		return fmt.Errorf("official installer optional node-deps stage: %w", err)
	}
	return nil
}

func runHermesStage(installer, stage string) error {
	return runHermesInstallerFile(installer, hermesStageArgs(hermesRuntimeOS, stage)...)
}

func hermesStageArgs(goos, stage string) []string {
	if goos == "windows" {
		args := []string{"-Stage", stage, "-NonInteractive"}
		if installDir := strings.TrimSpace(os.Getenv("HERMES_INSTALL_DIR")); installDir != "" {
			args = append(args, "-InstallDir", installDir)
		}
		return args
	}
	args := []string{"--stage", stage, "--non-interactive", "--skip-setup"}
	if stage != "node-deps" {
		args = append(args, "--skip-browser")
	}
	return args
}

func runHermesInstallerFile(installer string, args ...string) error {
	var command *exec.Cmd
	if hermesRuntimeOS == "windows" {
		powershell, err := hermesLookPath("powershell.exe")
		if err != nil {
			powershell, err = hermesLookPath("pwsh")
			if err != nil {
				return fmt.Errorf("PowerShell is required to run the official Hermes installer")
			}
		}
		commandArgs := []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", installer}
		command = hermesExecCommand(powershell, append(commandArgs, args...)...)
	} else {
		bash, err := hermesLookPath("bash")
		if err != nil {
			return fmt.Errorf("bash is required to run the official Hermes installer")
		}
		command = hermesExecCommand(bash, append([]string{installer}, args...)...)
	}
	command.Stdin = os.Stdin
	command.Stdout = commandOutput()
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return err
	}
	return nil
}

func hermesInstallDir(userHome string) string {
	if configured := strings.TrimSpace(os.Getenv("HERMES_INSTALL_DIR")); configured != "" {
		return configured
	}
	return filepath.Join(integrations.HermesHome(userHome), "hermes-agent")
}

func validateHermesPlatform(goos, goarch string) error {
	switch goos {
	case "linux":
		if goarch == "amd64" || goarch == "arm64" {
			return nil
		}
	case "windows":
		if goarch == "amd64" || goarch == "arm64" {
			return nil
		}
	case "darwin":
		if goarch == "arm64" {
			return nil
		}
	}
	return fmt.Errorf("Hermes Agent does not support automatic installation on %s/%s", goos, goarch)
}

func downloadHermesInstaller(installerURL string) (string, func(), error) {
	parsed, err := url.Parse(installerURL)
	if err != nil || parsed.Scheme != "https" {
		return "", nil, fmt.Errorf("Hermes installer URL must use HTTPS")
	}
	response, err := httpClient.Get(installerURL)
	if err != nil {
		return "", nil, fmt.Errorf("download official installer: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("download official installer: HTTP %d", response.StatusCode)
	}
	if response.Request.URL.Scheme != "https" || !trustedHermesInstallerHost(response.Request.URL.Hostname()) {
		return "", nil, fmt.Errorf("official installer redirected to an untrusted host: %s", response.Request.URL.Hostname())
	}
	extension := ".sh"
	if hermesRuntimeOS == "windows" {
		extension = ".ps1"
	}
	file, err := os.CreateTemp("", "offgrid-hermes-installer-*"+extension)
	if err != nil {
		return "", nil, err
	}
	path := file.Name()
	cleanup := func() { _ = os.Remove(path) }
	written, copyErr := io.Copy(file, io.LimitReader(response.Body, maxHermesInstallerSize+1))
	closeErr := file.Close()
	if copyErr != nil {
		cleanup()
		return "", nil, copyErr
	}
	if closeErr != nil {
		cleanup()
		return "", nil, closeErr
	}
	if written > maxHermesInstallerSize {
		cleanup()
		return "", nil, fmt.Errorf("official installer exceeds the %d-byte safety limit", maxHermesInstallerSize)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		cleanup()
		return "", nil, err
	}
	return path, cleanup, nil
}

func trustedHermesInstallerHost(host string) bool {
	return strings.EqualFold(host, "hermes-agent.nousresearch.com") ||
		strings.EqualFold(host, "raw.githubusercontent.com")
}

func runHermesProcess(executable string, args, environment []string, quiet bool) error {
	command := hermesExecCommand(executable, args...)
	command.Stdin = os.Stdin
	command.Stderr = os.Stderr
	command.Env = environment
	if quiet {
		command.Stdout = os.Stderr
	} else {
		command.Stdout = os.Stdout
	}
	return command.Run()
}

func runHermesSmokeTest(executable string, args, environment []string, expected string) error {
	command := hermesExecCommand(executable, args...)
	command.Stdin = os.Stdin
	command.Env = environment
	var captured hermesOutputTail
	command.Stdout = io.MultiWriter(os.Stdout, &captured)
	command.Stderr = io.MultiWriter(os.Stderr, &captured)
	if err := command.Run(); err != nil {
		return err
	}
	if !containsExactOutputLine(captured.String(), expected) {
		return fmt.Errorf("Hermes exited without the expected model response")
	}
	return nil
}

// Hermes can stream a large response and writes stdout and stderr concurrently.
// Bound and synchronize the portion retained for the smoke-test assertion.
type hermesOutputTail struct {
	mu   sync.Mutex
	data []byte
}

const hermesOutputLimit = 64 << 10

func (tail *hermesOutputTail) Write(data []byte) (int, error) {
	tail.mu.Lock()
	defer tail.mu.Unlock()
	length := len(data)
	if length >= hermesOutputLimit {
		tail.data = append(tail.data[:0], data[length-hermesOutputLimit:]...)
		return length, nil
	}
	tail.data = append(tail.data, data...)
	if len(tail.data) > hermesOutputLimit {
		tail.data = append(tail.data[:0], tail.data[len(tail.data)-hermesOutputLimit:]...)
	}
	return length, nil
}

func (tail *hermesOutputTail) String() string {
	tail.mu.Lock()
	defer tail.mu.Unlock()
	return string(tail.data)
}

func containsExactOutputLine(value, expected string) bool {
	for _, line := range strings.Split(stripTerminalEscapes(value), "\n") {
		if strings.TrimSpace(line) == expected {
			return true
		}
	}
	return false
}

func stripTerminalEscapes(value string) string {
	var cleaned strings.Builder
	for index := 0; index < len(value); index++ {
		if value[index] != 0x1b || index+1 >= len(value) || value[index+1] != '[' {
			cleaned.WriteByte(value[index])
			continue
		}
		index += 2
		for index < len(value) {
			character := value[index]
			if character >= 0x40 && character <= 0x7e {
				break
			}
			index++
		}
	}
	return cleaned.String()
}

func mergedEnvironment(base []string, updates map[string]string) []string {
	result := append([]string(nil), base...)
	for key, value := range updates {
		prefix := key + "="
		filtered := result[:0]
		for _, item := range result {
			if !strings.EqualFold(strings.SplitN(item, "=", 2)[0]+"=", prefix) {
				filtered = append(filtered, item)
			}
		}
		result = append(filtered, prefix+value)
	}
	return result
}

func commandOutput() io.Writer {
	if output.JSONMode {
		return os.Stderr
	}
	return os.Stdout
}

func hermesProgress(message string) {
	if output.JSONMode {
		fmt.Fprintln(os.Stderr, message)
		return
	}
	printInfo(message)
}

func printHermesStatusLine(label string, ready bool, detail string) {
	color := brandError
	state := "blocked"
	if ready {
		color = brandSuccess
		state = "ready"
	}
	fmt.Printf("  %-18s %s%-8s%s %s\n", label, color, state, colorReset, detail)
}

func containsHelp(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}
