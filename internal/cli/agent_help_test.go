package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestAgentHelpProgressivelyDisclosesFullReference(t *testing.T) {
	var out bytes.Buffer
	env := Environment{Stdout: &out}
	legacyAgentHelp(env)
	if len(strings.Split(out.String(), "\n")) > 40 || !strings.Contains(out.String(), "agent help all") || strings.Contains(out.String(), "APPROVAL_ID") {
		t.Fatal(out.String())
	}
	for _, line := range strings.Split(out.String(), "\n") {
		if len([]rune(line)) > 80 {
			t.Fatal("help exceeds 80 columns", line)
		}
	}
	out.Reset()
	legacyAgentHelpAll(env)
	for _, want := range []string{"Approval and recovery", "Evidence and artifacts", "agent reconcile", "agent recover-execution", "--request-id", "workspace trust revoke", "schedules help"} {
		if !strings.Contains(out.String(), want) {
			t.Fatal("missing", want)
		}
	}
	if strings.Count(out.String(), "agent status RUN_ID") != 1 {
		t.Fatal("duplicate command")
	}
	out.Reset()
	if err := agentWorkspaceCommand(context.Background(), []string{"help"}, env); err != nil || !strings.Contains(out.String(), "--backend isolated_linux") {
		t.Fatal("offline workspace help failed", err, out.String())
	}
}
