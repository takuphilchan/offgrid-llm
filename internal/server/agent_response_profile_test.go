package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/config"
)

func TestNativeResponseProfilePinsAllocationWithoutChangingService(t *testing.T) {
	s := taskFirstServer(t)
	s.config = &config.Config{MaxContextSize: 65536, ChatContextSize: 8192, AdaptiveContext: false}
	for _, profile := range []string{"", "interactive", "extended", "invalid"} {
		w := httptest.NewRecorder()
		request := jobSubmitRequest{Model: "model", Prompt: "Explain rainbows", ResponseProfile: profile}
		cfg, ok := s.resolveJobConfig(w, httptest.NewRequest("POST", "/api/v2/jobs", nil), &request)
		if profile == "invalid" {
			if ok || w.Code != 400 {
				t.Fatal("invalid selection accepted", w.Code)
			}
			continue
		}
		want := 65536
		if profile == "interactive" {
			want = 8192
		}
		if !ok || cfg.ContextWindow != want || cfg.ConversationProfile != profile {
			t.Fatal(profile, ok, cfg.ContextWindow, cfg.ConversationProfile, w.Body.String())
		}
		if s.config.MaxContextSize != 65536 || s.modelContextWindow("model") != 65536 {
			t.Fatal("changed external/default allocation")
		}
	}
}

func TestNativeResponseProfileIsPartOfSavedJobIdentity(t *testing.T) {
	s := taskFirstServer(t)
	s.config = &config.Config{MaxContextSize: 65536, ChatContextSize: 8192}
	var first string
	for _, profile := range []string{"interactive", "interactive", "extended"} {
		w := httptest.NewRecorder()
		body := `{"model":"model","prompt":"Hello","request_id":"response-profile-fixture","response_profile":"` + profile + `"}`
		s.handleJobSubmit(w, httptest.NewRequest("POST", "/api/v2/jobs", strings.NewReader(body)))
		if profile == "extended" {
			if w.Code != 409 {
				t.Fatal("changed allocation reused request identity", w.Code, w.Body.String())
			}
			continue
		}
		var response struct {
			RunID string `json:"run_id"`
		}
		if w.Code != 202 || json.Unmarshal(w.Body.Bytes(), &response) != nil || response.RunID == "" {
			t.Fatal(w.Code, w.Body.String())
		}
		if first != "" && response.RunID != first {
			t.Fatal("duplicate job")
		}
		first = response.RunID
		saved, ok := s.agentManager.GetTask(first)
		if !ok || saved.Config.ContextWindow != 8192 || saved.Config.ConversationProfile != "interactive" {
			t.Fatal("allocation was not pinned")
		}
	}
}
