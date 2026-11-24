package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Draft editing never mutates the detached input retained for an explicit retry.
// A receipt clears sources only when this exact draft version was submitted.
type canonicalSourceDraft struct {
	files     ConversationFileDraft
	images    []api.SessionImage
	knowledge bool
	version   uint64
}

func conversationInputHasSources(input api.ConversationInboxInput) bool {
	return len(input.Attachments)+len(input.Images)+len(input.KnowledgeDocuments)+len(input.KnowledgeSelections) > 0 || input.UseKnowledge
}

func (d *canonicalSourceDraft) hasCaptionSource() bool {
	return len(d.files.Files)+len(d.files.FailedUploads)+len(d.images) > 0
}

func (d *canonicalSourceDraft) command(ctx context.Context, client *serviceclient.Client, session, text string) (bool, string, error) {
	command, arg, _ := strings.Cut(text, " ")
	if IsConversationFileCommand(text) {
		switch command {
		case "/attach", "/use-attachment", "/retry-attachment", "/attachment-text-only":
			for _, capability := range []string{"conversation-source-inputs-v1", "session-files-v1"} {
				if err := client.RequireCapability(ctx, capability); err != nil {
					return true, "", err
				}
			}
		}
		before, _ := json.Marshal(d.files)
		handled, message, err := d.files.Command(ctx, client, session, text)
		after, _ := json.Marshal(d.files)
		if string(before) != string(after) {
			d.version++
		}
		return handled, message, err
	}
	switch strings.ToLower(command) {
	case "/image", "/images", "/remove-image":
		if strings.EqualFold(command, "/image") {
			if err := client.RequireCapability(ctx, "conversation-source-inputs-v1"); err != nil {
				return true, "", err
			}
		}
		before := slices.Clone(d.images)
		handled, message, err := ChatImageCommand(text, &d.images)
		if !slices.Equal(before, d.images) {
			d.version++
		}
		return handled, message, err
	case "/rag":
		if strings.TrimSpace(arg) != "" {
			return true, "", Usage("/rag takes no arguments")
		}
		if !d.knowledge {
			if err := client.RequireCapability(ctx, "conversation-source-inputs-v1"); err != nil {
				return true, "", err
			}
		}
		d.knowledge = !d.knowledge
		d.version++
		return true, fmt.Sprintf("Knowledge retrieval: %t", d.knowledge), nil
	}
	return false, "", nil
}

func (d *canonicalSourceDraft) snapshot(session string, input *api.ConversationInboxInput) error {
	if len(d.files.FailedUploads) > 0 {
		return Usage("an upload was not confirmed; use /attachments for recovery instructions; nothing was sent")
	}
	if !d.hasCaptionSource() && !d.knowledge {
		return nil
	}
	if input.Intent == "answer" || input.Intent == "correction" {
		return Usage("answers and corrections do not accept source changes; remove the draft sources or use /message MESSAGE or /new MESSAGE explicitly")
	}
	if err := api.ValidateSessionImages(d.images); err != nil {
		return err
	}
	refs := make([]api.ConversationAttachment, 0, len(d.files.Files))
	seen := map[string]bool{}
	for _, file := range d.files.Files {
		if file.Status != "ready" || file.Revision == "" || seen[file.ID] {
			return Usage("files need preparation or contain duplicates; use /attachments, /retry-attachment N and /remove-attachment N")
		}
		seen[file.ID] = true
		refs = append(refs, api.ConversationAttachment{ID: file.ID, Revision: file.Revision})
	}
	input.SourceSession, input.Attachments = session, refs
	input.Images = slices.Clone(d.images)
	input.UseKnowledge = d.knowledge
	return nil
}

func (d *canonicalSourceDraft) accepted(version uint64) {
	if version == d.version {
		d.files.Files = nil
		d.images = nil
		d.version++
	}
}
