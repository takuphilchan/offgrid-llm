package cli

import (
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"strings"
)

func ExportSessionMarkdown(session *sessions.Session) string {
	var sb strings.Builder

	sb.WriteString("# Chat Session: " + session.Name + "\n\n")
	sb.WriteString("**Model:** " + session.ModelID + "  \n")
	sb.WriteString("**Created:** " + session.CreatedAt.Format("2006-01-02 15:04:05") + "  \n")
	sb.WriteString("**Updated:** " + session.UpdatedAt.Format("2006-01-02 15:04:05") + "  \n")
	sb.WriteString("\n---\n\n")

	for _, msg := range session.Messages {
		if msg.Role == "user" {
			sb.WriteString("### User\n\n")
		} else {
			sb.WriteString("### Assistant\n\n")
		}
		sb.WriteString(msg.Content + "\n\n")
	}

	return sb.String()
}

func ExportSessionText(session *sessions.Session) string {
	var sb strings.Builder

	sb.WriteString("Chat Session: " + session.Name + "\n")
	sb.WriteString("Model: " + session.ModelID + "\n")
	sb.WriteString("Created: " + session.CreatedAt.Format("2006-01-02 15:04:05") + "\n")
	sb.WriteString(strings.Repeat("=", 60) + "\n\n")

	for _, msg := range session.Messages {
		if msg.Role == "user" {
			sb.WriteString("[USER]\n")
		} else {
			sb.WriteString("[ASSISTANT]\n")
		}
		sb.WriteString(msg.Content + "\n\n")
	}

	return sb.String()
}
