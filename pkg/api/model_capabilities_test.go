package api

import "testing"

func TestChatModelCapabilityBoundary(t *testing.T) {
	for _, typ := range []string{"", "llm", "vlm", "chat"} {
		if !(Model{Type: typ}).SupportsChat() {
			t.Fatalf("legacy %q rejected", typ)
		}
	}
	for _, typ := range []string{"embedding", "asr", "tts", "whisper", "unknown"} {
		if (Model{Type: typ, Capabilities: []string{"chat"}}).SupportsChat() {
			t.Fatalf("non-chat %q accepted", typ)
		}
	}
	if (Model{Type: "llm", Capabilities: []string{"transcription"}}).SupportsChat() {
		t.Fatal("explicit non-chat capabilities ignored")
	}
}
