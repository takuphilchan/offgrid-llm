package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"unicode"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/output"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

type agentRunSnapshot struct {
	Evidence        json.RawMessage      `json:"-"`
	Artifacts       json.RawMessage      `json:"artifacts,omitempty"`
	Plan            []agents.PlanItem    `json:"plan,omitempty"`
	Context         *agents.ContextState `json:"context,omitempty"`
	Children        []agents.ChildLink   `json:"children,omitempty"`
	ParentID        string               `json:"parent_id,omitempty"`
	RunID           string               `json:"run_id"`
	Status          string               `json:"status"`
	Output          string               `json:"output"`
	Error           string               `json:"error,omitempty"`
	Resumable       bool                 `json:"resumable"`
	PendingApproval *agents.Approval     `json:"pending_approval"`
	PendingInput    *agents.InputRequest `json:"pending_input,omitempty"`
	UncertainCallID string               `json:"uncertain_call_id,omitempty"`
	Progress        *agents.RunProgress  `json:"progress,omitempty"`
}

// Terminal output contains model/tool data, not terminal control instructions.
func terminalSafe(value string) string {
	return strings.Map(func(r rune) rune {
		if (unicode.IsControl(r) && r != '\n' && r != '\t') || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, value)
}

func agentRequest(client *http.Client, method, endpoint string, body io.Reader) (*http.Response, error) {
	return agentRequestContext(context.Background(), client, method, endpoint, body)
}

func agentRequestContext(ctx context.Context, transport *http.Client, method, endpoint string, body io.Reader) (*http.Response, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.User != nil || u.Fragment != "" {
		return nil, usage("Invalid agent service URL")
	}
	client, err := serviceclient.New(u.Scheme+"://"+u.Host, os.Getenv("OFFGRID_API_KEY"), transport)
	if err != nil {
		return nil, err
	}
	return client.Do(ctx, method, u.RequestURI(), "application/json", body)
}

func agentControl(base string, args []string) (*agentRunSnapshot, error) {
	return agentControlContext(context.Background(), base, args)
}

func agentControlContext(ctx context.Context, base string, args []string) (*agentRunSnapshot, error) {
	if len(args) < 2 {
		return nil, usage("usage: offgrid agent status|approve|deny|cancel|resume|reconcile RUN_ID [APPROVAL_ID or CALL_ID RESULT]")
	}
	action, id := args[0], args[1]
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, "/\\?#") {
		return nil, usage("invalid run ID")
	}
	client, err := serviceclient.New(base, os.Getenv("OFFGRID_API_KEY"), httpClient)
	if err != nil {
		return nil, err
	}
	endpoint := "/api/v2/jobs/" + url.PathEscape(id)
	method := http.MethodPost
	data := map[string]any{"async": true}
	switch action {
	case "status", "resume", "cancel", "pause", "takeover", "reconnect", "export":
		if len(args) != 2 {
			return nil, usage("usage: offgrid agent %s RUN_ID", action)
		}
	case "approve", "deny":
		if len(args) != 3 || args[2] == "" {
			return nil, usage("usage: offgrid agent %s RUN_ID APPROVAL_ID", action)
		}
		data["approval_id"] = args[2]
	case "reconcile":
		if len(args) < 4 || args[2] == "" || strings.TrimSpace(strings.Join(args[3:], " ")) == "" {
			return nil, usage("usage: offgrid agent reconcile RUN_ID CALL_ID \"verified outcome\"")
		}
		data["call_id"], data["result"] = args[2], strings.Join(args[3:], " ")
	case "steer":
		if len(args) < 4 {
			return nil, usage("agent steer RUN_ID REQUEST_ID <instruction>")
		}
		data["request_id"], data["instruction"] = args[2], strings.Join(args[3:], " ")
	default:
		return nil, usage("unknown run action %q", action)
	}
	var body io.Reader
	if action == "status" || action == "export" {
		method = http.MethodGet
		if action == "export" {
			endpoint += "/export"
		}
	} else {
		endpoint += "/" + action
		encoded, _ := json.Marshal(data)
		body = bytes.NewReader(encoded)
	}
	resp, err := client.Do(ctx, method, endpoint, "application/json", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var raw json.RawMessage
	if err := serviceclient.DecodeContext(ctx, resp.Body, &raw); err != nil {
		return nil, err
	}
	var snapshot agentRunSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil, err
	}
	if action == "export" {
		snapshot.Evidence = raw
	}
	if snapshot.RunID != id || !validAgentStatus(snapshot.Status) {
		return nil, &serviceclient.Error{Code: "invalid_response", Message: "OffGrid returned an invalid run snapshot. Check the service version."}
	}
	return &snapshot, nil
}

func validAgentStatus(status string) bool {
	switch agents.TaskStatus(status) {
	case agents.TaskPending, agents.TaskRunning, agents.TaskWaiting, agents.TaskInput, agents.TaskChildren, agents.TaskInterrupted, agents.TaskUncertain, agents.TaskCompleted, agents.TaskFailed, agents.TaskCancelled:
		return true
	}
	return false
}

func isAgentControl(action string) bool {
	switch action {
	case "status", "approve", "deny", "cancel", "resume", "reconcile", "pause", "takeover", "steer", "reconnect", "export", "artifact":
		return true
	}
	return false
}

func runAgentControl(ctx context.Context, args []string) error {
	if len(args) > 0 && args[0] == "artifact" {
		return downloadJobArtifact(ctx, args[1:])
	}
	snapshot, err := agentControlContext(ctx, commandServerURL(), args)
	if err != nil {
		return err
	}
	if len(snapshot.Evidence) > 0 {
		return json.NewEncoder(os.Stdout).Encode(snapshot.Evidence)
	}
	if output.JSONMode {
		return json.NewEncoder(os.Stdout).Encode(snapshot)
	}
	fmt.Printf("Run: %s\n", terminalSafe(snapshot.RunID))
	renderAgentSnapshot(os.Stdout, *snapshot)
	return nil
}

func handleAgentControl(args []string) {
	executeCommand(func(ctx context.Context) error { return runAgentControl(ctx, args) })
}

func renderAgentSnapshot(w io.Writer, snapshot agentRunSnapshot) {
	fmt.Fprintf(w, "Status: %s\n", terminalSafe(snapshot.Status))
	for _, child := range snapshot.Children {
		fmt.Fprintf(w, "Subtask: %s — %s\n  offgrid agent status %s\n", terminalSafe(child.Spec.Key), terminalSafe(child.Spec.Goal), terminalSafe(child.ID))
	}
	if input := snapshot.PendingInput; snapshot.Status == string(agents.TaskInput) && input != nil {
		fmt.Fprintf(w, "Local access needed: %s\nOpen this saved task in Agents in the matching OffGrid Desktop workspace to review access. No input has been dispatched.\n  offgrid agent cancel %s\n", terminalSafe(input.Target), terminalSafe(snapshot.RunID))
	}
	if snapshot.Output != "" {
		fmt.Fprintln(w, terminalSafe(snapshot.Output))
	}
	if snapshot.Error != "" {
		fmt.Fprintln(w, terminalSafe(snapshot.Error))
	}
	if approval := snapshot.PendingApproval; approval != nil {
		fmt.Fprintf(w, "Approval required: %s\nArguments: %s\nExpires: %s\n", terminalSafe(approval.Tool), terminalSafe(string(approval.Arguments)), approval.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"))
		fmt.Fprintf(w, "  offgrid agent approve %s %s\n  offgrid agent deny %s %s\n", terminalSafe(snapshot.RunID), terminalSafe(approval.ID), terminalSafe(snapshot.RunID), terminalSafe(approval.ID))
		fmt.Fprintln(w, "If expired, refresh this same call with: offgrid agent resume "+terminalSafe(snapshot.RunID))
	}
	if snapshot.Status == "running" || snapshot.Status == "pending" {
		fmt.Fprintf(w, "  offgrid agent status %s\n  offgrid agent cancel %s\n", terminalSafe(snapshot.RunID), terminalSafe(snapshot.RunID))
	}
	if snapshot.Status == "interrupted" && snapshot.Resumable {
		fmt.Fprintf(w, "  offgrid agent resume %s\n", terminalSafe(snapshot.RunID))
	}
	if snapshot.UncertainCallID != "" {
		fmt.Fprintln(w, "Inspect the tool's target first. Do not repeat the action blindly.")
		fmt.Fprintf(w, "  offgrid agent reconcile %s %s \"verified outcome\"\n", terminalSafe(snapshot.RunID), terminalSafe(snapshot.UncertainCallID))
	}
}
