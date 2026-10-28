package cli

import (
	"bufio"
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestCLIExplicitRememberSkipsLaterReviewUntilRevoked(t *testing.T) {
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	store := &execution.FolderTrustStore{Directory: filepath.Join(t.TempDir(), "private")}
	env.Execution.Trust = func() (*execution.FolderTrustStore, error) { return store, nil }
	root := t.TempDir()
	approval := execution.LocalApproval{Backend: "local", Root: root, Actor: "alice", Host: "host", Service: "http://fixture:11611", Workspace: "workspace", Permissions: execution.Permissions{Read: true}}
	opens := 0
	env.Execution.Open = func(_ context.Context, _ *serviceclient.Client, _ string, p execution.Permissions, approve func(execution.LocalApproval) bool) (ExecutionLease, error) {
		opens++
		current := approval
		current.Permissions = p
		if !approve(current) {
			return nil, execution.ErrAuthority
		}
		return &fixtureExecutionLease{done: make(chan struct{})}, nil
	}
	client, _ := serviceclient.New("http://fixture:11611", "", env.HTTP)
	for _, answer := range []string{"remember\n", ""} {
		lease, err := openCLIWorkspace(context.Background(), client, root, "read", bufio.NewReader(strings.NewReader(answer)), &out, env)
		if err != nil {
			t.Fatal(err, out.String())
		}
		_ = lease.Close()
	}
	if opens != 2 || strings.Count(out.String(), "[y/remember/N]") != 1 || !strings.Contains(out.String(), "Using remembered") {
		t.Fatal(out.String(), opens)
	}
	if _, err := openCLIWorkspace(context.Background(), client, root, "edit", bufio.NewReader(strings.NewReader("n\n")), &out, env); err == nil {
		t.Fatal("remember widened permission")
	}
	records, err := store.List()
	if err != nil || len(records) != 1 {
		t.Fatal(records, err)
	}
	if err = store.Revoke(records[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := openCLIWorkspace(context.Background(), client, root, "read", bufio.NewReader(strings.NewReader("n\n")), &out, env); err == nil {
		t.Fatal("revoked trust approved")
	}
}
