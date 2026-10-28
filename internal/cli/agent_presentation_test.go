package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
)

func TestAgentPresentationKeepsAnswerAndInspectionSeparate(t *testing.T) {
	var out, progress bytes.Buffer
	answer := "A story — café 日本語.\n\n```text\n1| unchanged\n```"
	RenderAgentOutcome(&out, &progress, AgentRunSnapshot{RunID: "run-123", Status: "completed", Output: answer}, PresentationStyle{}, 1500*time.Millisecond)
	if !strings.Contains(out.String(), answer) || strings.Contains(out.String(), "run-123") || strings.Count(progress.String(), "run-123") != 1 || strings.Contains(out.String(), "Status:") || strings.ContainsAny(out.String()+progress.String(), "\x1b\r") {
		t.Fatal(out.String(), progress.String())
	}
	for _, status := range []string{"failed", "interrupted", "uncertain", "waiting_for_input", "waiting_for_approval"} {
		out.Reset()
		progress.Reset()
		RenderAgentOutcome(&out, &progress, AgentRunSnapshot{RunID: "run-123", Status: status, Error: "Keep the saved evidence."}, PresentationStyle{}, time.Second)
		if strings.Contains(progress.String(), "Completed") || !strings.Contains(out.String(), status) || !strings.Contains(out.String(), "Keep the saved evidence.") {
			t.Fatal(status, out.String(), progress.String())
		}
	}
}

func TestAgentPresentationDoesNotTrustTerminalControls(t *testing.T) {
	var out bytes.Buffer
	AgentHeader(&out, PresentationStyle{}, "model\x1b[2J\r\u202e", "")
	if strings.ContainsAny(out.String(), "\x1b\r\u202e") || !strings.Contains(out.String(), "Ctrl+C") {
		t.Fatal(out.String())
	}
	if AgentActionLabel("workspace_write") == "workspace_write" || strings.ContainsAny(AgentActionLabel("unknown\x1b[2J"), "\x1b") {
		t.Fatal("unsafe or unreadable action label")
	}
}

func TestWrittenPathRequiresVerifiedReceiptAndPrintsOnce(t *testing.T) {
	valid := `{"host":"host","session":"session","operation":"file_write","receipt":{"state":"complete","result":{"host":"host","path":"/tmp/café story.txt","revision":"` + strings.Repeat("a", 64) + `","bytes":12,"verified":true}}}`
	for _, kind := range []string{"valid", "unverified", "uncertain", "foreign", "bad-revision"} {
		raw := valid
		switch kind {
		case "unverified":
			raw = strings.Replace(raw, `"verified":true`, `"verified":false`, 1)
		case "uncertain":
			raw = strings.Replace(raw, `"state":"complete"`, `"state":"uncertain"`, 1)
		case "foreign":
			raw = strings.Replace(raw, `"host":"host"`, `"host":"foreign"`, 1)
		case "bad-revision":
			raw = strings.Replace(raw, strings.Repeat("a", 64), "bad", 1)
		}
		var out bytes.Buffer
		seen := map[string]bool{}
		step := agents.Step{Type: "action", ToolName: "workspace_write", ToolResult: raw}
		RenderExecutionObservations(&out, []agents.Step{step, step}, seen)
		if kind == "valid" {
			if strings.Count(out.String(), "/tmp/café story.txt\n") != 1 {
				t.Fatal(out.String())
			}
		} else if out.Len() != 0 {
			t.Fatal(kind, out.String())
		}
	}
}
