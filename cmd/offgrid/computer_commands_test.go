package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestComputerRunWithOptionalSuccessCriterion(t *testing.T) {
	for _, args := range [][]string{{"run", "s", "m"}, {"run", "s", "m", "--expect", "", "save draft"}, {"run", "s", "m", "--expect"}, {"run", "s", "m", " "}} {
		err := runComputerCommand(context.Background(), args)
		var out, stderr bytes.Buffer
		if reportCommandError(err, true, &out, &stderr) != 2 {
			t.Fatalf("malformed command must be invalid usage: %v", err)
		}
	}
	var received struct {
		Session  string `json:"computer_session"`
		Expected string `json:"computer_expected_text"`
		Prompt   string `json:"prompt"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-key" {
			t.Error("missing authentication")
		}
		if r.URL.Path != "/v1/agents/run" || r.Method != "POST" {
			t.Error("wrong endpoint")
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(202)
		_, _ = w.Write([]byte(`{"run_id":"fixture-run","status":"pending"}`))
	}))
	defer server.Close()
	t.Setenv("OFFGRID_SERVER_URL", server.URL)
	t.Setenv("OFFGRID_API_KEY", "fixture-key")
	if err := runComputerCommand(context.Background(), []string{"run", "session", "model", "--expect", "Saved 日本語", "Save", "draft"}); err != nil {
		t.Fatal(err)
	}
	if received.Session != "session" || received.Expected != "Saved 日本語" || received.Prompt != "Save draft" {
		t.Fatalf("wrong submission: %+v", received)
	}
	if err := runComputerCommand(context.Background(), []string{"run", "session", "model", "Save", "draft"}); err != nil {
		t.Fatal(err)
	}
	if received.Expected != "" || received.Prompt != "Save draft" {
		t.Fatalf("automatic verification submission: %+v", received)
	}
}
