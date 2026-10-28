package execution

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNativeHostSeedRejectsRemoteAuthorityAndUnsafePermissions(t *testing.T) {
	r := NativeHostRequest{Protocol: 1, Service: "http://127.0.0.1:11611", Pairing: Pairing{Protocol: 1, Code: strings.Repeat("a", 64), Actor: "local", Workspace: "workspace"}, Folder: filepath.Join(t.TempDir(), "selected"), Permissions: Permissions{Read: true}}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*NativeHostRequest){func(r *NativeHostRequest) { r.Service = "https://remote.invalid" }, func(r *NativeHostRequest) { r.Service = "http://user:secret@127.0.0.1:11611" }, func(r *NativeHostRequest) { r.Service += "/foreign" }, func(r *NativeHostRequest) { r.Service += "?token=secret" }, func(r *NativeHostRequest) { r.Folder = "relative" }, func(r *NativeHostRequest) { r.Permissions = Permissions{Read: true, Processes: true} }, func(r *NativeHostRequest) { r.Protocol = 2 }, func(r *NativeHostRequest) { r.Pairing.Actor = "" }} {
		bad := r
		change(&bad)
		if bad.Validate() == nil {
			t.Fatal("invalid seed accepted")
		}
	}
}

func TestNativeHostInputCancellationCannotCreateAuthority(t *testing.T) {
	input, writer := io.Pipe()
	defer writer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- ServeNativeHost(ctx, input, &bytes.Buffer{}, t.TempDir(), t.TempDir(), "development") }()
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled setup succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("seed read blocked cancellation")
	}
}
