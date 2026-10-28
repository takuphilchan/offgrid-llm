package server

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/artifacts"
)

func TestArtifactReceiptNeverClaimsHostDestination(t *testing.T) {
	store, err := artifacts.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{artifactStore: store}
	result, err := s.saveTaskArtifact(context.Background(), []byte(`{"name":"zemo.txt","format":"text","content":"hello there"}`))
	if err != nil {
		t.Fatal(err)
	}
	var receipt struct {
		StorageLocation string `json:"storage_location"`
		HostFileCreated *bool  `json:"host_file_created"`
		Verified        bool   `json:"verified"`
		Check           string `json:"check"`
	}
	if json.Unmarshal([]byte(result), &receipt) != nil || receipt.StorageLocation != "offgrid_task_artifact" || receipt.HostFileCreated == nil || *receipt.HostFileCreated || !receipt.Verified || receipt.Check != "stored_bytes_sha256_and_format" {
		t.Fatal(result)
	}
	if taskArtifactDescriptor().Location != "service" {
		t.Fatal("download destination not discoverable")
	}
}
