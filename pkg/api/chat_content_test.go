package api

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func testImageURL(t *testing.T) string {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, 2, 2))
	picture.Set(0, 0, color.White)
	var content bytes.Buffer
	if err := png.Encode(&content, picture); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(content.Bytes())
}

func TestValidateChatMessagesAcceptsBoundedLocalImage(t *testing.T) {
	messages := []ChatMessage{{Role: "user", Content: []ChatContentPart{
		{Type: "text", Text: "Inspect this selected surface."},
		{Type: "image_url", ImageURL: &ChatImageURL{URL: testImageURL(t), Detail: "high"}},
	}}}
	hasImage, err := ValidateChatMessages(messages)
	if err != nil || !hasImage {
		t.Fatalf("valid image rejected: image=%v err=%v", hasImage, err)
	}
	if messages[0].StringContent() != "Inspect this selected surface." {
		t.Fatalf("text projection lost: %q", messages[0].StringContent())
	}
}

func TestValidateChatMessagesRejectsUnsafeOrMalformedImages(t *testing.T) {
	valid := testImageURL(t)
	tests := []ChatMessage{
		{Role: "user", Content: []any{map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/private.png"}}}},
		{Role: "assistant", Content: []any{map[string]any{"type": "image_url", "image_url": map[string]any{"url": valid}}}},
		{Role: "user", Content: []any{map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,not-base64"}}}},
		{Role: "user", Content: []any{map[string]any{"type": "image_url", "image_url": map[string]any{"url": valid}, "unexpected": true}}},
		{Role: "user", Content: []any{map[string]any{"type": "input_image", "image_url": map[string]any{"url": valid}}}},
	}
	for index, message := range tests {
		if _, err := ValidateChatMessages([]ChatMessage{message}); err == nil {
			t.Fatalf("unsafe content %d was accepted", index)
		}
	}
}

func TestValidateChatMessagesRejectsInvalidText(t *testing.T) {
	if _, err := ValidateChatMessages([]ChatMessage{{Role: "user", Content: "unsafe\x00text"}}); err == nil {
		t.Fatal("NUL text was accepted")
	}
}

func TestValidateChatMessagesAllowsNullAssistantToolCallContent(t *testing.T) {
	messages := []ChatMessage{{Role: "assistant", Content: nil, ToolCalls: []ToolCall{{ID: "call", Type: "function", Function: FunctionCall{Name: "tool", Arguments: `{}`}}}}}
	if _, err := ValidateChatMessages(messages); err != nil {
		t.Fatalf("assistant tool call rejected: %v", err)
	}
	if _, err := ValidateChatMessages([]ChatMessage{{Role: "user", Content: nil}}); err == nil {
		t.Fatal("null user content was accepted")
	}
}
