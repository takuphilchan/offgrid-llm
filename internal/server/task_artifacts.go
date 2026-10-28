package server

// Generated task artifacts are private workspace outputs, not permission to
// write host paths. Deterministic read-back and parsing verify persistence and
// format; they do NOT establish factual accuracy of model-authored content.
import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/artifacts"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

const taskArtifactToolName = "task_save_artifact"

type taskArtifactRequest struct {
	Name    string `json:"name"`
	Format  string `json:"format"`
	Content string `json:"content"`
}
type taskArtifactEvidence struct {
	StorageLocation string `json:"storage_location,omitempty"`
	HostFileCreated bool   `json:"host_file_created"`
	Name            string `json:"name"`
	Format          string `json:"format"`
	SHA256          string `json:"sha256"`
	Bytes           int    `json:"bytes"`
	Rows            int    `json:"rows,omitempty"`
	Columns         int    `json:"columns,omitempty"`
	Verified        bool   `json:"verified"`
	Check           string `json:"check"`
}

func taskArtifactTool() api.Tool {
	variants := []any{}
	for _, format := range []string{"text", "markdown", "json", "csv"} {
		suffix := artifactSuffix(format)
		// Keep patterns printable: literal control ranges survive JSON decoding
		// and corrupt llama.cpp's generated grammar. The authoritative parser
		// below still rejects all control characters and enforces UTF-8 byte limits.
		variants = append(variants, map[string]any{
			"type": "object", "additionalProperties": false, "required": []string{"name", "format", "content"},
			"properties": map[string]any{
				"name":    map[string]any{"type": "string", "maxLength": 120, "pattern": `^[^/\\:]+\` + suffix + "$", "description": "Plain filename, at most 120 UTF-8 bytes, ending in " + suffix + "; no directories or control characters."},
				"format":  map[string]any{"type": "string", "enum": []string{format}},
				"content": map[string]any{"type": "string", "minLength": 1, "maxLength": 131072, "description": "Nonempty UTF-8 content, at most 131072 bytes (not characters), without NUL. JSON and CSV must parse; CSV formulas are forbidden."},
			},
		})
	}
	return api.Tool{Type: "function", Function: api.FunctionDef{Name: taskArtifactToolName, Description: "Create a private download in OffGrid task storage, NEVER a file in the user's current directory or computer. For local files request workspace access and use workspace_write instead. Text downloads use .txt, markdown .md, json .json, csv .csv. Storage read-back verifies stored bytes and format ONLY, not the requested destination, factual accuracy or completed research. First perform requested work and gather actual evidence. Report this result as an OffGrid download, never as a host file. Call separately from computer actions.", Parameters: map[string]any{"oneOf": variants}}}
}

func artifactSuffix(format string) string {
	return map[string]string{"text": ".txt", "markdown": ".md", "csv": ".csv", "json": ".json"}[format]
}
func taskArtifactDescriptor() capabilities.Descriptor {
	return capabilities.Descriptor{Name: taskArtifactToolName, Namespace: "workspace", Source: "workspace", Location: "service", Kind: capabilities.Write, Risk: capabilities.RiskLow, Description: taskArtifactTool().Function.Description}
}
func parseTaskArtifact(raw json.RawMessage) (taskArtifactRequest, taskArtifactEvidence, error) {
	var req taskArtifactRequest
	var evidence taskArtifactEvidence
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if len(raw) > 1<<20 || !utf8.Valid(raw) || jsonwire.Validate(raw) != nil || decoder.Decode(&req) != nil || decoder.Decode(new(any)) != io.EOF {
		return req, evidence, fmt.Errorf("invalid artifact request")
	}
	suffix := artifactSuffix(req.Format)
	if suffix == "" {
		return req, evidence, fmt.Errorf("artifact format must be text, markdown, json or csv")
	}
	if req.Name == "" || path.Base(req.Name) != req.Name || len(req.Name) > 120 || strings.ContainsAny(req.Name, "\\:\x00\r\n") || strings.IndexFunc(req.Name, unicode.IsControl) >= 0 {
		return req, evidence, fmt.Errorf("artifact name must be a plain filename of at most 120 UTF-8 bytes, without paths or control characters")
	}
	if len(req.Name) <= len(suffix) || !strings.HasSuffix(req.Name, suffix) {
		return req, evidence, fmt.Errorf("artifact name must have a nonempty basename followed by %s for the selected format", suffix)
	}
	if len(req.Content) == 0 {
		return req, evidence, fmt.Errorf("artifact content is empty; supply the actual requested content")
	}
	if len(req.Content) > 128<<10 {
		return req, evidence, fmt.Errorf("artifact content exceeds 131072 UTF-8 bytes; shorten it without omitting required work")
	}
	if !utf8.ValidString(req.Content) || strings.ContainsRune(req.Content, 0) {
		return req, evidence, fmt.Errorf("artifact content must be valid UTF-8 without NUL characters")
	}
	if req.Format == "json" && !json.Valid([]byte(req.Content)) {
		return req, evidence, fmt.Errorf("artifact content is not valid JSON")
	}
	if req.Format == "csv" {
		records, err := csv.NewReader(strings.NewReader(req.Content)).ReadAll()
		if err != nil || len(records) == 0 || len(records) > 5000 {
			return req, evidence, fmt.Errorf("CSV must have consistent columns and at most 5000 rows")
		}
		for _, row := range records {
			for _, cell := range row {
				v := strings.TrimSpace(cell)
				if v != "" && strings.ContainsAny(v[:1], "=+@-") {
					return req, evidence, fmt.Errorf("CSV formula-like cells are not permitted; use JSON or text to preserve them")
				}
			}
		}
		evidence.Rows, evidence.Columns = len(records), len(records[0])
	}
	digest := sha256.Sum256([]byte(req.Content))
	evidence.Name, evidence.Format, evidence.SHA256, evidence.Bytes = req.Name, req.Format, hex.EncodeToString(digest[:]), len(req.Content)
	return req, evidence, nil
}
func (s *Server) saveTaskArtifact(ctx context.Context, args json.RawMessage) (string, error) {
	req, evidence, err := parseTaskArtifact(args)
	if err != nil {
		return "", err
	}
	if s.artifactStore == nil {
		return "", fmt.Errorf("artifact storage unavailable")
	}
	// Digests deduplicate bytes, not ownership. Ownership and chosen filename are
	// the task's persisted evidence; never inherit another writer's shared metadata.
	if _, err = s.artifactStore.Put(ctx, strings.NewReader(req.Content), artifacts.Metadata{MediaType: "application/octet-stream"}); err != nil {
		return "", err
	}
	reader, _, err := s.artifactStore.Open(evidence.SHA256)
	if err != nil {
		return "", err
	}
	data, err := io.ReadAll(io.LimitReader(reader, 128<<10+1))
	closeErr := reader.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	if !bytes.Equal(data, []byte(req.Content)) {
		return "", fmt.Errorf("saved artifact read-back mismatch")
	}
	evidence.Verified, evidence.Check = true, "stored_bytes_sha256_and_format"
	evidence.StorageLocation = "offgrid_task_artifact"
	result, _ := json.Marshal(evidence)
	return string(result), nil
}
func taskArtifacts(task *agents.Task) []taskArtifactEvidence {
	found := []taskArtifactEvidence{}
	for _, step := range task.Steps {
		if step.Type != "action" || step.ToolName != taskArtifactToolName {
			continue
		}
		var evidence taskArtifactEvidence
		if json.Unmarshal([]byte(step.ToolResult), &evidence) == nil && evidence.Verified && evidence.Check == "stored_bytes_sha256_and_format" && len(evidence.SHA256) == 64 {
			found = append(found, evidence)
		}
	}
	return found
}

func (s *Server) ownsTaskArtifact(actor, digest string) bool {
	if s.agentManager == nil || s.agentManager.StorageError() != nil {
		return false
	}
	for _, task := range s.agentManager.ListTasks() {
		if task.Actor != actor {
			continue
		}
		for _, evidence := range taskArtifacts(task) {
			if evidence.SHA256 == digest {
				return true
			}
		}
		if task.Status == agents.TaskCompleted && task.Result != "" {
			sum := sha256.Sum256([]byte(task.Result))
			if hex.EncodeToString(sum[:]) == digest {
				return true
			}
		}
	}
	return false
}
func (s *Server) downloadTaskArtifact(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != "GET" {
		writeJobError(w, 405, "method_not_allowed", "Use GET to download an artifact.", false)
		return
	}
	if s.agentManager == nil || s.artifactStore == nil {
		writeJobReadError(w, agents.ErrRunStorage)
		return
	}
	task, ok := s.agentManager.GetTask(id)
	if !ok || task.Actor != s.agentActor(r) {
		writeJobReadError(w, agents.ErrTaskNotFound)
		return
	}
	digest := r.URL.Query().Get("digest")
	var matched *taskArtifactEvidence
	for _, e := range taskArtifacts(task) {
		if e.SHA256 == digest {
			copy := e
			matched = &copy
			break
		}
	}
	if matched == nil {
		writeJobReadError(w, agents.ErrTaskNotFound)
		return
	}
	reader, _, err := s.artifactStore.Open(digest)
	if err != nil {
		writeJobReadError(w, agents.ErrRunStorage)
		return
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, 128<<10+1))
	hash := sha256.Sum256(data)
	if err != nil || len(data) != matched.Bytes || hex.EncodeToString(hash[:]) != digest {
		writeJobError(w, 503, "artifact_integrity_failed", "The saved artifact failed its integrity check. Do not use it.", false)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": matched.Name}))
	w.Write(data)
}
