package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

type AgentRunSnapshot struct {
	Steps               []agents.Step        `json:"steps,omitempty"`
	CanRecoverExecution bool                 `json:"can_recover_execution,omitempty"`
	Execution           *execution.Reference `json:"execution,omitempty"`
	ExecutionWorkspace  *execution.Resource  `json:"execution_workspace,omitempty"`
	Profile             json.RawMessage      `json:"profile,omitempty"`
	Origin              json.RawMessage      `json:"origin,omitempty"`
	PendingInstructions []agents.Instruction `json:"pending_instructions,omitempty"`
	CanSteer            bool                 `json:"can_steer,omitempty"`
	Outcome             string               `json:"outcome,omitempty"`
	Verification        json.RawMessage      `json:"verification,omitempty"`
	Evidence            json.RawMessage      `json:"-"`
	Artifacts           json.RawMessage      `json:"artifacts,omitempty"`
	Plan                []agents.PlanItem    `json:"plan,omitempty"`
	Context             *agents.ContextState `json:"context,omitempty"`
	Children            []agents.ChildLink   `json:"children,omitempty"`
	ParentID            string               `json:"parent_id,omitempty"`
	RunID               string               `json:"run_id"`
	Status              string               `json:"status"`
	Output              string               `json:"output"`
	Error               string               `json:"error,omitempty"`
	Resumable           bool                 `json:"resumable"`
	PendingApproval     *agents.Approval     `json:"pending_approval"`
	PendingInput        *agents.InputRequest `json:"pending_input,omitempty"`
	UncertainCallID     string               `json:"uncertain_call_id,omitempty"`
	Progress            *agents.RunProgress  `json:"progress,omitempty"`
}

func AgentControlService(ctx context.Context, client *serviceclient.Client, args []string) (*AgentRunSnapshot, error) {
	if len(args) < 2 {
		return nil, Usage("usage: offgrid agent status|approve|deny|cancel|resume|reconcile|recover-execution RUN_ID [APPROVAL_ID or CALL_ID RESULT]")
	}
	action, id := args[0], args[1]
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, "/\\?#") {
		return nil, Usage("invalid run ID")
	}
	endpoint := "/api/v2/jobs/" + url.PathEscape(id)
	method := http.MethodPost
	data := map[string]any{"async": true}
	switch action {
	case "status", "resume", "cancel", "pause", "takeover", "reconnect", "export", "recover-execution":
		if len(args) != 2 {
			return nil, Usage("usage: offgrid agent %s RUN_ID", action)
		}
	case "approve", "deny":
		if len(args) != 3 || args[2] == "" {
			return nil, Usage("usage: offgrid agent %s RUN_ID APPROVAL_ID", action)
		}
		data["approval_id"] = args[2]
	case "reconcile":
		if len(args) < 4 || args[2] == "" || strings.TrimSpace(strings.Join(args[3:], " ")) == "" {
			return nil, Usage("usage: offgrid agent reconcile RUN_ID CALL_ID \"verified outcome\"")
		}
		data["call_id"], data["result"] = args[2], strings.Join(args[3:], " ")
	case "steer":
		if len(args) < 4 {
			return nil, Usage("agent steer RUN_ID REQUEST_ID <instruction>")
		}
		data["request_id"], data["instruction"] = args[2], strings.Join(args[3:], " ")
	default:
		return nil, Usage("unknown run action %q", action)
	}
	if action == "recover-execution" {
		data = map[string]any{}
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
	var snapshot AgentRunSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil, err
	}
	if action == "export" {
		snapshot.Evidence = raw
	}
	if snapshot.RunID != id || !ValidAgentStatus(snapshot.Status) {
		return nil, &serviceclient.Error{Code: "invalid_response", Message: "OffGrid returned an invalid run snapshot. Check the service version."}
	}
	return &snapshot, nil
}

func ValidAgentStatus(status string) bool {
	switch agents.TaskStatus(status) {
	case agents.TaskPending, agents.TaskRunning, agents.TaskWaiting, agents.TaskInput, agents.TaskChildren, agents.TaskInterrupted, agents.TaskUncertain, agents.TaskCompleted, agents.TaskFailed, agents.TaskCancelled:
		return true
	}
	return false
}

func RenderAgentSnapshot(w io.Writer, snapshot AgentRunSnapshot) {
	fmt.Fprintf(w, "Status: %s\n", TerminalSafe(snapshot.Status))
	if snapshot.CanRecoverExecution {
		fmt.Fprintf(w, "Inspect the saved host receipt without replaying work:\n  offgrid agent recover-execution %s\n", TerminalSafe(snapshot.RunID))
	}
	for _, child := range snapshot.Children {
		fmt.Fprintf(w, "Subtask: %s â€” %s\n  offgrid agent status %s\n", TerminalSafe(child.Spec.Key), TerminalSafe(child.Spec.Goal), TerminalSafe(child.ID))
	}
	if input := snapshot.PendingInput; snapshot.Status == string(agents.TaskInput) && input != nil {
		if input.Kind == "workspace" {
			fmt.Fprintf(w, "Folder access needed: %s. Review the local folder selection to continue this saved job.\n", TerminalSafe(input.Mode))
		} else {
			fmt.Fprintf(w, "Local access needed: %s\nOpen this saved task in Agents in the matching OffGrid Desktop workspace to review access. No input has been dispatched.\n  offgrid agent cancel %s\n", TerminalSafe(input.Target), TerminalSafe(snapshot.RunID))
		}
	}
	if snapshot.Output != "" {
		fmt.Fprintln(w, TerminalSafe(snapshot.Output))
	}
	if snapshot.Error != "" {
		fmt.Fprintln(w, TerminalSafe(snapshot.Error))
	}
	if approval := snapshot.PendingApproval; approval != nil {
		fmt.Fprintf(w, "Approval required: %s\nArguments: %s\nExpires: %s\n", TerminalSafe(approval.Tool), TerminalSafe(string(approval.Arguments)), approval.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"))
		fmt.Fprintf(w, "  offgrid agent approve %s %s\n  offgrid agent deny %s %s\n", TerminalSafe(snapshot.RunID), TerminalSafe(approval.ID), TerminalSafe(snapshot.RunID), TerminalSafe(approval.ID))
		fmt.Fprintln(w, "If expired, refresh this same call with: offgrid agent resume "+TerminalSafe(snapshot.RunID))
	}
	if snapshot.Status == "running" || snapshot.Status == "pending" {
		fmt.Fprintf(w, "  offgrid agent status %s\n  offgrid agent cancel %s\n", TerminalSafe(snapshot.RunID), TerminalSafe(snapshot.RunID))
	}
	if snapshot.Status == "interrupted" && snapshot.Resumable {
		fmt.Fprintf(w, "  offgrid agent resume %s\n", TerminalSafe(snapshot.RunID))
	}
	if snapshot.UncertainCallID != "" {
		fmt.Fprintln(w, "Inspect the tool's target first. Do not repeat the action blindly. A report is human-observed evidence, not independent verification.")
		fmt.Fprintf(w, "  offgrid agent reconcile %s %s \"what you observed\"\n", TerminalSafe(snapshot.RunID), TerminalSafe(snapshot.UncertainCallID))
	}
	if len(snapshot.PendingInstructions) > 0 {
		fmt.Fprintln(w, "Instruction queued for the next safe boundary; dispatched actions are not cancelled.")
	}
}
