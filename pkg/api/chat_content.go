package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"unicode/utf8"
)

const (
	maxChatTextBytes  = 1 << 20
	maxImageBytes     = 8 << 20
	maxImageDimension = 8192
	maxImagePixels    = 32 << 20
	maxContentParts   = 16
)

// ErrInvalidMessageContent is intentionally value-free: request errors and
// support logs must not copy prompt text or image data.
var ErrInvalidMessageContent = errors.New("invalid message content")

// ChatContentPart is the supported OpenAI-compatible multimodal message part.
// Image URLs are deliberately limited to bounded local data URLs; allowing the
// inference runtime to fetch model-provided or caller-provided URLs would create
// an ungoverned network path.
type ChatContentPart struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *ChatImageURL `json:"image_url,omitempty"`
}

type ChatImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// ValidateChatMessages validates the bounded text/image shapes forwarded to
// llama.cpp. It returns whether an image is present so the service can require
// an installed, matching projector before loading the model.
func ValidateChatMessages(messages []ChatMessage) (bool, error) {
	hasImage := false
	for _, message := range messages {
		if message.Content == nil && message.Role == "assistant" && len(message.ToolCalls) > 0 {
			continue
		}
		image, err := validateMessageContent(message.Role, message.Content)
		if err != nil {
			return false, err
		}
		hasImage = hasImage || image
	}
	return hasImage, nil
}

func validateMessageContent(role string, content any) (bool, error) {
	if text, ok := content.(string); ok {
		if !validChatText(text) {
			return false, ErrInvalidMessageContent
		}
		return false, nil
	}
	data, err := json.Marshal(content)
	if err != nil || bytes.Equal(data, []byte("null")) {
		return false, ErrInvalidMessageContent
	}
	var raw []json.RawMessage
	if json.Unmarshal(data, &raw) != nil || len(raw) == 0 || len(raw) > maxContentParts {
		return false, ErrInvalidMessageContent
	}
	hasImage := false
	for _, encoded := range raw {
		var part ChatContentPart
		decoder := json.NewDecoder(bytes.NewReader(encoded))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&part) != nil {
			return false, ErrInvalidMessageContent
		}
		switch part.Type {
		case "text":
			if part.ImageURL != nil || !validChatText(part.Text) {
				return false, ErrInvalidMessageContent
			}
		case "image_url":
			if role != "user" || part.Text != "" || part.ImageURL == nil || validateDataImage(*part.ImageURL) != nil {
				return false, ErrInvalidMessageContent
			}
			hasImage = true
		default:
			return false, ErrInvalidMessageContent
		}
	}
	return hasImage, nil
}

func validChatText(value string) bool {
	return len(value) <= maxChatTextBytes && utf8.ValidString(value) && !strings.ContainsRune(value, 0)
}

func validateDataImage(imageURL ChatImageURL) error {
	if imageURL.Detail != "" && imageURL.Detail != "auto" && imageURL.Detail != "low" && imageURL.Detail != "high" {
		return ErrInvalidMessageContent
	}
	mediaType := ""
	encoded := ""
	for _, candidate := range []string{"image/png", "image/jpeg"} {
		prefix := "data:" + candidate + ";base64,"
		if strings.HasPrefix(imageURL.URL, prefix) {
			mediaType, encoded = candidate, strings.TrimPrefix(imageURL.URL, prefix)
			break
		}
	}
	if mediaType == "" || encoded == "" || len(encoded) > base64.StdEncoding.EncodedLen(maxImageBytes) {
		return ErrInvalidMessageContent
	}
	content, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(content) == 0 || len(content) > maxImageBytes {
		return ErrInvalidMessageContent
	}
	configuration, detected, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil || detected != strings.TrimPrefix(mediaType, "image/") || configuration.Width < 1 || configuration.Height < 1 || configuration.Width > maxImageDimension || configuration.Height > maxImageDimension || int64(configuration.Width)*int64(configuration.Height) > maxImagePixels {
		return ErrInvalidMessageContent
	}
	return nil
}
