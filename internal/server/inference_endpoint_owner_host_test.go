package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

func endpointOwnerInputFixture() endpointOwnerInput {
	entry := endpointEnrollmentFixture()
	return endpointOwnerInput{ID: entry.ID, Revision: entry.Revision, Provider: entry.Provider, URL: entry.URL, AllowedIPs: entry.AllowedIPs, Actors: entry.Actors, Model: entry.Model, ModelDigest: entry.ModelDigest, Format: entry.Format, RuntimeVersion: entry.RuntimeVersion, ContextLength: entry.ContextLength, DeploymentAttested: true}
}

func TestEndpointOwnerCredentialReferenceBindsExactOrigin(t *testing.T) {
	input := endpointOwnerInputFixture()
	input.Credential = "approved-lab"
	host := endpointOwnerHost{Version: 1, Credentials: []endpointCredentialReference{{ID: input.Credential, Environment: "OFFGRID_FIXTURE_APPROVED_CREDENTIAL", Origins: []string{input.URL}}}}
	entry, err := input.entry(host)
	if err != nil || entry.CredentialEnv != host.Credentials[0].Environment {
		t.Fatal(entry, err)
	}
	for _, mutate := range []func(*endpointOwnerInput){func(i *endpointOwnerInput) { i.Credential = "AWS_SECRET_ACCESS_KEY" }, func(i *endpointOwnerInput) { i.URL = "https://other.example.test" }, func(i *endpointOwnerInput) { i.URL += "/" }, func(i *endpointOwnerInput) { i.Provider = "llama_cpp" }} {
		changed := input
		mutate(&changed)
		if _, err := changed.entry(host); err == nil {
			t.Fatal("invalid credential destination accepted", changed)
		}
	}
	raw, _ := json.Marshal(host)
	path := filepath.Join(t.TempDir(), "installation-policy.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	_, pin, err := readEndpointOwnerHost(path)
	if err != nil || len(pin) != 64 {
		t.Fatal(pin, err)
	}
	host.Credentials[0].Origins = []string{"https://user:secret@example.test"}
	raw, _ = json.Marshal(host)
	os.WriteFile(path, raw, 0600)
	if _, _, err := readEndpointOwnerHost(path); err == nil {
		t.Fatal("credential-bearing installation origin accepted")
	}
}

func TestEndpointOwnerExternalWriterRequiresDelegationAndExclusiveLease(t *testing.T) {
	workspace := endpointOwnerFixtureDirectory(t)
	external := endpointOwnerFixtureDirectory(t)
	path := filepath.Join(external, "endpoints.json")
	if err := storage.WriteDurableJSON(path, inferenceEndpointFile{Version: 1, Endpoints: []inferenceEndpointEnrollment{endpointEnrollmentFixture()}}); err != nil {
		t.Fatal(err)
	}
	if release, err := acquireEndpointOwnerWriter(path, workspace, ""); err == nil {
		release()
		t.Fatal("external writer lacked installation delegation")
	}
	release, err := acquireEndpointOwnerWriter(path, workspace, path)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if other, err := acquireEndpointOwnerWriter(path, t.TempDir(), path); err == nil {
		other()
		t.Fatal("second workspace acquired the same external writer")
	}
}

func TestEndpointOwnerJournalRejectsUnknownOutcomeAndAmbiguousJSON(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, endpointOwnerJournalFilename)
	for _, raw := range []string{`{"version":1,"version":1,"records":[]}`, `{"version":1,"records":[]} {}`, `{"version":2,"records":[]}`} {
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if err := endpointOwnerJournalReady(directory); err == nil {
			t.Fatal("malformed owner journal authorized requests", raw)
		}
	}
}
