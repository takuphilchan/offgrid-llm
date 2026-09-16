package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"unicode"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/output"
)

type agentRunSnapshot struct {
	RunID           string           `json:"run_id"`
	Status          string           `json:"status"`
	Output          string           `json:"output"`
	Error           string           `json:"error,omitempty"`
	Resumable       bool             `json:"resumable"`
	PendingApproval *agents.Approval `json:"pending_approval"`
	UncertainCallID string           `json:"uncertain_call_id,omitempty"`
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
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key := strings.TrimSpace(os.Getenv("OFFGRID_API_KEY")); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("agent service returned HTTP %d: %s", resp.StatusCode, terminalSafe(strings.TrimSpace(string(data))))
	}
	return resp, nil
}

func agentControl(base string, args []string) (*agentRunSnapshot, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("usage: offgrid agent status|approve|deny|cancel|resume|reconcile RUN_ID [APPROVAL_ID or CALL_ID RESULT]")
	}
	action, id := args[0], args[1]
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, "/\\?#") {
		return nil, fmt.Errorf("invalid run ID")
	}
	endpoint := strings.TrimRight(base, "/") + "/v1/agents/tasks/" + url.PathEscape(id)
	method := http.MethodPost
	data := map[string]any{"async": true}
	switch action {
	case "status", "resume", "cancel":
		if len(args) != 2 {
			return nil, fmt.Errorf("usage: offgrid agent %s RUN_ID", action)
		}
	case "approve", "deny":
		if len(args) != 3 || args[2] == "" {
			return nil, fmt.Errorf("usage: offgrid agent %s RUN_ID APPROVAL_ID", action)
		}
		data["approval_id"] = args[2]
	case "reconcile":
		if len(args) < 4 || args[2] == "" || strings.TrimSpace(strings.Join(args[3:], " ")) == "" {
			return nil, fmt.Errorf("usage: offgrid agent reconcile RUN_ID CALL_ID \"verified outcome\"")
		}
		data["call_id"], data["result"] = args[2], strings.Join(args[3:], " ")
	default:
		return nil, fmt.Errorf("unknown run action %q", action)
	}
	var body io.Reader
	if action == "status" {
		method = http.MethodGet
	} else {
		endpoint += "/" + action
		encoded, _ := json.Marshal(data)
		body = bytes.NewReader(encoded)
	}
	resp, err := agentRequest(httpClient, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var snapshot agentRunSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func handleAgentControl(args []string) {
	cfg := config.LoadConfig()
	snapshot, err := agentControl(fmt.Sprintf("http://127.0.0.1:%d", cfg.ServerPort), args)
	if err != nil {
		printError(err.Error())
		return
	}
	if output.JSONMode {
		output.PrintJSON(snapshot)
		return
	}
	fmt.Printf("Run: %s\n", terminalSafe(snapshot.RunID))
	renderAgentSnapshot(os.Stdout, *snapshot)
}

func renderAgentSnapshot(w io.Writer, snapshot agentRunSnapshot) {
	fmt.Fprintf(w, "Status: %s\n", terminalSafe(snapshot.Status))
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
