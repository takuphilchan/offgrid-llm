package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/output"
)

const (
	openClawUnixInstallerURL    = "https://openclaw.ai/install.sh"
	openClawWindowsInstallerURL = "https://openclaw.ai/install.ps1"
	maxOpenClawInstallerSize    = 4 << 20
)

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
	fmt.Println("  offgrid openclaw run \"Summarize this directory\" [--model <id>] [--timeout <seconds>]")
	fmt.Println("  offgrid openclaw install --server <url> --base-url <url> --api-key <token>")
	fmt.Println()
}

func handleOpenClawInstall(args []string) error { return managedAlias("configure", "openclaw", args) }

func confirmOpenClawInstall(yes bool) (bool, error) {
	return confirmOpenClawAction(yes, "OpenClaw is external software. OffGrid will run the official installer from "+openClawInstallerURL()+". Continue?")
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

func installOpenClawRuntime(ctx context.Context) error {
	installerURL := openClawInstallerURL()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, installerURL, nil)
	if err != nil {
		return err
	}
	response, err := httpClient.Do(request)
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
	if err := validateIntegrationArgs("OpenClaw installer", command.Args[1:]); err != nil {
		return err
	}
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, commandOutput(), os.Stderr
	return runIntegrationProcess(bindIntegrationContext(ctx, command))
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
	if err := validateIntegrationArgs("OpenClaw", append([]string{executable}, args...)); err != nil {
		return nil, err
	}
	if strings.EqualFold(filepath.Ext(executable), ".mjs") {
		node, err := openClawLookPath("node")
		if err != nil {
			return nil, fmt.Errorf("Node.js is required to launch OpenClaw")
		}
		if err := validateIntegrationArgs("OpenClaw", append([]string{node, executable}, args...)); err != nil {
			return nil, err
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
		if err := validateIntegrationArgs("OpenClaw", append([]string{node, entry}, args...)); err != nil {
			return nil, err
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
	return runIntegrationProcess(command)
}

func inspectOpenClawPlugin(ctx context.Context, executable string) (rootDir string, loaded bool, err error) {
	command, err := openClawCommand(executable, []string{"plugins", "inspect", "offgrid", "--json"})
	if err != nil {
		return "", false, err
	}
	var stdout bytes.Buffer
	command.Stdout, command.Stderr = &stdout, io.Discard
	if err := runIntegrationProcess(bindIntegrationContext(ctx, command)); err != nil {
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

func ensureOpenClawPlugin(ctx context.Context, executable, pluginPath string, force bool) error {
	if rootDir, loaded, err := inspectOpenClawPlugin(ctx, executable); err == nil && loaded {
		if filepath.Clean(rootDir) == filepath.Clean(pluginPath) {
			return nil
		}
		if !force {
			return fmt.Errorf("an OffGrid OpenClaw plugin is already installed at %s; use --force only if you want to replace it", rootDir)
		}
	}
	if err := externalCommand(ctx, openClawCommand, executable, []string{"plugins", "install", "--link", "--force", "--accept-capabilities", pluginPath}, nil, os.Stdin, commandOutput(), os.Stderr); err != nil {
		return fmt.Errorf("register OffGrid provider in OpenClaw: %w", err)
	}
	rootDir, loaded, err := inspectOpenClawPlugin(ctx, executable)
	if err != nil || !loaded {
		return fmt.Errorf("OpenClaw accepted the plugin but it did not load; run openclaw plugins inspect offgrid --runtime --json")
	}
	if filepath.Clean(rootDir) != filepath.Clean(pluginPath) {
		return fmt.Errorf("OpenClaw still loads another offgrid plugin at %s; remove the duplicate plugin path from OpenClaw config, then retry", rootDir)
	}
	return nil
}

func handleOpenClawStatus(args []string) error { return managedAlias("status", "openclaw", args) }

func handleOpenClawTest(args []string) error { return managedAlias("test", "openclaw", args) }

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

func runOpenClawAgent(args []string) error { return managedAlias("run", "openclaw", args) }
