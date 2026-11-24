package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/output"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

const workspaceHelp = `Offline workspace maintenance (stop the service first):
  offgrid workspace backup --data-dir DIR --output backup.zip
  offgrid workspace backup --data-dir DIR --output backup.zip.age --recipient AGE_PUBLIC_KEY
  offgrid workspace backup --data-dir DIR --output backup.zip.age --passphrase
  offgrid workspace verify backup.zip
  offgrid workspace verify backup.zip.age --identity KEY_FILE
  offgrid workspace restore backup.zip --data-dir NEW_DIR --yes
  offgrid workspace rebuild-knowledge --data-dir DIR --model-id ID --model-file MODEL.gguf --runtime LLAMA_SERVER --output backup.zip --yes

Backup/rebuild: repeat --recipient or --recipients-file for native age public keys.
Verify/restore: repeat --identity for native age private-key files.
All operations: --passphrase prompts without echo; --passphrase-stdin reads one
bounded secret from redirected stdin. Backup prompts require confirmation.
Key files: at most 1 MiB each and 32 keys total. Passphrases: 1–4096 UTF-8 bytes.
Recipient/identity and passphrase modes cannot be combined. No secrets in arguments.
Passphrase derivation uses scrypt logN18 (about 256 MiB); larger costs are refused.
Use recipient keys for unattended backups. Without encryption flags, backups
remain legacy unencrypted ZIP files, regardless of their filename extension.

Backups include all files in the data directory, including credentials.
Keep the archive private. Models, runtimes, and configuration outside that
directory must be retained separately. Restore never overwrites existing data
and requires the matching application version. No service is stopped for you.
`

func runWorkspaceCommand(ctx context.Context, args []string) error {
	return runWorkspaceCommandWithIO(ctx, args, workspaceStreams{
		in: os.Stdin, out: os.Stdout, err: os.Stderr, json: output.JSONMode, version: getVersion(),
		terminalSecret: func(ctx context.Context, out io.Writer, confirm bool) ([]byte, error) {
			return readWorkspaceTerminalSecret(ctx, os.Stdin, out, confirm)
		},
		rebuild: rag.RebuildKnowledgeOfflineWithBackup,
	})
}

type workspaceStreams struct {
	in             io.Reader
	out, err       io.Writer
	json           bool
	version        string
	terminalSecret func(context.Context, io.Writer, bool) ([]byte, error)
	rebuild        func(context.Context, string, string, string, string, string, string, storage.BackupWriteOptions, func(int, int)) (*storage.BackupManifest, error)
}

func runWorkspaceCommandWithIO(ctx context.Context, args []string, streams workspaceStreams) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		if streams.json {
			return json.NewEncoder(streams.out).Encode(map[string]string{"help": workspaceHelp})
		}
		_, err := fmt.Fprint(streams.out, workspaceHelp)
		return err
	}
	command := args[0]
	if command != "backup" && command != "verify" && command != "restore" && command != "rebuild-knowledge" {
		return usage("unknown workspace command; see workspace help")
	}
	writing := command == "backup" || command == "rebuild-knowledge"
	flags := flag.NewFlagSet("workspace "+command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var dir, out, modelID, modelFile, runtimePath string
	var yes bool
	if command != "verify" {
		flags.StringVar(&dir, "data-dir", "", "explicit workspace directory")
	}
	if writing {
		flags.StringVar(&out, "output", "", "new backup archive path")
	}
	if command == "restore" || command == "rebuild-knowledge" {
		flags.BoolVar(&yes, "yes", false, "confirm maintenance into an explicit destination")
	}
	if command == "rebuild-knowledge" {
		flags.StringVar(&modelID, "model-id", "", "installed embedding model ID")
		flags.StringVar(&modelFile, "model-file", "", "local embedding GGUF file")
		flags.StringVar(&runtimePath, "runtime", "", "installed llama-server binary")
	}
	var keys workspaceKeyFlags
	keys.register(flags, writing)
	rest := args[1:]
	var archive string
	if command == "verify" || command == "restore" {
		if len(rest) == 0 || strings.HasPrefix(rest[0], "-") {
			return usage("workspace %s requires a backup archive", command)
		}
		archive, rest = rest[0], rest[1:]
	}
	if err := flags.Parse(rest); err != nil {
		// flag errors can echo a secret mistakenly supplied as a boolean value.
		return usage("invalid workspace option; see workspace help (never pass secrets as arguments)")
	}
	if flags.NArg() != 0 {
		return usage("unexpected workspace arguments")
	}
	var disabledSecretMode bool
	flags.Visit(func(f *flag.Flag) {
		if (f.Name == "passphrase" || f.Name == "passphrase-stdin") && f.Value.String() != "true" {
			disabledSecretMode = true
		}
	})
	if disabledSecretMode {
		return usage("passphrase flags must be enabled; omit encryption flags only for legacy plaintext backup")
	}
	if writing && (dir == "" || out == "") {
		return usage("workspace backup/rebuild requires --data-dir DIR --output FILE")
	}
	if command == "restore" && (dir == "" || !yes) {
		return usage("workspace restore requires ARCHIVE --data-dir NEW_DIR --yes")
	}
	if command == "rebuild-knowledge" && (modelID == "" || modelFile == "" || runtimePath == "" || !yes) {
		return usage("workspace rebuild-knowledge requires --data-dir DIR --model-id ID --model-file FILE --runtime FILE --output BACKUP --yes")
	}
	writeOptions, readOptions, err := keys.options(ctx, streams, writing)
	if err != nil {
		return err
	}
	var manifest *storage.BackupManifest
	switch command {
	case "rebuild-knowledge":
		manifest, err = streams.rebuild(ctx, dir, modelID, modelFile, runtimePath, out, streams.version, writeOptions, func(done, total int) {
			fmt.Fprintf(streams.err, "Rebuilding knowledge: %d/%d documents staged\n", done, total)
		})
		if err != nil && manifest != nil {
			return fmt.Errorf("knowledge rebuild failed; verified %s recovery backup retained at %s: %w", manifest.Encryption, terminalSafe(out), err)
		}
	case "backup":
		if !streams.json {
			fmt.Fprintln(streams.err, "Backing up the stopped workspace. The archive contains private data and credentials.")
		}
		manifest, err = storage.BackupWorkspaceWithOptions(ctx, dir, out, streams.version, writeOptions)
	case "verify":
		manifest, err = storage.VerifyBackupWithOptions(ctx, archive, readOptions)
	case "restore":
		manifest, err = storage.RestoreWorkspaceWithOptions(ctx, archive, dir, streams.version, readOptions)
	}
	if err != nil {
		return err
	}
	if streams.json {
		return json.NewEncoder(streams.out).Encode(map[string]any{"success": true, "operation": command, "manifest": manifest, "encryption": manifest.Encryption, "authenticated": manifest.Encryption == storage.BackupEncryptionAge})
	}
	fmt.Fprintf(streams.out, "OK Workspace %s: %d files, %d bytes, application %s, encryption %s\n", command, len(manifest.Files), manifest.Bytes, terminalSafe(manifest.ApplicationVersion), manifest.Encryption)
	if command == "verify" {
		fmt.Fprintln(streams.out, "Archive checksums verified. Restore additionally checks SQLite integrity before activation.")
	}
	if command == "restore" {
		fmt.Fprintln(streams.out, "Original workspace unchanged. Start the matching OffGrid version with OFFGRID_DATA_DIR pointing at the restored directory.")
	}
	if command == "rebuild-knowledge" {
		fmt.Fprintln(streams.out, "Verified replacement index published. Source documents and backup are preserved. Restart the service to use knowledge.")
	}
	return nil
}
