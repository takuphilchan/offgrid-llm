package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// LogDependencies supplies read-only host paths. Process execution goes
// through CommandRunner with named, bounded capabilities.
type LogDependencies struct {
	HomeDir func() (string, error)
	Stat    func(string) (os.FileInfo, error)
}

func RunLogsCommand(ctx context.Context, args []string, env Environment) error {
	if env.Logs.HomeDir == nil || env.Logs.Stat == nil {
		return fmt.Errorf("logs host adapter unavailable")
	}
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = nil
	}
	follow, lines := false, 50
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "-f", "--follow":
			follow = true
		case "-n", "--lines":
			if index+1 < len(args) {
				fmt.Sscanf(args[index+1], "%d", &lines)
				index++
			}
		}
	}
	if len(args) == 0 || (len(args) == 1 && args[0] == "") {
		printLogsHelp(env)
		return nil
	}
	// The preexisting command accepted arbitrary integers; the process boundary
	// now refuses unreasonable tail/journalctl counts before spawning anything.
	if lines < 1 || lines > 10000 {
		return Usage("log line count must be between 1 and 10000")
	}
	s := env.Style
	var systemdOutput bytes.Buffer
	if err := env.Exec.Run(ctx, CommandRequest{Capability: "systemd-status", Stdout: &systemdOutput, Stderr: io.Discard}); err == nil && strings.TrimSpace(systemdOutput.String()) == "active" {
		if follow {
			fmt.Fprintf(env.Stdout, "%sFollowing logs (Ctrl+C to stop)...%s\n\n", s.Muted, s.Reset)
		}
		err := env.Exec.Run(ctx, CommandRequest{Capability: "journalctl-logs", Args: []string{strconv.Itoa(lines), strconv.FormatBool(follow)}, Stdin: env.Stdin, Stdout: env.Stdout, Stderr: env.Stderr})
		if ctx.Err() != nil {
			return ctx.Err()
		}
		_ = err // Legacy logs ignored nonzero journalctl status.
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	homeDir, _ := env.Logs.HomeDir()
	offgridLogDir := filepath.Join(homeDir, ".offgrid", "logs")
	logFiles := []string{filepath.Join(offgridLogDir, "server.log"), "/var/log/offgrid/server.log", "/tmp/offgrid-server.log"}
	var foundLogFile string
	for _, candidate := range logFiles {
		if _, err := env.Logs.Stat(candidate); err == nil {
			foundLogFile = candidate
			break
		}
	}
	if foundLogFile == "" {
		fmt.Fprintln(env.Stdout)
		Presenter{Stdout: env.Stdout, Stderr: env.Stderr, Style: s}.Info("No log files found. The server may be running in foreground mode.")
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sTry these alternatives:%s\n", s.Muted, s.Reset)
		fmt.Fprintf(env.Stdout, "    %s1.%s Run offgrid serve in a terminal to see live output\n", s.Muted, s.Reset)
		fmt.Fprintf(env.Stdout, "    %s2.%s Use systemd: sudo journalctl -u offgrid-llm -f\n", s.Muted, s.Reset)
		fmt.Fprintf(env.Stdout, "    %s3.%s Check: %s\n", s.Muted, s.Reset, offgridLogDir)
		fmt.Fprintln(env.Stdout)
		return nil
	}
	fmt.Fprintf(env.Stdout, "%sLog file: %s%s\n\n", s.Muted, foundLogFile, s.Reset)
	if follow {
		fmt.Fprintf(env.Stdout, "%sFollowing logs (Ctrl+C to stop)...%s\n\n", s.Muted, s.Reset)
	}
	err := env.Exec.Run(ctx, CommandRequest{Capability: "tail-logs", Args: []string{foundLogFile, strconv.Itoa(lines), strconv.FormatBool(follow)}, Stdin: env.Stdin, Stdout: env.Stdout, Stderr: env.Stderr})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	_ = err // Legacy logs ignored nonzero tail status.
	return nil
}

func printLogsHelp(env Environment) {
	s := env.Style
	fmt.Fprintln(env.Stdout)
	fmt.Fprintf(env.Stdout, "  %s%s Logs%s\n", s.Primary+s.Bold, s.Circle, s.Reset)
	fmt.Fprintf(env.Stdout, "  %sView server logs%s\n", s.Dim, s.Reset)
	fmt.Fprintln(env.Stdout)
	fmt.Fprintf(env.Stdout, "  %sUsage%s  offgrid logs [options]\n", s.Dim, s.Reset)
	fmt.Fprintln(env.Stdout)
	fmt.Fprintf(env.Stdout, "  %sOptions%s\n", s.Primary, s.Reset)
	fmt.Fprintf(env.Stdout, "    %-20s %sFollow log output (like tail -f)%s\n", "-f, --follow", s.Dim, s.Reset)
	fmt.Fprintf(env.Stdout, "    %-20s %sNumber of lines to show (default: 50)%s\n", "-n, --lines <num>", s.Dim, s.Reset)
	fmt.Fprintln(env.Stdout)
	fmt.Fprintf(env.Stdout, "  %sExamples%s\n", s.Primary, s.Reset)
	fmt.Fprintf(env.Stdout, "    %s$%s offgrid logs                   %s# Show recent logs%s\n", s.Dim, s.Reset, s.Muted, s.Reset)
	fmt.Fprintf(env.Stdout, "    %s$%s offgrid logs -f                %s# Follow logs in real-time%s\n", s.Dim, s.Reset, s.Muted, s.Reset)
	fmt.Fprintf(env.Stdout, "    %s$%s offgrid logs -n 100            %s# Show last 100 lines%s\n", s.Dim, s.Reset, s.Muted, s.Reset)
	fmt.Fprintln(env.Stdout)
}

func logsCommand(ctx context.Context, args []string, env Environment) error {
	return RunLogsCommand(ctx, args, env)
}
