package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/takuphilchan/offgrid-llm/internal/output"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

const workspaceHelp = `Offline workspace maintenance (stop the service first):
  offgrid workspace backup --data-dir DIR --output backup.zip
  offgrid workspace verify backup.zip
  offgrid workspace restore backup.zip --data-dir NEW_DIR --yes
  offgrid workspace rebuild-knowledge --data-dir DIR --model-id ID --model-file MODEL.gguf --runtime LLAMA_SERVER --output backup.zip --yes

Backups include all files in the data directory, including credentials.
Keep the archive private. Models, runtimes, and configuration outside that
directory must be retained separately. Restore never overwrites existing data
and requires the matching application version. No service is stopped for you.
`

func runWorkspaceCommand(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		if output.JSONMode {
			return json.NewEncoder(os.Stdout).Encode(map[string]string{"help": workspaceHelp})
		}
		_, err := fmt.Fprint(os.Stdout, workspaceHelp)
		return err
	}
	command := args[0]
	flags := flag.NewFlagSet("workspace "+command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	dir := flags.String("data-dir", "", "explicit workspace directory")
	out := flags.String("output", "", "new backup archive path")
	yes := flags.Bool("yes", false, "confirm restoring into a new directory")
	modelID := flags.String("model-id", "", "installed embedding model ID")
	modelFile := flags.String("model-file", "", "local embedding GGUF file")
	runtimePath := flags.String("runtime", "", "installed llama-server binary")
	rest := args[1:]
	var archive string
	if command == "verify" || command == "restore" {
		if len(rest) == 0 {
			return usage("workspace %s requires a backup archive", command)
		}
		archive, rest = rest[0], rest[1:]
	}
	if err := flags.Parse(rest); err != nil {
		return usage("%s", err)
	}
	if flags.NArg() != 0 {
		return usage("unexpected workspace arguments")
	}
	var manifest *storage.BackupManifest
	var err error
	if command != "rebuild-knowledge" && (*modelID != "" || *modelFile != "" || *runtimePath != "") {
		return usage("model/runtime flags are only valid for rebuild-knowledge")
	}
	switch command {
	case "rebuild-knowledge":
		if *dir == "" || *modelID == "" || *modelFile == "" || *runtimePath == "" || *out == "" || !*yes {
			return usage("workspace rebuild-knowledge requires --data-dir DIR --model-id ID --model-file FILE --runtime FILE --output BACKUP.zip --yes")
		}
		manifest, err = storage.BackupWorkspace(ctx, *dir, *out, getVersion())
		if err != nil {
			return err
		}
		err = rag.RebuildKnowledgeOffline(ctx, *dir, *modelID, *modelFile, *runtimePath, func(done, total int) {
			fmt.Fprintf(os.Stderr, "Rebuilding knowledge: %d/%d documents staged\n", done, total)
		})
	case "backup":
		if *dir == "" || *out == "" || *yes {
			return usage("workspace backup requires --data-dir DIR --output FILE")
		}
		if !output.JSONMode {
			fmt.Fprintln(os.Stderr, "Backing up the stopped workspace. The archive contains private data and credentials.")
		}
		manifest, err = storage.BackupWorkspace(ctx, *dir, *out, getVersion())
	case "verify":
		if *dir != "" || *out != "" || *yes {
			return usage("workspace verify accepts only an archive path")
		}
		manifest, err = storage.VerifyBackup(ctx, archive)
	case "restore":
		if *dir == "" || !*yes || *out != "" {
			return usage("workspace restore requires ARCHIVE --data-dir NEW_DIR --yes")
		}
		manifest, err = storage.RestoreWorkspace(ctx, archive, *dir, getVersion())
	default:
		return usage("unknown workspace command %q", command)
	}
	if err != nil {
		return err
	}
	if output.JSONMode {
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"success": true, "operation": command, "manifest": manifest})
	}
	fmt.Fprintf(os.Stdout, "OK Workspace %s: %d files, %d bytes, application %s\n", command, len(manifest.Files), manifest.Bytes, terminalSafe(manifest.ApplicationVersion))
	if command == "verify" {
		fmt.Fprintln(os.Stdout, "Archive checksums verified. Restore additionally checks SQLite integrity before activation.")
	}
	if command == "restore" {
		fmt.Fprintln(os.Stdout, "Original workspace unchanged. Start the matching OffGrid version with OFFGRID_DATA_DIR pointing at the restored directory.")
	}
	if command == "rebuild-knowledge" {
		fmt.Fprintln(os.Stdout, "Verified replacement index published. Source documents and backup are preserved. Restart the service to use knowledge.")
	}
	return nil
}
