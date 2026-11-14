package cli

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func runSessionChanges(ctx context.Context, client *serviceclient.Client, args []string, out io.Writer, jsonMode bool) error {
	if len(args) < 2 || len(args) > 6 {
		return Usage("session changes NAME [CHANGE_ID [files|validation|integration|recovery|content|preserved [SHA256 [ARTIFACT_ID]]]]")
	}
	saved, err := LoadServiceSession(ctx, client, args[1])
	if err != nil {
		return err
	}
	if saved.CanonicalID == "" {
		return Usage("change inspection requires a canonical conversation")
	}
	return inspectConversationChanges(ctx, client, saved.CanonicalID, args[2:], out, jsonMode)
}

func inspectConversationChanges(ctx context.Context, client *serviceclient.Client, conversation string, args []string, out io.Writer, jsonMode bool) error {
	if len(args) > 4 {
		return Usage("/changes [CHANGE_ID [files|validation|integration|recovery|content|preserved [SHA256 [ARTIFACT_ID]]]]")
	}
	current, err := client.CanonicalConversation(ctx, conversation)
	if err != nil {
		return err
	}
	if current.FocusedWorkID == "" {
		return Usage("this conversation has no focused work")
	}
	print := func(value any) error {
		if jsonMode {
			return json.NewEncoder(out).Encode(value)
		}
		return printWorkerJSON(out, value)
	}
	if len(args) == 0 {
		items := []api.WorkChange{}
		for cursor := ""; ; {
			page, err := client.WorkChanges(ctx, current.FocusedWorkID, cursor, 64)
			if err != nil {
				return err
			}
			for _, item := range page.Items {
				if item.WorkID != current.FocusedWorkID {
					return fmt.Errorf("change belongs to different work")
				}
			}
			items = append(items, page.Items...)
			if !page.HasMore {
				return print(items)
			}
			if page.NextCursor <= cursor {
				return fmt.Errorf("invalid change cursor")
			}
			cursor = page.NextCursor
		}
	}
	view, hash, limit := "files", "", 32
	if len(args) > 1 {
		view = args[1]
	}
	if len(args) > 2 {
		hash = args[2]
	}
	artifact := ""
	if len(args) > 3 {
		artifact = args[3]
	}
	contentView := view == "content" || view == "preserved"
	if view != "files" && view != "validation" && view != "integration" && view != "recovery" && !contentView || contentView && hash == "" || !contentView && hash != "" || (view == "preserved") != (artifact != "") {
		return Usage("use files, validation, integration, recovery, content SHA256, or preserved SHA256 ARTIFACT_ID")
	}
	if contentView {
		limit = 65536
	}
	digest, total := sha256.New(), int64(-1)
	for offset := int64(0); ; {
		page, err := client.WorkChange(ctx, current.FocusedWorkID, args[0], view, hash, offset, limit, artifact)
		if err != nil {
			return err
		}
		if page.Change.ID != args[0] || page.Change.WorkID != current.FocusedWorkID || page.View != view {
			return fmt.Errorf("change evidence does not match request")
		}
		if contentView {
			content := page.Content
			if content == nil || content.SHA256 != hash || content.Offset != offset || content.Bytes < 0 || total >= 0 && content.Bytes != total {
				return fmt.Errorf("change content does not match request")
			}
			total = content.Bytes
			end := offset + int64(len(content.Data))
			if end > total || end < offset || page.NextOffset != nil && (*page.NextOffset != end || end == offset) {
				return fmt.Errorf("invalid change content cursor")
			}
			digest.Write(content.Data)
			if page.NextOffset == nil && (end != total || fmt.Sprintf("%x", digest.Sum(nil)) != hash) {
				return fmt.Errorf("change content digest mismatch")
			}
		}
		if err = print(page); err != nil {
			return err
		}
		if page.NextOffset == nil {
			return nil
		}
		if *page.NextOffset <= offset {
			return fmt.Errorf("invalid change evidence cursor")
		}
		offset = *page.NextOffset
	}
}
