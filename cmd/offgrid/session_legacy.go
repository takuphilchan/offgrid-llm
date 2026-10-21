package main

import (
	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

func exportSessionMarkdown(session *sessions.Session) string {
	return cli.ExportSessionMarkdown(session)
}
func exportSessionText(session *sessions.Session) string { return cli.ExportSessionText(session) }
