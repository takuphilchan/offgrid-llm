// offgrid-executor is a private host operation worker, not an agent or a network
// command server. Its grant file is prepared locally by an approving supervisor.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "executor unavailable:", err)
		os.Exit(1)
	}
}

var Version = "development"

func run(args []string) error {
	flags := flag.NewFlagSet("offgrid-executor", flag.ContinueOnError)
	stdio := flags.Bool("stdio", false, "Use supervised private stdio")
	connect := flags.Bool("connect", false, "Use native application private enrollment IPC")
	grant := flags.String("grant-file", "", "Locally approved session configuration")
	state := flags.String("state-dir", "", "Private dispatch journal directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *connect {
		if *stdio || *grant != "" || flags.NArg() != 0 || !filepath.IsAbs(*state) {
			return execution.ErrAuthority
		}
		return connectNativeHost(*state)
	}
	if !*stdio || flags.NArg() != 0 || !filepath.IsAbs(*grant) || !filepath.IsAbs(*state) {
		return errors.New("explicit stdio, local grant and state directory required")
	}
	token := os.Getenv("OFFGRID_EXECUTOR_TOKEN")
	if len(token) != 64 {
		return errors.New("private supervisor credential required")
	}
	info, err := os.Lstat(*grant)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		return errors.New("grant file must be private and regular")
	}
	if err = execution.CheckPrivatePath(*grant); err != nil {
		return errors.New("grant file authority is not private to this account")
	}
	f, err := os.Open(*grant)
	if err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(f, execution.MaxFrameBytes+1))
	f.Close()
	if err != nil || len(data) > execution.MaxFrameBytes || jsonwire.Validate(data) != nil {
		return errors.New("invalid local grant")
	}
	var config execution.Context
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&config) != nil {
		return errors.New("invalid local grant")
	}
	if err = execution.PrivateStateOutsideRoot(config.Root, *grant, *state); err != nil {
		return errors.New("grant and journal must be outside the approved file workspace")
	}
	session, err := execution.OpenSession(config)
	if err != nil {
		return err
	}
	defer session.Close()
	if err = execution.EnsurePrivateDirectory(*state); err != nil {
		return errors.New("journal directory must be private to this account")
	}
	journal, err := execution.OpenJournal(*state)
	if err != nil {
		return err
	}
	defer journal.Close()
	processes := execution.NewProcessManager(session)
	if config.Backend == "isolated_linux" {
		processes, err = execution.NewIsolatedProcessManager(context.Background(), session, journal)
		if err != nil {
			return err
		}
	}
	worker := execution.Worker{Session: session, Journal: journal, Processes: processes, Token: token, Version: Version}
	if config.Permissions.Processes {
		worker.AuthorizeProcess = execution.TrustedProcessPolicy
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	return worker.Serve(ctx, os.Stdin, os.Stdout)
}
