package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// ReadChatImage snapshots an explicitly selected regular local file. Paths and
// remote URLs are never forwarded to inference, and reads have a hard bound.
func ReadChatImage(path string) (api.SessionImage, error) {
	var item api.SessionImage
	info, err := os.Stat(path)
	if err != nil {
		return item, fmt.Errorf("cannot read image: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > api.MaxSessionImageBytes {
		return item, fmt.Errorf("select a regular PNG/JPEG file of at most 2 MiB")
	}
	f, err := os.Open(path)
	if err != nil {
		return item, fmt.Errorf("cannot open image: %w", err)
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return item, fmt.Errorf("image is not a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(f, api.MaxSessionImageBytes+1))
	if err != nil {
		return item, fmt.Errorf("cannot read image: %w", err)
	}
	if len(data) > api.MaxSessionImageBytes {
		return item, fmt.Errorf("image exceeds 2 MiB")
	}
	media := http.DetectContentType(data)
	if media != "image/png" && media != "image/jpeg" {
		return item, fmt.Errorf("only PNG and JPEG images are supported")
	}
	item = api.SessionImage{Name: filepath.Base(path), DataURL: "data:" + media + ";base64," + base64.StdEncoding.EncodeToString(data)}
	if err := api.ValidateSessionImages([]api.SessionImage{item}); err != nil {
		return api.SessionImage{}, err
	}
	return item, nil
}

// ChatImageCommand handles only explicit attachment commands. It never submits
// a prompt; a rejected attachment leaves every previously accepted image intact.
func ChatImageCommand(input string, images *[]api.SessionImage) (bool, string, error) {
	command, arg, _ := strings.Cut(strings.TrimSpace(input), " ")
	command = strings.ToLower(strings.TrimPrefix(command, "/"))
	arg = strings.TrimSpace(arg)
	switch command {
	case "image":
		if len(*images) >= api.MaxSessionImages {
			return true, "", fmt.Errorf("at most four images may be attached; use /remove-image first")
		}
		if len(arg) >= 2 && ((arg[0] == '"' && arg[len(arg)-1] == '"') || (arg[0] == '\'' && arg[len(arg)-1] == '\'')) {
			arg = arg[1 : len(arg)-1]
		}
		if arg == "" {
			return true, "", fmt.Errorf("usage: /image PATH (PNG/JPEG, at most 2 MiB)")
		}
		item, err := ReadChatImage(arg)
		if err != nil {
			return true, "", err
		}
		*images = append(*images, item)
		return true, ChatImagesSummary(*images), nil
	case "images":
		if arg != "" {
			return true, "", fmt.Errorf("usage: /images")
		}
		return true, ChatImagesSummary(*images), nil
	case "remove-image":
		if arg == "" || arg == "all" {
			*images = nil
			return true, "Pending images removed.", nil
		}
		index, err := strconv.Atoi(arg)
		if err != nil || index < 1 || index > len(*images) {
			return true, "", fmt.Errorf("use /remove-image N with a number from /images, or /remove-image all")
		}
		*images = append((*images)[:index-1], (*images)[index:]...)
		return true, ChatImagesSummary(*images), nil
	default:
		return false, "", nil
	}
}

func ChatImagesSummary(images []api.SessionImage) string {
	if len(images) == 0 {
		return "No pending images. Use /image PATH to attach PNG/JPEG."
	}
	var text strings.Builder
	fmt.Fprintf(&text, "Pending images (%d/4):", len(images))
	for i, item := range images {
		fmt.Fprintf(&text, "\n  %d. %s", i+1, TerminalSafe(item.Name))
	}
	return text.String()
}

// Match the saved-Chat image budget. This also bounds accumulated legacy CLI
// history before any request is sent, without removing earlier image context.
func ValidateChatImageBudget(messages []ChatMessage, images []api.SessionImage) error {
	total := 0
	for _, item := range images {
		total += len(item.DataURL)
	}
	for _, message := range messages {
		if _, plain := message.Content.(string); plain {
			continue
		}
		encoded, err := json.Marshal(message.Content)
		if err != nil {
			return err
		}
		var parts []api.ChatContentPart
		if err = json.Unmarshal(encoded, &parts); err != nil {
			return err
		}
		for _, part := range parts {
			if part.ImageURL != nil {
				total += len(part.ImageURL.URL)
			}
		}
	}
	if total > api.MaxSessionImageRequestBytes {
		return fmt.Errorf("conversation image limit reached; start a new conversation or /clear explicitly")
	}
	return nil
}
