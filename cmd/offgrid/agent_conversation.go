package main

import (
	"context"
	"io"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

type linkedTurnResponse = cli.LinkedTurnResponse

func conversationChat(ctx context.Context, client *serviceclient.Client, args []string, stdin io.Reader, stdout io.Writer, jsonMode bool) error {
	return cli.RunConversationChat(ctx, client, args, stdin, stdout, jsonMode, renderAgentStreamTo)
}

func conversationCommand(ctx context.Context, client *serviceclient.Client, args []string, stdout io.Writer, jsonMode bool) error {
	return cli.RunConversationService(ctx, client, args, stdout, jsonMode, renderAgentStreamTo)
}
