package cli

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func hasRunFiles(args []string) bool {
	for _, arg := range args {
		if arg == "--file" {
			return true
		}
	}
	return false
}

// File runs start with a service-owned conversation. They never start another
// inference process, read the service's filesystem, or downgrade to legacy text
// completion when attachment support is unavailable.
func runWithFiles(ctx context.Context, args []string, env Environment) error {
	if len(args) < 3 || strings.HasPrefix(args[0], "-") {
		return Usage("run MODEL --file PATH [--prompt QUESTION] [--save NAME | --load NAME]")
	}
	model, name, prompt := args[0], "", ""
	load, oneShot, knowledge := false, false, false
	var paths []string
	var images []api.SessionImage
	for i := 1; i < len(args); i++ {
		option := args[i]
		if option == "--rag" {
			knowledge = true
			continue
		}
		if option != "--file" && option != "--prompt" && option != "--save" && option != "--load" && option != "--image" {
			return Usage("unknown file chat option %s", option)
		}
		if i+1 >= len(args) {
			return Usage("%s requires a value", option)
		}
		i++
		value := args[i]
		switch option {
		case "--file":
			paths = append(paths, value)
		case "--prompt":
			if oneShot {
				return Usage("--prompt may be supplied once")
			}
			prompt = value
			oneShot = true
		case "--save", "--load":
			if name != "" || value == "" {
				return Usage("choose one --save or --load conversation")
			}
			name = value
			load = option == "--load"
		case "--image":
			if _, _, err := ChatImageCommand("/image "+value, &images); err != nil {
				return err
			}
		}
	}
	if len(paths) == 0 || len(paths) > 8 {
		return Usage("attach one to eight files")
	}
	if env.Options.JSON && !oneShot {
		return Usage("JSON file chat requires --prompt QUESTION")
	}
	// Validate all paths before creating durable state. Uploads snapshot again so
	// a changed source is not represented by the earlier preflight bytes.
	for _, path := range paths {
		if _, _, err := ReadChatFile(path); err != nil {
			return err
		}
	}
	client, err := serviceClient(env)
	if err != nil {
		return err
	}
	if err = client.RequireCapability(ctx, "session-files-v1"); err != nil {
		return err
	}
	if len(images) > 0 {
		if err = client.RequireCapability(ctx, "session-images-v1"); err != nil {
			return err
		}
	}
	if name == "" {
		name = "files-" + rand.Text()
	}
	var saved *sessions.Session
	if load {
		saved, err = LoadServiceSession(ctx, client, name)
	} else {
		saved = &sessions.Session{}
		err = client.JSON(ctx, http.MethodPost, "/v1/sessions", map[string]string{"name": name, "model_id": model}, saved)
		if err == nil && saved.Name != name {
			err = fmt.Errorf("service returned a different conversation identity")
		}
	}
	if err != nil {
		return err
	}
	// Report recovery identity before upload. JSON stdout stays a single result.
	fmt.Fprintf(env.Stderr, "Saved conversation: %s\nResume: offgrid session chat %s\n", TerminalSafe(name), TerminalSafe(name))
	draft := ConversationFileDraft{}
	for _, path := range paths {
		if _, _, err = draft.Command(ctx, client, name, "/attach "+path); err != nil {
			return err
		}
	}
	if oneShot {
		request := serviceclient.ConversationRequest{RequestID: rand.Text(), Model: model, Content: prompt, Images: images, Knowledge: knowledge}
		for _, file := range draft.Files {
			request.Attachments = append(request.Attachments, sessions.AttachmentReference{ID: file.ID, Revision: file.Revision})
		}
		return sendPlainConversation(ctx, client, name, request, env.Stdout, env.Options.JSON)
	}
	if env.Interactive.ShouldUseTUI != nil && env.Interactive.RunTUI != nil && env.Interactive.ShouldUseTUI() {
		state := &InteractiveTUIState{ModelName: model, ResolvedModel: model, Images: images, UseKnowledgeBase: knowledge, Messages: SavedChatMessages(saved), CurrentSession: saved, Files: &ServiceFileConversation{Context: ctx, Client: client, Name: name, Draft: draft}}
		return env.Interactive.RunTUI(ctx, state)
	}
	return conversationChatLoop(ctx, client, saved, draft, images, model, knowledge, env.Stdin, env.Stdout)
}

// ServiceFileConversation is shared by both terminal presentations. Only the
// authenticated service assembles source context and persists completed turns.
type ServiceFileConversation struct {
	pending *serviceclient.ConversationRequest
	Saved   *sessions.Session
	Context context.Context
	Client  *serviceclient.Client
	Name    string
	Draft   ConversationFileDraft
}

func (c *ServiceFileConversation) Stream(model, input string, images []api.SessionImage, knowledge bool, output func(string) error) (*sessions.Session, error) {
	if len(c.Draft.FailedUploads) > 0 {
		return nil, Usage("an upload was not confirmed; use /attachments for recovery instructions; nothing was sent")
	}
	request := serviceclient.ConversationRequest{RequestID: rand.Text(), Model: model, Content: input, Images: images, Knowledge: knowledge}
	for _, file := range c.Draft.Files {
		if file.Status != "ready" || file.Revision == "" {
			return nil, Usage("files need preparation; use /attachments and /retry-attachment N")
		}
		request.Attachments = append(request.Attachments, sessions.AttachmentReference{ID: file.ID, Revision: file.Revision})
	}
	request.Images = append([]api.SessionImage(nil), images...)
	if c.pending != nil {
		previous := *c.pending
		previous.RequestID = request.RequestID
		if reflect.DeepEqual(previous, request) {
			request.RequestID = c.pending.RequestID
		}
	}
	c.pending = &request
	response, err := c.Client.Conversation(c.Context, c.Name, request, output)
	if err != nil {
		return nil, err
	}
	c.Draft.Files = nil
	c.pending = nil
	c.Saved = response.Session
	return response.Session, nil
}

// DocumentCoverageNotice describes service-recorded evidence, not model prose.
func DocumentCoverageNotice(saved *sessions.Session) string {
	if saved == nil || len(saved.Messages) == 0 {
		return ""
	}
	last := saved.Messages[len(saved.Messages)-1]
	if last.Role != "assistant" {
		return ""
	}
	for _, selection := range last.AttachmentSelections {
		if selection.Mode == "excerpts" {
			return "Source coverage: selected document passages, not a complete review. Open the source to inspect the full text."
		}
	}
	return ""
}
