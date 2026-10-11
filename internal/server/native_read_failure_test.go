package server

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

func TestApplicationReadinessFailureKeepsBoundedReasonAndLegacyResponses(t *testing.T) {
	args := json.RawMessage(`{"application_id":"app"}`)
	for _, state := range []string{"", "starting", "surface_unavailable", "observation_unavailable", "unsupported_identity"} {
		payload, _ := json.Marshal(map[string]any{"application_id": "app", "verified": false, "check": "application_open", "readiness": state})
		failure := applicationReadinessFailure(string(payload), args)
		if failure == nil || failure.Code != "computer_application_not_ready" || failure.Readiness == "" || (failure.Recovery == "") != (state == "unsupported_identity") {
			t.Fatal(state, failure)
		}
	}
	for _, payload := range []string{
		`{"application_id":"other","verified":true,"check":"application_open"}`,
		`{"application_id":"app","verified":false,"check":"application_open","readiness":"invented"}`,
		`{"application_id":"app","verified":true,"check":"application_open","readiness":"starting"}`,
		`{}`, `null`,
	} {
		if failure := applicationReadinessFailure(payload, args); failure == nil || failure.Recovery != "" {
			t.Fatal("malformed readiness accepted", payload)
		}
	}
	if failure := applicationReadinessFailure(`{"application_id":"app","verified":true,"check":"application_open"}`, args); failure != nil {
		t.Fatal("legacy success rejected", failure)
	}
}

func TestNativeReadFailureRecoveryIsTypedAndSanitized(t *testing.T) {
	for _, tc := range []struct {
		err    error
		code   string
		repair bool
	}{
		{computer.ErrTargetNotSelected, "computer_target_not_selected", true},
		{computer.ErrApplicationNotReady, "computer_application_not_ready", true},
		{computer.ErrDocumentChanged, "computer_document_changed", true},
		{computer.ErrControlScope, "computer_scope_violation", false},
		{computer.ErrUncertain, "computer_read_interrupted", false},
		{errors.New("private document and provider details"), "computer_read_failed", false},
	} {
		failure := nativeReadFailure(tc.err)
		if failure.Code != tc.code || (failure.Recovery != "") != tc.repair || strings.Contains(failure.Message, "private document") {
			t.Fatalf("unsafe recovery: %+v", failure)
		}
	}
}
