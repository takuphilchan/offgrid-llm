package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestLiveCorrectionRechecksStateAndNeverApproves(t *testing.T) {
	for _, raw := range []string{
		`{"run_id":"run-test","status":"completed","can_steer":true}`,
		`{"run_id":"run-other","status":"running","can_steer":true}`,
		`{"run_id":"run-test","status":"waiting_for_approval","can_steer":true}`,
		`{"run_id":"run-test","status":"waiting_for_input","can_steer":true}`,
		`{"run_id":"run-test","status":"running","can_steer":false}`,
		`{"run_id":"run-test","status":"running","can_steer":true,"uncertain_call_id":"call-old"}`,
	} {
		t.Run(raw, func(t *testing.T) {
			writes := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					writes++
				}
				w.Write([]byte(raw))
			}))
			defer server.Close()
			client, _ := serviceclient.New(server.URL, "", server.Client())
			message, ok := liveAgentControl(context.Background(), client, "run-test")("y")
			if ok || writes != 0 || message == "" {
				t.Fatal(message, ok, writes)
			}
		})
	}
}

func TestLiveCorrectionUnconfirmedIsNotReplayed(t *testing.T) {
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Write([]byte(`{"run_id":"run-test","status":"running","can_steer":true}`))
			return
		}
		if r.URL.Path != "/api/v2/jobs/run-test/steer" {
			t.Errorf("unexpected mutation %s", r.URL.Path)
		}
		writes++
		var request map[string]string
		json.NewDecoder(r.Body).Decode(&request)
		if request["instruction"] != "use Unicode café" || len(request["request_id"]) < 8 {
			t.Error(request)
		}
		w.WriteHeader(503)
		w.Write([]byte(`{"error":{"message":"ack lost"}}`))
	}))
	defer server.Close()
	client, _ := serviceclient.New(server.URL, "", server.Client())
	control := liveAgentControl(context.Background(), client, "run-test")
	message, ok := control("/steer use Unicode café")
	if ok || !strings.Contains(message, "not confirmed") {
		t.Fatal(message, ok)
	}
	_, ok = control("use Unicode café")
	if ok || writes != 1 {
		t.Fatal("uncertain correction replayed", writes)
	}
}

func TestLiveStopReportsActualState(t *testing.T) {
	for _, state := range []string{"running", "cancelled", "uncertain"} {
		t.Run(state, func(t *testing.T) {
			writes := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "GET" {
					w.Write([]byte(`{"run_id":"run-test","status":"running"}`))
					return
				}
				if r.URL.Path != "/api/v2/jobs/run-test/cancel" {
					t.Error(r.URL.Path)
				}
				writes++
				json.NewEncoder(w).Encode(map[string]string{"run_id": "run-test", "status": state})
			}))
			defer server.Close()
			client, _ := serviceclient.New(server.URL, "", server.Client())
			message, ok := liveAgentControl(context.Background(), client, "run-test")("/stop")
			if !ok || writes != 1 || !strings.Contains(message, "Stop response: "+state) {
				t.Fatal(message, ok, writes)
			}
		})
	}
}

func TestLiveCancelledSessionRejectsSteeringButCanStop(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method == http.MethodPost && r.URL.Path != "/api/v2/jobs/run-test/cancel" {
			t.Error("unexpected mutation", r.URL.Path)
		}
		w.Write([]byte(`{"run_id":"run-test","status":"running","can_steer":true}`))
	}))
	defer server.Close()
	client, _ := serviceclient.New(server.URL, "", server.Client())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	control := liveAgentControl(ctx, client, "run-test")
	if _, accepted := control("new instruction"); accepted || requests != 0 {
		t.Fatal("cancelled session sent instruction", requests)
	}
	message, accepted := control("/stop")
	if !accepted || requests != 2 || !strings.Contains(message, "Stop response: running") {
		t.Fatal("stop lost to cancelled parent or claimed completion", message, requests)
	}
}

func TestLiveCorrectionCompletionRaceRetainsDraft(t *testing.T) {
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Write([]byte(`{"run_id":"run-test","status":"running","can_steer":true}`))
			return
		}
		writes++
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"error":{"message":"task completed before correction"}}`))
	}))
	defer server.Close()
	client, _ := serviceclient.New(server.URL, "", server.Client())
	control := liveAgentControl(context.Background(), client, "run-test")
	if _, accepted := control("correction"); accepted {
		t.Fatal("raced completion accepted")
	}
	if _, accepted := control("correction"); accepted || writes != 1 {
		t.Fatal("correction replayed after raced completion", writes)
	}
}
