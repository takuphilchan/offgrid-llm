package main

import (
	"context"
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

type chatFileCommandMsg struct {
	files   *cli.ServiceFileConversation
	draft   cli.ConversationFileDraft
	message string
	err     error
}

func isFileChatCommand(command string) bool {
	return cli.IsConversationFileCommand("/" + command)
}
func fileChatCommand(files *cli.ServiceFileConversation, command string) tea.Cmd {
	draft := cli.ConversationFileDraft{Files: append([]sessions.Attachment(nil), files.Draft.Files...), FailedUploads: append([]string(nil), files.Draft.FailedUploads...)}
	return func() tea.Msg {
		_, message, err := draft.Command(files.Context, files.Client, files.Name, "/"+command)
		return chatFileCommandMsg{files: files, draft: draft, message: message, err: err}
	}
}

func (rt *ChatRuntime) fileCommand(command string) tea.Cmd {
	files, start, model := rt.Files, rt.StartFiles, rt.ModelName
	messages := append([]ChatMessage(nil), rt.Messages...)
	ctx := rt.Context
	if ctx == nil {
		ctx = context.Background()
	}
	rt.fileWork.Add(1)
	return func() tea.Msg {
		defer rt.fileWork.Done()
		if files == nil {
			needed, message, err := cli.FileCommandNeedsSession(command)
			if err != nil || !needed {
				return chatFileCommandMsg{message: message, err: err}
			}
			if start == nil {
				return chatFileCommandMsg{err: fmt.Errorf("file attachment is unavailable in this client")}
			}
			files, err = start(ctx, model, messages)
			if err != nil {
				return chatFileCommandMsg{err: err}
			}
		}
		return fileChatCommand(files, command)()
	}
}

func (rt *ChatRuntime) streamFiles(input string, ch chan<- chatStreamMsg) error {
	saved, err := rt.Files.Stream(rt.ModelName, input, rt.Images, rt.UseKnowledgeBase, func(text string) error {
		select {
		case ch <- chatStreamMsg{token: cli.TerminalSafe(text)}:
			return nil
		case <-rt.Files.Context.Done():
			return rt.Files.Context.Err()
		}
	})
	if err != nil {
		return err
	}
	rt.CurrentSession = saved
	rt.Messages = cli.SavedChatMessages(saved)
	rt.Images = nil
	if notice := cli.DocumentCoverageNotice(saved); notice != "" {
		select {
		case ch <- chatStreamMsg{token: "\n\n" + notice}:
		case <-rt.Files.Context.Done():
			return rt.Files.Context.Err()
		}
	}
	return nil
}
