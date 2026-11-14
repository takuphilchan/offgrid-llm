package execution

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestIntegrationRecoveryDownloadsPreservedFilesWithoutReplay(t *testing.T) {
	for _, crash := range []bool{false, true} {
		t.Run(fmt.Sprint(crash), func(t *testing.T) {
			e, store, owner, source, _, req := integrationFixture(t)
			ctx := context.Background()
			var hooks *integrationHooks
			if crash {
				hooks = &integrationHooks{afterDispatch: func(int) error { return errors.New("lost acknowledgment") }}
			}
			attempt, err := store.integrateChangeSet(ctx, e.Worker.Session, req, hooks)
			if !crash && err != nil || crash && (err == nil || attempt.State != "uncertain") {
				t.Fatal(attempt, err)
			}
			// Observation stays possible without admitting another execution job.
			writeCandidateFile(t, source, "input.txt", "newer user edit")
			request := WorkerArtifactRequest{Operation: "recovery", Source: owner, IntegrationID: attempt.ID, Limit: 1}
			var files []IntegrationRecoveryFile
			for {
				reply, err := e.WorkerArtifact(ctx, request)
				if err != nil || reply.Recovery == nil {
					t.Fatal(reply, err)
				}
				files = append(files, reply.Recovery.Files...)
				if reply.Recovery.NextOffset == nil {
					break
				}
				if int64(*reply.Recovery.NextOffset) <= request.Offset {
					t.Fatal("cursor failed to advance")
				}
				request.Offset = int64(*reply.Recovery.NextOffset)
			}
			if len(files) == 0 {
				t.Fatal("preserved files inaccessible", attempt)
			}
			for _, file := range files {
				if file.Limitation != "" {
					t.Fatal(file)
				}
				if (file.Role == "displaced") == crash {
					t.Fatal("observations changed receipt confidence", file)
				}
				var data []byte
				read := WorkerArtifactRequest{Operation: "preserved", Source: owner, IntegrationID: attempt.ID, ArtifactID: file.ID, Hash: file.SHA256, Limit: 3}
				for {
					reply, err := e.WorkerArtifact(ctx, read)
					if err != nil || reply.Blob == nil || reply.Blob.SHA256 != file.SHA256 || reply.Blob.Offset != read.Offset {
						t.Fatal(reply, err)
					}
					data = append(data, reply.Blob.Data...)
					if reply.Blob.NextOffset == nil {
						break
					}
					read.Offset = *reply.Blob.NextOffset
				}
				want := "dirty original"
				if file.Path == "remove.txt" {
					want = "retain until integration"
				}
				if string(data) != want || digest(data) != file.SHA256 {
					t.Fatal("lost preserved original", file, string(data))
				}
				foreign := read
				foreign.Source.Job = "another-job"
				if _, err = e.WorkerArtifact(ctx, foreign); !errors.Is(err, ErrAuthority) {
					t.Fatal("foreign recovery read", err)
				}
				read.Hash = digest([]byte("unrelated"))
				if _, err = e.WorkerArtifact(ctx, read); !errors.Is(err, ErrRevision) {
					t.Fatal("unbound digest accepted", err)
				}
			}
			after, err := store.InspectIntegration(ctx, attempt.ID, owner, 0, 64)
			if err != nil || after.State != attempt.State || after.Integrated != attempt.Integrated {
				t.Fatal("observation settled work", after, err)
			}
			proposal, err := store.ReadChangeBlob(ctx, req.ChangeSetID, owner, digest([]byte("candidate input")), 0, MaxPageBytes)
			if err != nil || string(proposal.Data) != "candidate input" {
				t.Fatal("uncertain integration hid sealed proposal", proposal, err)
			}
			current, err := os.ReadFile(filepath.Join(source, "input.txt"))
			if err != nil || string(current) != "newer user edit" {
				t.Fatal("recovery overwrote user", string(current), err)
			}
			e.Worker.Session.Revoke()
			if _, err = e.WorkerArtifact(ctx, request); !errors.Is(err, ErrAuthority) {
				t.Fatal("revocation ignored", err)
			}
		})
	}
}

func TestIntegrationRecoveryReopenObservesWithoutSettling(t *testing.T) {
	e, store, owner, _, _, req := integrationFixture(t)
	ctx := context.Background()
	attempt, err := store.integrateChangeSet(ctx, e.Worker.Session, req, &integrationHooks{afterDispatch: func(int) error { return errors.New("lost receipt") }})
	if err == nil || attempt.State != "uncertain" {
		t.Fatal(attempt, err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenWorkingCopies(store.directory, store.policy)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	// No session, execution grant or background job is reconstructed.
	page, err := reopened.inspectRecoveryLocked(ctx, attempt.ID, owner, 0, 64)
	if err != nil || len(page.Files) != 1 || page.Files[0].Role != "unconfirmed" {
		t.Fatal(page, err)
	}
	file := page.Files[0]
	blob, err := reopened.readRecoveryBlobLocked(ctx, attempt.ID, owner, file.ID, file.SHA256, 0, MaxPageBytes)
	if err != nil || string(blob.Data) != "retain until integration" {
		t.Fatal(blob, err)
	}
	status, err := reopened.InspectIntegration(ctx, attempt.ID, owner, 0, 64)
	if err != nil || status.State != "uncertain" || status.Integrated != 0 {
		t.Fatal("observation changed outcome", status, err)
	}
}

func TestIntegrationRecoveryRejectsTamperedChunksAndSelectors(t *testing.T) {
	e, store, owner, _, set, req := integrationFixture(t)
	ctx := context.Background()
	page, err := store.IntegrateChangeSet(ctx, e.Worker.Session, req)
	if err != nil {
		t.Fatal(err)
	}
	request := WorkerArtifactRequest{Operation: "recovery", Source: owner, IntegrationID: page.ID, Limit: 64}
	reply, err := e.WorkerArtifact(ctx, request)
	if err != nil || reply.Recovery == nil || len(reply.Recovery.Files) != 2 {
		t.Fatal(reply, err)
	}
	file := reply.Recovery.Files[len(reply.Recovery.Files)-1]
	read := WorkerArtifactRequest{Operation: "preserved", Source: owner, IntegrationID: page.ID, ArtifactID: file.ID, Hash: file.SHA256, Limit: 3}
	if _, err = e.WorkerArtifact(ctx, read); err != nil {
		t.Fatal(err)
	}
	ordinal, _, _ := recoveryArtifactID(file.ID)
	actual := page.Files[ordinal]
	name := filepath.Join(store.directory, set.CopyID, "artifacts", "integration-"+page.ID, fmt.Sprintf("%06d", ordinal), actual.Displaced)
	info, err := os.Stat(name)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(name, bytes.Repeat([]byte("x"), int(info.Size())), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = e.WorkerArtifact(ctx, read); !errors.Is(err, ErrRevision) {
		t.Fatal("cached index trusted corrupted content", err)
	}
	for _, id := range []string{"../displaced", "-1:displaced", "0:../../secret", "00:staged", "200001:staged"} {
		read.ArtifactID = id
		if _, err = e.WorkerArtifact(ctx, read); !errors.Is(err, ErrProtocol) {
			t.Fatal("unsafe selector", id, err)
		}
	}
}
