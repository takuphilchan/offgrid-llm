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
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

const taskArtifactToolName = "task_save_artifact"

type taskArtifactRequest struct {
	Name    string `json:"name"`
	Format  string `json:"format"`
	Content string `json:"content"`
}
type taskArtifactEvidence struct {
	Name     string `json:"name"`
	Format   string `json:"format"`
	SHA256   string `json:"sha256"`
	Bytes    int    `json:"bytes"`
	Rows     int    `json:"rows,omitempty"`
	Columns  int    `json:"columns,omitempty"`
	Verified bool   `json:"verified"`
	Check    string `json:"check"`
}

func taskArtifactTool() api.Tool {
	return api.Tool{Type: "function", Function: api.FunctionDef{Name: taskArtifactToolName, Description: "Save a private downloadable workspace artifact (UTF-8 text, Markdown, JSON or CSV; at most 128 KiB). The service independently rereads, hashes and parses it before reporting verified storage. This does not save into a host application or establish factual accuracy. No executable files or spreadsheet formulas. Use a basename with matching extension. Call separately from computer actions.", Parameters: map[string]interface{}{"type": "object", "additionalProperties": false, "required": []string{"name", "format", "content"}, "properties": map[string]interface{}{"name": map[string]string{"type": "string"}, "format": map[string]interface{}{"type": "string", "enum": []string{"text", "markdown", "json", "csv"}}, "content": map[string]string{"type": "string"}}}}}
}
func taskArtifactDescriptor() capabilities.Descriptor {
	return capabilities.Descriptor{Name: taskArtifactToolName, Namespace: "workspace", Source: "workspace", Kind: capabilities.Write, Risk: capabilities.RiskLow, Description: taskArtifactTool().Function.Description}
}
func parseTaskArtifact(raw json.RawMessage) (taskArtifactRequest, taskArtifactEvidence, error) {
	var req taskArtifactRequest
	var evidence taskArtifactEvidence
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if len(raw) > 1<<20 || decoder.Decode(&req) != nil || decoder.Decode(new(any)) != io.EOF {
		return req, evidence, fmt.Errorf("invalid artifact request")
	}
	suffix := map[string]string{"text": ".txt", "markdown": ".md", "csv": ".csv", "json": ".json"}[req.Format]
	if suffix == "" || path.Base(req.Name) != req.Name || len(req.Name) > 120 || len(req.Name) <= len(suffix) || !strings.HasSuffix(req.Name, suffix) || strings.ContainsAny(req.Name, "\\:\x00\r\n") || strings.IndexFunc(req.Name, unicode.IsControl) >= 0 || len(req.Content) == 0 || len(req.Content) > 128<<10 || !utf8.ValidString(req.Content) || strings.ContainsRune(req.Content, 0) {
		return req, evidence, fmt.Errorf("use a plain filename and 1–131072 bytes of UTF-8 content in a supported format")
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
