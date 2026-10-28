// executor-pack builds a trusted application asset pack. It is a developer /
// release tool, not a model tool, installer, runtime download or agent executor.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "executor pack:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("executor-pack", flag.ContinueOnError)
	output := flags.String("output", "", "Absolute trusted asset directory")
	version := flags.String("version", "", "Version matching the CLI/service build")
	platform := flags.String("goos", runtime.GOOS, "Target operating system")
	architecture := flags.String("goarch", runtime.GOARCH, "Target Go architecture")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || !filepath.IsAbs(*output) || *version == "" || len(*version) > 128 || strings.IndexFunc(*version, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("v.-+", r))
	}) >= 0 {
		return errors.New("explicit absolute output and a plain matching version are required")
	}
	file := "offgrid-executor"
	if *platform == "windows" {
		file += ".exe"
	}
	pack := execution.Pack{Schema: 1, Protocol: execution.ProtocolVersion, Revision: execution.PackRevision, Version: execution.CanonicalVersion(*version), Platform: *platform, Architecture: *architecture, File: file, SHA256: strings.Repeat("0", 64)}
	if err := pack.Validate(*version); err != nil {
		return err
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	module, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil || !strings.Contains(string(module), "module github.com/takuphilchan/offgrid-llm") {
		return errors.New("run from the OffGrid source root")
	}
	if err = os.MkdirAll(*output, 0755); err != nil {
		return err
	}
	path := filepath.Join(*output, file)
	goFile := "go"
	if runtime.GOOS == "windows" {
		goFile += ".exe"
	}
	command := exec.CommandContext(ctx, filepath.Join(runtime.GOROOT(), "bin", goFile), "build", "-trimpath", "-ldflags", "-X main.Version="+pack.Version, "-o", path, "./cmd/offgrid-executor")
	command.Env = append(os.Environ(), "GOOS="+*platform, "GOARCH="+*architecture, "CGO_ENABLED=0", "GOTOOLCHAIN=local", "GOPROXY=off")
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err = command.Run(); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(data)
	pack.SHA256 = hex.EncodeToString(digest[:])
	encoded, err := json.MarshalIndent(pack, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(*output, ".execution-pack-*.json")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if _, err = temporary.Write(append(encoded, '\n')); err != nil {
		temporary.Close()
		return err
	}
	if err = temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err = temporary.Close(); err != nil {
		return err
	}
	if err = os.Rename(name, filepath.Join(*output, "execution-pack.json")); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, filepath.Join(*output, "execution-pack.json"))
	return nil
}
