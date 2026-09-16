package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/output"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

const knowledgeUsage = "offgrid kb status|list|enable MODEL|disable|add PATH|remove ID|search QUERY|clear --yes"

func runKnowledgeCommand(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		if output.JSONMode {
			return json.NewEncoder(os.Stdout).Encode(map[string]string{"usage": knowledgeUsage})
		}
		fmt.Println(knowledgeUsage)
		fmt.Println("Uses OFFGRID_SERVER_URL and OFFGRID_API_KEY. --json emits one result; Ctrl+C cancels the request.")
		return nil
	}
	client, err := newCommandClient()
	if err != nil {
		return err
	}
	result, err := knowledgeCommand(ctx, client, args, os.Stdin, os.Stderr)
	if err != nil {
		return err
	}
	if output.JSONMode {
		return json.NewEncoder(os.Stdout).Encode(result)
	}
	// Stable keys also make the plain presentation useful in narrow terminals.
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, terminalSafe(string(data)))
	return nil
}

func knowledgeCommand(ctx context.Context, client *serviceclient.Client, args []string, input io.Reader, progress io.Writer) (any, error) {
	if len(args) == 0 {
		return nil, usage("Usage: %s", knowledgeUsage)
	}
	var result map[string]any
	get := func(path string) (any, error) {
		err := client.JSON(ctx, http.MethodGet, path, nil, &result)
		if err == nil {
			err = validateKnowledgeResponse(args[0], result)
		}
		return result, err
	}
	mutate := func(method, path string, body any) (any, error) {
		err := client.JSON(ctx, method, path, body, &result)
		if err == nil {
			err = validateKnowledgeResponse(args[0], result)
		}
		return result, err
	}
	switch args[0] {
	case "status", "list", "ls", "disable":
		if len(args) != 1 {
			return nil, usage("Usage: offgrid kb %s", args[0])
		}
		switch args[0] {
		case "status":
			return get("/v1/rag/status")
		case "list", "ls":
			return get("/v1/documents")
		default:
			return mutate(http.MethodPost, "/v1/rag/disable", nil)
		}
	case "enable", "remove", "rm", "delete", "del", "add":
		if len(args) != 2 || strings.TrimSpace(args[1]) == "" {
			return nil, usage("Usage: offgrid kb %s VALUE", args[0])
		}
		switch args[0] {
		case "enable":
			return mutate(http.MethodPost, "/v1/rag/enable", map[string]string{"embedding_model": args[1]})
		case "add":
			return ingestKnowledgePath(ctx, client, args[1], progress)
		default:
			return mutate(http.MethodDelete, "/v1/documents/delete?id="+url.QueryEscape(args[1]), nil)
		}
	case "search":
		if len(args) < 2 || strings.TrimSpace(strings.Join(args[1:], " ")) == "" {
			return nil, usage("Usage: offgrid kb search QUERY")
		}
		return mutate(http.MethodPost, "/v1/documents/search", map[string]any{"query": strings.Join(args[1:], " "), "top_k": 5})
	case "clear":
		if len(args) > 2 || (len(args) == 2 && args[1] != "--yes") {
			return nil, usage("Usage: offgrid kb clear --yes")
		}
		if len(args) == 1 {
			if output.JSONMode || !isInteractiveInput(input) {
				return nil, usage("Clearing documents requires explicit confirmation: offgrid kb clear --yes")
			}
			fmt.Fprint(progress, "Remove all accessible knowledge documents? Type yes: ")
			answer, err := bufio.NewReader(input).ReadString('\n')
			if err != nil || strings.TrimSpace(strings.ToLower(answer)) != "yes" {
				return nil, context.Canceled
			}
		}
		var listing map[string]any
		if err := client.JSON(ctx, http.MethodGet, "/v1/documents", nil, &listing); err != nil {
			return nil, err
		}
		if err := validateKnowledgeResponse("list", listing); err != nil {
			return nil, err
		}
		deleted := 0
		for _, doc := range listing["documents"].([]any) {
			id := doc.(map[string]any)["id"].(string)
			if _, err := mutate(http.MethodDelete, "/v1/documents/delete?id="+url.QueryEscape(id), nil); err != nil {
				return nil, fmt.Errorf("Stopped after deleting %d documents: %w", deleted, err)
			}
			deleted++
		}
		return map[string]any{"deleted": deleted}, nil
	default:
		return nil, usage("Unknown knowledge command %q. Usage: %s", args[0], knowledgeUsage)
	}
}

func isInteractiveInput(input io.Reader) bool {
	file, ok := input.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func ingestKnowledgePath(ctx context.Context, client *serviceclient.Client, path string, progress io.Writer) (any, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("Cannot inspect input: %w", err)
	}
	if !info.IsDir() {
		return ingestKnowledgeFile(ctx, client, path)
	}
	added, skipped := 0, 0
	err = filepath.WalkDir(path, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !isKBSupportedFile(current) {
			skipped++
			return nil
		}
		if _, err := ingestKnowledgeFile(ctx, client, current); err != nil {
			return fmt.Errorf("Import stopped after %d files (%s): %w", added, terminalSafe(filepath.Base(current)), err)
		}
		added++
		fmt.Fprintf(progress, "Imported %d: %s\n", added, terminalSafe(filepath.Base(current)))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"added": added, "skipped": skipped}, nil
}

func ingestKnowledgeFile(ctx context.Context, client *serviceclient.Client, path string) (any, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || !isKBSupportedFile(path) {
		return nil, usage("Input must be a regular supported document, not a symlink or device")
	}
	const limit = 50 << 20
	if info.Size() > limit {
		return nil, usage("Document exceeds the 50 MiB upload limit")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	done := make(chan struct{})
	defer func() { _ = reader.Close(); <-done }()
	go func() {
		defer close(done)
		part, err := multipartWriter.CreateFormFile("file", filepath.Base(path))
		if err == nil {
			var size int64
			size, err = io.Copy(part, io.LimitReader(file, limit+1))
			if err == nil && size > limit {
				err = fmt.Errorf("document grew past the upload limit")
			}
		}
		if err == nil {
			err = multipartWriter.Close()
		}
		_ = writer.CloseWithError(err)
	}()
	resp, err := client.Do(ctx, http.MethodPost, "/v1/documents/ingest", multipartWriter.FormDataContentType(), reader)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result map[string]any
	if err := serviceclient.DecodeContext(ctx, resp.Body, &result); err != nil {
		return nil, err
	}
	if err := validateKnowledgeResponse("add", result); err != nil {
		return nil, err
	}
	return result, nil
}

func validateKnowledgeResponse(action string, result map[string]any) error {
	invalid := &serviceclient.Error{Code: "invalid_response", Message: "OffGrid returned an incomplete knowledge response. Inspect service state before retrying a mutation."}
	switch action {
	case "status":
		if _, ok := result["enabled"].(bool); !ok {
			return invalid
		}
	case "list", "ls":
		documents, ok := result["documents"].([]any)
		if !ok {
			return invalid
		}
		for _, item := range documents {
			doc, ok := item.(map[string]any)
			if !ok {
				return invalid
			}
			if id, ok := doc["id"].(string); !ok || id == "" {
				return invalid
			}
		}
	case "search":
		value, exists := result["results"]
		if !exists {
			return invalid
		}
		if _, ok := value.([]any); value != nil && !ok {
			return invalid
		}
	default:
		if success, ok := result["success"].(bool); !ok || !success {
			return invalid
		}
	}
	return nil
}

func isKBSupportedFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".txt", ".md", ".markdown", ".json", ".csv", ".xml", ".html", ".htm", ".pdf", ".docx", ".xlsx", ".pptx", ".rtf", ".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".h", ".hpp", ".rs", ".sh", ".yaml", ".yml", ".toml":
		return true
	}
	return false
}
