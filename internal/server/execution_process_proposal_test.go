package server

import (
	"errors"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
)

func TestRepeatedProgramRequiresFreshProposalBeforeDispatch(t *testing.T) {
	args := []byte(`{"program":"go","arguments":["go","test","."]}`)
	err := authorizeExecutionInvocation("workspace_process_start", args, agents.ToolExecution{})
	var rejection *agents.ToolAuthorizationError
	if !errors.As(err, &rejection) || rejection.Code != "execution_program_repeated" {
		t.Fatalf("not a pre-dispatch repair: %v", err)
	}
	if string(args) != `{"program":"go","arguments":["go","test","."]}` {
		t.Fatal("arguments silently rewritten")
	}
	if err := authorizeExecutionInvocation("workspace_process_start", args, agents.ToolExecution{Approved: true}); err != nil {
		t.Fatalf("explicit exact approval changed: %v", err)
	}
	if err := authorizeExecutionInvocation("workspace_process_start", []byte(`{"program":"go","arguments":["test","."]}`), agents.ToolExecution{}); err != nil {
		t.Fatal(err)
	}
	for _, args := range []string{`{"program":"echo","arguments":["echo","hello"]}`, `{"program":"go","arguments":["go","test","-exec=other"]}`} {
		if err := authorizeExecutionInvocation("workspace_process_start", []byte(args), agents.ToolExecution{}); !errors.Is(err, capabilities.ErrApprovalRequired) {
			t.Fatalf("custom command changed: %v", err)
		}
	}
}
