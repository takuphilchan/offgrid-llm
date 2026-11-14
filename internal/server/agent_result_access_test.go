package server

import (
	"context"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
)

func TestResultRetrievalRechecksLiveSourceAccess(t *testing.T) {
	s := profileFixture(t)
	tools := &browserRunTools{server: s}
	caller := &agents.Task{Actor: "local-admin"}
	result := &agents.Task{Actor: caller.Actor}
	if err := tools.AuthorizeTaskResult(context.Background(), caller, result); err != nil {
		t.Fatal(err)
	}
	result.Config.KnowledgeDependencies = []string{"removed-source"}
	if err := tools.AuthorizeTaskResult(context.Background(), caller, result); err == nil {
		t.Fatal("revoked dependency disclosed")
	}
	result.Config.KnowledgeDependencies = nil
	result.Actor = "another-actor"
	if err := tools.AuthorizeTaskResult(context.Background(), caller, result); err == nil {
		t.Fatal("cross-actor result disclosed")
	}
}
