package server

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

// Drops the service result after genuine native integration and ledger
// settlement. Recovery must use that evidence without another tool dispatch.
type integrationLostResultFixture struct {
	*browserRunTools
	calls atomic.Int32
}

func (f *integrationLostResultFixture) ExecuteWithPolicy(ctx context.Context, name string, args json.RawMessage, call agents.ToolExecution) (string, error) {
	result, err := f.browserRunTools.ExecuteWithPolicy(ctx, name, args, call)
	if name == execution.IntegrateTool && err == nil {
		f.calls.Add(1)
		return "", errors.New("fixture lost integration result")
	}
	return result, err
}
