package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/output"
)

func downloadJobArtifact(ctx context.Context, args []string) error {
	if len(args) != 4 || args[2] != "--output" || args[3] == "" || args[0] == "" || strings.ContainsAny(args[0], "/\\?#") || len(args[1]) != 64 || strings.Trim(args[1], "0123456789abcdef") != "" {
		return usage("agent artifact RUN_ID SHA256 --output NEW_FILE")
	}
	client, err := newCommandClient()
	if err != nil {
		return err
	}
	resp, err := client.Do(ctx, http.MethodGet, "/api/v2/jobs/"+url.PathEscape(args[0])+"/artifact?digest="+args[1], "", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 128<<10+1))
	if err != nil {
		return err
	}
	digest := sha256.Sum256(data)
	if len(data) > 128<<10 || hex.EncodeToString(digest[:]) != args[1] {
		return fmt.Errorf("artifact integrity verification failed; no file written")
	}
	file, err := os.OpenFile(args[3], os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("choose a new writable file; existing files are never overwritten: %w", err)
	}
	_, writeErr := file.Write(data)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("artifact write failed; inspect the incomplete file: %w", writeErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if output.JSONMode {
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"path": args[3], "sha256": args[1], "bytes": len(data), "verified": true})
	}
	fmt.Fprintf(os.Stdout, "Saved: %s\nSHA256: %s\n", terminalSafe(args[3]), args[1])
	return nil
}
