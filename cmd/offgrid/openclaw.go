package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/integrations"
	"github.com/takuphilchan/offgrid-llm/internal/output"
)

const (
	openClawUnixInstallerURL    = "https://openclaw.ai/install.sh"
	openClawWindowsInstallerURL = "https://openclaw.ai/install.ps1"
	maxOpenClawInstallerSize    = 4 << 20
)

type openClawOptions struct {
	ServerURL string
	BaseURL   string
	ModelID   string
	APIKey    string
	Force     bool
	Yes       bool
}

type openClawInstallResult struct {
	Installed  bool   `json:"installed"`
	Executable string `json:"executable"`
	PluginPath string `json:"plugin_path"`
	ModelID    string `json:"model_id"`
	BaseURL    string `json:"base_url"`
}

var openClawLookPath = exec.LookPath
var openClawExecCommand = exec.Command
var openClawRuntimeOS = runtime.GOOS

func handleOpenClaw(args []string) error {
	if len(args) == 0 {
		printOpenClawHelp()
		return nil
	}
	switch strings.ToLower(args[0]) {
	case "help", "-h", "--help":
		printOpenClawHelp()
		return nil
	case "install", "setup":
		if containsHelp(args[1:]) {
			printOpenClawHelp()
			return nil
		}
		return handleOpenClawInstall(args[1:])
	case "status", "check":
		return handleOpenClawStatus(args[1:])
	case "test", "verify":
		return handleOpenClawTest(args[1:])
	case "run", "exec":
		return runOpenClawAgent(args[1:])
	default:
		return fmt.Errorf("unknown OpenClaw command %q; run offgrid openclaw help", args[0])
	}
}

func printOpenClawHelp() {
	fmt.Println()
	fmt.Printf("  %sOpenClaw on OffGrid%s\n", brandPrimary+colorBold, colorReset)
	fmt.Printf("  %sInstall and run the external OpenClaw agent with private OffGrid inference%s\n\n", brandMuted, colorReset)
	fmt.Println("  offgrid openclaw install [--model <id>] [--yes] [--force]")
	fmt.Println("  offgrid openclaw status")
	fmt.Println("  offgrid openclaw test")
	fmt.Println("  offgrid openclaw run \"Summarize this directory\"")
	fmt.Println("  offgrid openclaw install --server <url> --base-url <url> --api-key <token>")
	fmt.Println()
}

func parseOpenClawOptions(args []string) (openClawOptions, error) {
	cfg, err := config.LoadWithPriority(os.Getenv("OFFGRID_CONFIG"))
	if err != nil {
		return openClawOptions{}, err
	}
	options := openClawOptions{
		ServerURL: fmt.Sprintf("http://127.0.0.1:%d", cfg.ServerPort),
		APIKey:    strings.TrimSpace(os.Getenv("OFFGRID_API_KEY")),
	}
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--server", "--base-url", "--model", "--api-key":
			flag := args[index]
			index++
			if index >= len(args) || strings.TrimSpace(args[index]) == "" {
				return openClawOptions{}, fmt.Errorf("%s requires a value", flag)
			}
			value := strings.TrimSpace(args[index])
			switch flag {
			case "--server":
				options.ServerURL = strings.TrimRight(value, "/")
			case "--base-url":
				options.BaseURL = strings.TrimRight(value, "/")
			case "--model":
				options.ModelID = value
			case "--api-key":
				options.APIKey = value
			}
		case "--yes", "-y":
			options.Yes = true
		case "--force":
			options.Force = true
		default:
			return openClawOptions{}, fmt.Errorf("unknown option: %s", args[index])
		}
	}
	return options, nil
}

func openClawSetup(options openClawOptions) (cliIntegrationSetupResponse, error) {
	baseURL := options.BaseURL
	if baseURL == "" {
		baseURL = options.ServerURL
	}
	endpoint := options.ServerURL + "/v1/integrations/openclaw/setup?" + (url.Values{
		"base_url": {baseURL},
		"model":    {options.ModelID},
	}).Encode()
	var setup cliIntegrationSetupResponse
	if err := getIntegrationJSONWithKey(endpoint, options.APIKey, &setup); err != nil {
		return setup, fmt.Errorf("cannot inspect OffGrid: %w; start the service and retry", err)
	}
	return setup, nil
}

func getIntegrationJSONWithKey(endpoint, apiKey string, target any) error {
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	if apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+apiKey)
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("service returned HTTP %d", response.StatusCode)
	}
	return json.NewDecoder(response.Body).Decode(target)
}

func handleOpenClawInstall(args []string) error {
	options, err := parseOpenClawOptions(args)
	if err != nil {
		return err
	}
	setup, err := openClawSetup(options)
	if err != nil {
		return err
	}
	if !setup.Integration.Ready {
		return fmt.Errorf("OffGrid is reachable but OpenClaw is not ready: %s", strings.Join(setup.Integration.Warnings, "; "))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	executable, err := findOpenClawExecutable(home)
	if err != nil {
		accepted, confirmErr := confirmOpenClawInstall(options.Yes)
		if confirmErr != nil {
			return confirmErr
		}
		if !accepted {
			printInfo("Installation cancelled; no external software was changed")
			return nil
		}
		if err := installOpenClawRuntime(); err != nil {
			return fmt.Errorf("official OpenClaw installer failed: %w", err)
		}
		executable, err = findOpenClawExecutable(home)
		if err != nil {
			return fmt.Errorf("OpenClaw installed but its command is not on PATH; open a new terminal and retry offgrid openclaw install")
		}
	}
	// A local plugin executes code in OpenClaw. Consent is explicit even when
	// the OpenClaw CLI itself is already present.
	accepted, err := confirmOpenClawPlugin(options.Yes)
	if err != nil {
		return err
	}
	if !accepted {
		printInfo("OpenClaw provider registration cancelled")
		return nil
	}
	plugin, err := integrations.InstallPluginBundle("openclaw", home, options.Force)
	if err != nil {
		return err
	}
	if err := ensureOpenClawPlugin(executable, plugin.Path, options.Force); err != nil {
		return err
	}
	var rendered struct {
		Models struct {
			Providers map[string]json.RawMessage `json:"providers"`
		} `json:"models"`
	}
	if err := json.Unmarshal([]byte(setup.Setup.Content), &rendered); err != nil {
		return fmt.Errorf("invalid generated OpenClaw config: %w", err)
	}
	var provider map[string]any
	if err := json.Unmarshal(rendered.Models.Providers["offgrid"], &provider); err != nil {
		return fmt.Errorf("invalid generated OffGrid provider: %w", err)
	}
	configuredBaseURL, ok := provider["baseUrl"].(string)
	if !ok || configuredBaseURL == "" {
		return fmt.Errorf("generated OffGrid provider has no base URL")
	}
	apiKey := options.APIKey
	if apiKey == "" {
		apiKey = "offgrid-local"
	}
	provider["apiKey"] = apiKey
	patch, err := json.Marshal(map[string]any{"models": map[string]any{
		"mode": "merge", "providers": map[string]any{"offgrid": provider},
	}})
	if err != nil {
		return err
	}
	for _, dryRun := range []bool{true, false} {
		command := []string{"config", "patch", "--stdin"}
		if dryRun {
			command = append(command, "--dry-run")
		}
		if err := runOpenClawCommand(executable, command, bytes.NewReader(patch), nil); err != nil {
			return fmt.Errorf("cannot configure OpenClaw provider: %w", err)
		}
	}
	if configured, problem := inspectOpenClawProvider(executable, configuredBaseURL, setup.Integration.ModelID); !configured {
		return fmt.Errorf("OpenClaw saved an incomplete OffGrid provider: %s", problem)
	}
	result := openClawInstallResult{
		Installed: true, Executable: executable, PluginPath: plugin.Path,
		ModelID: setup.Integration.ModelID, BaseURL: configuredBaseURL,
	}
	if outputJSON(result) {
		return nil
	}
	printSuccess("OpenClaw is configured to use OffGrid")
	fmt.Printf("  %sModel%s     %s\n", brandMuted, colorReset, result.ModelID)
	fmt.Printf("  %sEndpoint%s  %s\n", brandMuted, colorReset, result.BaseURL)
	fmt.Printf("  %sVerify%s    offgrid openclaw test\n", brandMuted, colorReset)
	fmt.Printf("  %sRun%s       offgrid openclaw run \"your task\"\n\n", brandMuted, colorReset)
	return nil
}

func confirmOpenClawInstall(yes bool) (bool, error) {
	return confirmOpenClawAction(yes, "OpenClaw is external software. OffGrid will run the official installer from "+openClawInstallerURL()+". Continue?")
}

func confirmOpenClawPlugin(yes bool) (bool, error) {
	return confirmOpenClawAction(yes, "The first-party OffGrid provider is local executable code with OpenClaw text-inference capability. Register it in OpenClaw?")
}

func confirmOpenClawAction(yes bool, prompt string) (bool, error) {
	if yes {
		return true, nil
	}
	if output.JSONMode {
		return false, fmt.Errorf("interactive approval is required; pass --yes to approve external installation and provider registration")
	}
	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return false, fmt.Errorf("interactive approval is required; rerun in a terminal or pass --yes")
	}
	fmt.Printf("\n  %s\n  Continue? [Y/n] ", prompt)
	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "" || answer == "y" || answer == "yes", nil
}

func openClawInstallerURL() string {
	if openClawRuntimeOS == "windows" {
		return openClawWindowsInstallerURL
	}
	return openClawUnixInstallerURL
}

func installOpenClawRuntime() error {
	installerURL := openClawInstallerURL()
	response, err := httpClient.Get(installerURL)
	if err != nil {
		return fmt.Errorf("download official installer: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("official installer returned HTTP %d", response.StatusCode)
	}
	if response.Request.URL.Scheme != "https" || !trustedOpenClawInstallerHost(response.Request.URL.Hostname()) {
		return fmt.Errorf("official installer redirected to untrusted host %q", response.Request.URL.Hostname())
	}
	extension := ".sh"
	if openClawRuntimeOS == "windows" {
		extension = ".ps1"
	}
	file, err := os.CreateTemp("", "offgrid-openclaw-installer-*"+extension)
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	written, copyErr := io.Copy(file, io.LimitReader(response.Body, maxOpenClawInstallerSize+1))
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written > maxOpenClawInstallerSize {
		return fmt.Errorf("official installer exceeds %d bytes", maxOpenClawInstallerSize)
	}
	if err := os.Chmod(file.Name(), 0o700); err != nil {
		return err
	}
	var command *exec.Cmd
	if openClawRuntimeOS == "windows" {
		powershell, err := openClawLookPath("powershell.exe")
		if err != nil {
			return fmt.Errorf("PowerShell is required for the official Windows OpenClaw installer")
		}
		command = openClawExecCommand(powershell, "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", file.Name(), "-NoOnboard")
	} else {
		bash, err := openClawLookPath("bash")
		if err != nil {
			return fmt.Errorf("bash is required for the official OpenClaw installer")
		}
		command = openClawExecCommand(bash, file.Name(), "--no-onboard")
	}
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, commandOutput(), os.Stderr
	return command.Run()
}

func trustedOpenClawInstallerHost(host string) bool {
	return strings.EqualFold(host, "openclaw.ai") || strings.EqualFold(host, "raw.githubusercontent.com")
}

func findOpenClawExecutable(home string) (string, error) {
	if configured := strings.TrimSpace(os.Getenv("OFFGRID_OPENCLAW_BIN")); configured != "" {
		if info, err := os.Stat(configured); err == nil && !info.IsDir() {
			return configured, nil
		}
		return "", fmt.Errorf("OFFGRID_OPENCLAW_BIN does not point to a file: %s", configured)
	}
	if executable, err := openClawLookPath("openclaw"); err == nil {
		return executable, nil
	}
	candidates := []string{
		filepath.Join(home, ".local", "bin", "openclaw"),
		filepath.Join(home, ".npm-global", "bin", "openclaw"),
		filepath.Join(home, ".openclaw", "bin", "openclaw"),
		filepath.Join(home, "AppData", "Roaming", "npm", "openclaw.cmd"),
	}
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	return "", fmt.Errorf("OpenClaw executable not found")
}

func openClawCommand(executable string, args []string) (*exec.Cmd, error) {
	if strings.EqualFold(filepath.Ext(executable), ".mjs") {
		node, err := openClawLookPath("node")
		if err != nil {
			return nil, fmt.Errorf("Node.js is required to launch OpenClaw")
		}
		return openClawExecCommand(node, append([]string{executable}, args...)...), nil
	}
	if openClawRuntimeOS == "windows" && strings.EqualFold(filepath.Ext(executable), ".cmd") {
		entry := filepath.Join(filepath.Dir(executable), "node_modules", "openclaw", "openclaw.mjs")
		if _, err := os.Stat(entry); err != nil {
			return nil, fmt.Errorf("cannot locate the OpenClaw Node entry beside %s", executable)
		}
		node, err := openClawLookPath("node")
		if err != nil {
			return nil, fmt.Errorf("Node.js is required to launch OpenClaw")
		}
		return openClawExecCommand(node, append([]string{entry}, args...)...), nil
	}
	return openClawExecCommand(executable, args...), nil
}

func runOpenClawCommand(executable string, args []string, stdin io.Reader, stdout io.Writer) error {
	command, err := openClawCommand(executable, args)
	if err != nil {
		return err
	}
	if stdin == nil {
		stdin = os.Stdin
	}
	if stdout == nil {
		stdout = commandOutput()
	}
	command.Stdin, command.Stdout, command.Stderr = stdin, stdout, os.Stderr
	return command.Run()
}

func inspectOpenClawPlugin(executable string) (rootDir string, loaded bool, err error) {
	command, err := openClawCommand(executable, []string{"plugins", "inspect", "offgrid", "--json"})
	if err != nil {
		return "", false, err
	}
	var stdout bytes.Buffer
	command.Stdout, command.Stderr = &stdout, io.Discard
	if err := command.Run(); err != nil {
		return "", false, err
	}
	var result struct {
		Plugin struct {
			RootDir string `json:"rootDir"`
			Status  string `json:"status"`
		} `json:"plugin"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return "", false, err
	}
	return result.Plugin.RootDir, result.Plugin.Status == "loaded", nil
}

func inspectOpenClawProvider(executable, expectedBaseURL, expectedModelID string) (bool, string) {
	command, err := openClawCommand(executable, []string{"config", "get", "models.providers.offgrid", "--json"})
	if err != nil {
		return false, err.Error()
	}
	var stdout bytes.Buffer
	command.Stdout, command.Stderr = &stdout, io.Discard
	if err := command.Run(); err != nil {
		return false, "OffGrid provider is not configured in OpenClaw"
	}
	var provider struct {
		BaseURL string `json:"baseUrl"`
		Models  []struct {
			ID string `json:"id"`
		} `json:"models"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &provider); err != nil {
		return false, "OpenClaw returned an invalid OffGrid provider configuration"
	}
	if strings.TrimRight(provider.BaseURL, "/") != strings.TrimRight(expectedBaseURL, "/") {
		return false, fmt.Sprintf("OpenClaw points at %s, expected %s", provider.BaseURL, expectedBaseURL)
	}
	for _, model := range provider.Models {
		if model.ID == expectedModelID {
			return true, ""
		}
	}
	return false, fmt.Sprintf("model %s is not configured in OpenClaw", expectedModelID)
}

func expectedOpenClawBaseURL(options openClawOptions) string {
	baseURL := options.BaseURL
	if baseURL == "" {
		baseURL = options.ServerURL
	}
	return strings.TrimSuffix(strings.TrimRight(baseURL, "/"), "/v1") + "/v1"
}

func ensureOpenClawPlugin(executable, pluginPath string, force bool) error {
	if rootDir, loaded, err := inspectOpenClawPlugin(executable); err == nil && loaded {
		if filepath.Clean(rootDir) == filepath.Clean(pluginPath) {
			return nil
		}
		if !force {
			return fmt.Errorf("an OffGrid OpenClaw plugin is already installed at %s; use --force only if you want to replace it", rootDir)
		}
	}
	if err := runOpenClawCommand(executable, []string{"plugins", "install", "--link", "--force", "--accept-capabilities", pluginPath}, nil, nil); err != nil {
		return fmt.Errorf("register OffGrid provider in OpenClaw: %w", err)
	}
	rootDir, loaded, err := inspectOpenClawPlugin(executable)
	if err != nil || !loaded {
		return fmt.Errorf("OpenClaw accepted the plugin but it did not load; run openclaw plugins inspect offgrid --runtime --json")
	}
	if filepath.Clean(rootDir) != filepath.Clean(pluginPath) {
		return fmt.Errorf("OpenClaw still loads another offgrid plugin at %s; remove the duplicate plugin path from OpenClaw config, then retry", rootDir)
	}
	return nil
}

func handleOpenClawStatus(args []string) error {
	options, err := parseOpenClawOptions(args)
	if err != nil {
		return err
	}
	home, _ := os.UserHomeDir()
	executable, runtimeErr := findOpenClawExecutable(home)
	pluginPath := ""
	pluginLoaded := false
	if runtimeErr == nil {
		pluginPath, pluginLoaded, _ = inspectOpenClawPlugin(executable)
	}
	setup, serviceErr := openClawSetup(options)
	configured := false
	configurationProblem := ""
	if runtimeErr == nil && serviceErr == nil {
		configured, configurationProblem = inspectOpenClawProvider(executable, expectedOpenClawBaseURL(options), setup.Integration.ModelID)
	}
	ready := runtimeErr == nil && pluginLoaded && configured && serviceErr == nil && setup.Integration.Ready
	status := map[string]any{"installed": runtimeErr == nil, "plugin_loaded": pluginLoaded, "plugin_path": pluginPath, "configured": configured, "service_ready": serviceErr == nil && setup.Integration.Ready, "ready": ready}
	if configurationProblem != "" {
		status["config_error"] = configurationProblem
	}
	if serviceErr == nil {
		status["model_id"] = setup.Integration.ModelID
		status["warnings"] = setup.Integration.Warnings
	} else {
		status["error"] = serviceErr.Error()
	}
	if !outputJSON(status) {
		fmt.Println()
		printHermesStatusLine("OpenClaw runtime", runtimeErr == nil, executable)
		printHermesStatusLine("OffGrid provider", pluginLoaded, pluginPath)
		printHermesStatusLine("Model config", configured, setup.Integration.ModelID)
		printHermesStatusLine("OffGrid service", serviceErr == nil && setup.Integration.Ready, options.ServerURL)
		if configurationProblem != "" {
			printWarning(configurationProblem)
		}
		if serviceErr != nil {
			printWarning(serviceErr.Error())
		} else {
			for _, warning := range setup.Integration.Warnings {
				printWarning(warning)
			}
		}
		fmt.Println()
	}
	if !ready {
		return fmt.Errorf("OpenClaw is not ready; run offgrid openclaw install")
	}
	return nil
}

func handleOpenClawTest(args []string) error {
	options, err := parseOpenClawOptions(args)
	if err != nil {
		return err
	}
	setup, err := openClawSetup(options)
	if err != nil {
		return err
	}
	if !setup.Integration.Ready {
		return fmt.Errorf("OpenClaw is not ready: %s", strings.Join(setup.Integration.Warnings, "; "))
	}
	home, _ := os.UserHomeDir()
	executable, err := findOpenClawExecutable(home)
	if err != nil {
		return fmt.Errorf("OpenClaw is not installed; run offgrid openclaw install")
	}
	if _, loaded, err := inspectOpenClawPlugin(executable); err != nil || !loaded {
		return fmt.Errorf("OffGrid provider is not loaded in OpenClaw; run offgrid openclaw install")
	}
	if configured, problem := inspectOpenClawProvider(executable, expectedOpenClawBaseURL(options), setup.Integration.ModelID); !configured {
		return fmt.Errorf("OpenClaw is not configured for this OffGrid model: %s; run offgrid openclaw install", problem)
	}
	// This checks the provider transport without loading OpenClaw's full agent
	// prompt, tool inventory, and workspace. A small local model can answer a
	// model.run probe even when it is too slow for a full agent task.
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	command, err := openClawCommand(executable, []string{
		"infer", "model", "run", "--local",
		"--model", "offgrid/" + setup.Integration.ModelID,
		"--prompt", "Reply briefly to confirm the model is reachable.",
		"--thinking", "off", "--json",
	})
	if err != nil {
		return err
	}
	command = exec.CommandContext(ctx, command.Path, command.Args[1:]...)
	var stdout bytes.Buffer
	command.Stdout, command.Stderr = &stdout, os.Stderr
	runErr := command.Run()
	if runErr != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("OpenClaw inference timed out after 180 seconds; inspect OffGrid model performance and logs")
		}
		return fmt.Errorf("OpenClaw inference failed: %w; inspect the model and OffGrid logs", runErr)
	}
	if _, err := parseOpenClawInferResult(stdout.Bytes(), setup.Integration.ModelID); err != nil {
		return fmt.Errorf("OpenClaw did not return a usable OffGrid model response: %w", err)
	}
	printSuccess("OpenClaw reached OffGrid and returned a model response (provider smoke test)")
	return nil
}

func parseOpenClawInferResult(data []byte, expectedModel string) (string, error) {
	var result struct {
		OK       bool   `json:"ok"`
		Provider string `json:"provider"`
		Model    string `json:"model"`
		Outputs  []struct {
			Text string `json:"text"`
		} `json:"outputs"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("invalid OpenClaw JSON output: %w", err)
	}
	if !result.OK || result.Provider != "offgrid" || result.Model != expectedModel {
		return "", fmt.Errorf("provider/model mismatch or failed inference (provider=%q, model=%q, ok=%t)", result.Provider, result.Model, result.OK)
	}
	for _, output := range result.Outputs {
		if reply := strings.TrimSpace(output.Text); reply != "" {
			return reply, nil
		}
	}
	return "", fmt.Errorf("model returned no text")
}

func runOpenClawAgent(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: offgrid openclaw run \"your task\"")
	}
	options, err := parseOpenClawOptions(args[1:])
	if err != nil {
		return err
	}
	setup, err := openClawSetup(options)
	if err != nil {
		return err
	}
	if !setup.Integration.Ready {
		return fmt.Errorf("OffGrid is not ready for OpenClaw: %s", strings.Join(setup.Integration.Warnings, "; "))
	}
	home, _ := os.UserHomeDir()
	executable, err := findOpenClawExecutable(home)
	if err != nil {
		return fmt.Errorf("OpenClaw is not installed; run offgrid openclaw install")
	}
	if configured, problem := inspectOpenClawProvider(executable, expectedOpenClawBaseURL(options), setup.Integration.ModelID); !configured {
		return fmt.Errorf("OpenClaw is not configured for this OffGrid model: %s; run offgrid openclaw install", problem)
	}
	command := []string{"agent", "exec", args[0], "--model", "offgrid/" + setup.Integration.ModelID, "--local-model-lean", "--code-mode", "direct"}
	return runOpenClawCommand(executable, command, nil, os.Stdout)
}
