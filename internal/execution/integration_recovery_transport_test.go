package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type recoveryReplyFixture struct {
	HostEndpoint
	reply WorkerArtifactReply
}

func (f recoveryReplyFixture) WorkerArtifact(context.Context, WorkerArtifactRequest) (WorkerArtifactReply, error) {
	return f.reply, nil
}

func TestIntegrationRecoveryTransportKeepsStepCursorWhenBoundingFrame(t *testing.T) {
	files := []IntegrationRecoveryFile{}
	for i := 0; i < 64; i++ {
		for _, slot := range []string{"displaced", "staged"} {
			files = append(files, IntegrationRecoveryFile{ID: fmt.Sprintf("%d:%s", i, slot), Path: strings.Repeat("\n", 4096), Role: "unconfirmed"})
		}
	}
	id := digest([]byte("integration"))
	request, err := artifactRequest(WorkerArtifactRequest{Operation: "recovery", IntegrationID: id, Limit: 64})
	if err != nil {
		t.Fatal(err)
	}
	fixture := recoveryReplyFixture{reply: WorkerArtifactReply{State: "observed", Recovery: &IntegrationRecoveryPage{IntegrationID: id, Files: files}}}
	response, err := dispatchWorkerArtifact(context.Background(), fixture, &request)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(response)
	if len(encoded) > MaxFrameBytes {
		t.Fatal("oversized frame", len(encoded))
	}
	decoded, err := decodeArtifactResponse(response)
	if err != nil || decoded.Recovery == nil || decoded.Recovery.NextOffset == nil {
		t.Fatal(decoded, err)
	}
	page := decoded.Recovery
	if *page.NextOffset < 1 || *page.NextOffset >= 64 || len(page.Files) != *page.NextOffset*2 {
		t.Fatal("split or skipped recovery step", page.NextOffset, len(page.Files))
	}
	for i, file := range page.Files {
		ordinal, _, err := recoveryArtifactID(file.ID)
		if err != nil || ordinal != i/2 {
			t.Fatal("lost recovery order", file, err)
		}
	}
}

func TestIntegrationRecoveryPagingCrossesChunksAndRechecksContents(t *testing.T) {
	e, store, owner, _, set, req := integrationFixture(t)
	ctx := context.Background()
	attempt, err := store.IntegrateChangeSet(ctx, e.Worker.Session, req)
	if err != nil {
		t.Fatal(err)
	}
	file := attempt.Files[0]
	private := filepath.Join(store.directory, set.CopyID, "artifacts", "integration-"+attempt.ID, fmt.Sprintf("%06d", file.Ordinal), file.Displaced)
	content := bytes.Repeat([]byte("q"), MaxPageBytes+27)
	store.policy.MaxCopyBytes = 1 << 20
	if err = os.WriteFile(private, content, 0600); err != nil {
		t.Fatal(err)
	}
	observed, err := e.WorkerArtifact(ctx, WorkerArtifactRequest{Operation: "recovery", Source: owner, IntegrationID: attempt.ID, Limit: 1})
	if err != nil || observed.Recovery == nil || len(observed.Recovery.Files) != 1 {
		t.Fatal(observed, err)
	}
	retained := observed.Recovery.Files[0]
	if retained.Role != "unconfirmed" || retained.SHA256 != digest(content) {
		t.Fatal("changed artifact inherited original proof", retained)
	}
	read := WorkerArtifactRequest{Operation: "preserved", Source: owner, IntegrationID: attempt.ID, ArtifactID: retained.ID, Hash: retained.SHA256, Offset: MaxPageBytes - 3, Limit: 17}
	reply, err := e.WorkerArtifact(ctx, read)
	if err != nil || reply.Blob == nil || !bytes.Equal(reply.Blob.Data, content[MaxPageBytes-3:MaxPageBytes+14]) {
		t.Fatal(reply, err)
	}
	content[MaxPageBytes] = 'x'
	if err = os.WriteFile(private, content, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = e.WorkerArtifact(ctx, read); !errors.Is(err, ErrRevision) {
		t.Fatal("tampered requested chunk returned", err)
	}
}
