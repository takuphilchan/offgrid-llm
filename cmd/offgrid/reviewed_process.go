package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/config"
)

// reviewedProcessRunner is the host-owned implementation of named CLI process
// capabilities. It never treats a command name or shell line as model input.
type reviewedProcessRunner struct{}

func (reviewedProcessRunner) Run(ctx context.Context, request cli.CommandRequest) error {
	program, args, err := reviewedProcessSpec(request)
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, program, args...)
	command.Stdin = request.Stdin
	command.Stdout = request.Stdout
	command.Stderr = request.Stderr
	return command.Run()
}

func reviewedProcessSpec(request cli.CommandRequest) (string, []string, error) {
	switch request.Capability {
	case "llama-systemd-active":
		if len(request.Args) != 0 {
			return "", nil, fmt.Errorf("llama service status accepts no arguments")
		}
		return "systemctl", []string{"is-active", "llama-server"}, nil
	case "llama-systemd-status":
		if len(request.Args) != 0 {
			return "", nil, fmt.Errorf("llama service status accepts no arguments")
		}
		return "systemctl", []string{"status", "llama-server"}, nil
	case "llama-systemd-restart":
		if len(request.Args) != 0 {
			return "", nil, fmt.Errorf("llama service restart accepts no arguments")
		}
		return "sudo", []string{"systemctl", "restart", "llama-server"}, nil
	case "llama-active-model":
		if len(request.Args) != 0 {
			return "", nil, fmt.Errorf("llama active-model write accepts no arguments")
		}
		path := config.GetDefaultPaths().ActiveModel
		if path == "" {
			return "", nil, fmt.Errorf("llama active-model path unavailable")
		}
		return "sudo", []string{"tee", path}, nil
	case "llama-process-status":
		if len(request.Args) != 0 {
			return "", nil, fmt.Errorf("llama process status accepts no arguments")
		}
		return "pgrep", []string{"-x", "llama-server"}, nil
	case "open-computer-workspace":
		if len(request.Args) != 1 || !reviewedComputerLink(request.Args[0]) {
			return "", nil, fmt.Errorf("invalid OffGrid computer workspace link")
		}
		link := request.Args[0]
		switch runtime.GOOS {
		case "windows":
			return "rundll32.exe", []string{"url.dll,FileProtocolHandler", link}, nil
		case "darwin":
			return "open", []string{link}, nil
		default:
			return "xdg-open", []string{link}, nil
		}
	case "systemd-status":
		if len(request.Args) != 0 {
			return "", nil, fmt.Errorf("systemd status does not accept arguments")
		}
		return "systemctl", []string{"is-active", "offgrid-llm"}, nil
	case "journalctl-logs":
		if len(request.Args) != 2 {
			return "", nil, fmt.Errorf("journalctl requires line count and follow flag")
		}
		lines, err := reviewedLogLines(request.Args[0])
		if err != nil {
			return "", nil, err
		}
		switch request.Args[1] {
		case "true":
			return "journalctl", []string{"-u", "offgrid-llm", "-f", "-n", lines}, nil
		case "false":
			return "journalctl", []string{"-u", "offgrid-llm", "-n", lines, "--no-pager"}, nil
		}
		return "", nil, fmt.Errorf("invalid journalctl follow flag")
	case "tail-logs":
		if len(request.Args) != 3 {
			return "", nil, fmt.Errorf("tail requires a selected log, line count and follow flag")
		}
		if !reviewedLogPath(request.Args[0]) {
			return "", nil, fmt.Errorf("tail log path is not an approved candidate")
		}
		lines, err := reviewedLogLines(request.Args[1])
		if err != nil {
			return "", nil, err
		}
		switch request.Args[2] {
		case "true":
			return "tail", []string{"-f", "-n", lines, request.Args[0]}, nil
		case "false":
			return "tail", []string{"-n", lines, request.Args[0]}, nil
		}
		return "", nil, fmt.Errorf("invalid tail follow flag")
	default:
		return "", nil, fmt.Errorf("unknown process capability %q", request.Capability)
	}
}

func reviewedComputerLink(link string) bool {
	if link == "offgrid://computer" {
		return true
	}
	const prefix = "offgrid://computer?task=run-"
	if !strings.HasPrefix(link, prefix) || len(link) != len(prefix)+32 {
		return false
	}
	for _, value := range link[len(prefix):] {
		if !strings.ContainsRune("0123456789abcdef", value) {
			return false
		}
	}
	return true
}

func reviewedLogLines(value string) (string, error) {
	lines, err := strconv.Atoi(value)
	if err != nil || lines < 1 || lines > 10000 {
		return "", fmt.Errorf("log line count must be between 1 and 10000")
	}
	return strconv.Itoa(lines), nil
}

func reviewedLogPath(path string) bool {
	if path == "/var/log/offgrid/server.log" || path == "/tmp/offgrid-server.log" {
		return true
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	return filepath.Clean(path) == filepath.Join(home, ".offgrid", "logs", "server.log")
}
