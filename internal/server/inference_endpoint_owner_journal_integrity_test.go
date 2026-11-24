package server

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

func TestEndpointOwnerSettledReceiptCannotBeRewrittenAfterLaterPublication(t *testing.T) {
	s := endpointOwnerFixture(t)
	input := endpointOwnerInputFixture()
	review := prepareEndpointOwnerFixture(t, s, "", input)
	if response := commitEndpointOwnerFixture(s, "", review, "first-change"); response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	first := reconcileEndpointOwnerFixture(t, s, "", "first-change")
	input.Replace, input.Revision = input.ID+"@"+input.Revision, "r2"
	review = prepareEndpointOwnerFixture(t, s, "", input)
	if response := commitEndpointOwnerFixture(s, "", review, "second-change"); response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	reconcileEndpointOwnerFixture(t, s, "", "second-change")
	before, err := readEndpointOwnerJournal(s.config.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	state := endpointOwnerSnapshotFixture(t, s, "")
	response := endpointManagementRequest(s, "", "/configuration/reconcile", map[string]string{"request_id": "first-change", "expected_revision": state.FileRevision})
	var returned endpointOwnerRecord
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &returned) != nil || !reflect.DeepEqual(returned, first) {
		t.Fatal("a settled receipt changed after a later publication", response.Code, response.Body.String())
	}
	after, err := readEndpointOwnerJournal(s.config.DataDir)
	if err != nil || !reflect.DeepEqual(before, after) || endpointOwnerJournalReady(s.config.DataDir) != nil {
		t.Fatal("historical inspection changed audit or blocked current policy", after, err)
	}
}

func TestEndpointOwnerJournalRejectsMalformedProofFields(t *testing.T) {
	mutations := map[string]func(*endpointOwnerRecord){
		"nonhex review":              func(r *endpointOwnerRecord) { r.ReviewID = strings.Repeat("z", 64) },
		"nonhex before":              func(r *endpointOwnerRecord) { r.Before = strings.Repeat("z", 64) },
		"nonhex after":               func(r *endpointOwnerRecord) { r.After = strings.Repeat("z", 64) },
		"nonhex epoch":               func(r *endpointOwnerRecord) { r.Epoch = strings.Repeat("z", 64) },
		"invalid endpoint":           func(r *endpointOwnerRecord) { r.Endpoint = "endpoint@r1@other" },
		"invalid created time":       func(r *endpointOwnerRecord) { r.CreatedAt = "not-a-time" },
		"missing resolved time":      func(r *endpointOwnerRecord) { r.ResolvedAt = "" },
		"invalid resolved time":      func(r *endpointOwnerRecord) { r.ResolvedAt = "not-a-time" },
		"missing resolver":           func(r *endpointOwnerRecord) { r.ResolvedBy = "" },
		"invalid resolved epoch":     func(r *endpointOwnerRecord) { r.ResolvedEpoch = strings.Repeat("z", 64) },
		"unresolved with resolution": func(r *endpointOwnerRecord) { r.State = "pending" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			s := endpointOwnerFixture(t)
			review := prepareEndpointOwnerFixture(t, s, "", endpointOwnerInputFixture())
			if response := commitEndpointOwnerFixture(s, "", review, "proof-fixture"); response.Code != 200 {
				t.Fatal(response.Code, response.Body.String())
			}
			record := reconcileEndpointOwnerFixture(t, s, "", "proof-fixture")
			mutate(&record)
			if err := storage.WriteDurableJSON(filepath.Join(s.config.DataDir, endpointOwnerJournalFilename), endpointOwnerJournal{Version: 1, Records: []endpointOwnerRecord{record}}); err != nil {
				t.Fatal(err)
			}
			if _, err := readEndpointOwnerJournal(s.config.DataDir); err == nil {
				t.Fatal("malformed proof accepted")
			}
			if err := endpointOwnerJournalReady(s.config.DataDir); err == nil {
				t.Fatal("malformed proof admitted endpoint access")
			}
		})
	}
}
