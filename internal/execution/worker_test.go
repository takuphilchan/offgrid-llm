package execution

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func workerFixture(t *testing.T) (*Worker, Binding, string) {
	s, b, root := fixture(t)
	j, err := OpenJournal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := NewProcessManager(s)
	w := &Worker{Session: s, Journal: j, Processes: m, Token: strings.Repeat("a", 64)}
	t.Cleanup(func() { m.Close(); w.finishing.Wait(); j.Close() })
	return w, b, root
}

func TestMissingExecutableHasSafeDurableRejection(t *testing.T) {
	w, b, _ := workerFixture(t)
	w.Session.config.Permissions.Processes = true
	w.AuthorizeProcess = TrustedProcessPolicy
	r := request(w, b, "process_start", "missing-command", ProcessSpec{Program: "offgrid-fixture-unavailable-command", Arguments: []string{"test"}})
	r.Approved = true
	response := w.Handle(context.Background(), r)
	if response.Receipt == nil || response.Receipt.State != "rejected" || string(response.Receipt.Result) != `{"error":"execution_program_unavailable"}` {
		t.Fatal(response)
	}
	repeated := w.Handle(context.Background(), r)
	if repeated.Receipt == nil || repeated.Receipt.State != "rejected" || string(repeated.Receipt.Result) != string(response.Receipt.Result) {
		t.Fatal("missing command replay changed receipt", repeated)
	}
}

func TestWorkerReceiptFailureBlocksFurtherDispatch(t *testing.T) {
	w, b, root := workerFixture(t)
	if err := w.initialize(); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Journal.db.Exec(`CREATE TRIGGER fixture_receipt_failure BEFORE UPDATE ON execution_operations BEGIN SELECT RAISE(FAIL,'fixture receipt unavailable'); END;`); err != nil {
		t.Fatal(err)
	}
	r := request(w, b, "file_write", "write-before-failure", map[string]any{"path": "saved.txt", "data": []byte("saved once")})
	result := w.Handle(context.Background(), r)
	if result.Error != "execution_receipt_unavailable" || result.Receipt == nil || result.Receipt.State != "uncertain" {
		t.Fatal(result)
	}
	data, err := os.ReadFile(filepath.Join(root, "saved.txt"))
	if err != nil || string(data) != "saved once" {
		t.Fatal(string(data), err)
	}
	if got := w.Handle(context.Background(), r); got.Error != ErrAuthority.Error() {
		t.Fatal("dispatched after receipt loss", got)
	}
}

func TestWorkerRestoresDirectoryWithoutReplayingChange(t *testing.T) {
	s, b, root := fixture(t)
	original := s.config
	if err := os.Mkdir(filepath.Join(root, "child"), 0700); err != nil {
		t.Fatal(err)
	}
	put(t, root, "outside.txt", "outside child")
	put(t, filepath.Join(root, "child"), "inside.txt", "inside child")
	dir := t.TempDir()
	j, err := OpenJournal(dir)
	if err != nil {
		t.Fatal(err)
	}
	w := &Worker{Session: s, Journal: j, Processes: NewProcessManager(s), Token: strings.Repeat("b", 64)}
	r := request(w, b, "directory_change", "change-once", map[string]any{"path": "child"})
	first := w.Handle(context.Background(), r)
	if first.Error != "" || first.Receipt.State != "complete" {
		t.Fatal(first)
	}
	w.Processes.Close()
	j.Close()
	s.Close()
	s2, err := OpenSession(original)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	j2, err := OpenJournal(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer j2.Close()
	w2 := &Worker{Session: s2, Journal: j2, Processes: NewProcessManager(s2), Token: w.Token}
	defer w2.Processes.Close()
	replayed := w2.Handle(context.Background(), r)
	if replayed.Error != "" || string(replayed.Receipt.Result) != string(first.Receipt.Result) {
		t.Fatal(replayed)
	}
	result := w2.Handle(context.Background(), request(w2, b, "file_list", "list-child", map[string]any{"path": "."}))
	if result.Error != "" || !strings.Contains(string(result.Receipt.Result), "inside.txt") || strings.Contains(string(result.Receipt.Result), "outside.txt") {
		t.Fatal(result)
	}
}

func TestWorkerProcessReceiptSettlesWithActualExit(t *testing.T) {
	w, b, _ := workerFixture(t)
	w.Session.config.Permissions.Processes = true
	w.AuthorizeProcess = func(context.Context, Binding, ProcessSpec, bool) error { return nil }
	r := request(w, b, "process_start", "one-process", testCommand("exit 7"))
	result := w.Handle(context.Background(), r)
	if result.Error != "" || result.Receipt.State != "dispatched" {
		t.Fatal(result)
	}
	w.finishing.Wait()
	result = w.Handle(context.Background(), r)
	if result.Error != "" || result.Receipt.State != "complete" {
		t.Fatal(result)
	}
	var process ProcessResult
	if err := json.Unmarshal(result.Receipt.Result, &process); err != nil || process.State != "failed" || process.ExitCode == nil || *process.ExitCode != 7 {
		t.Fatal(process, err)
	}
}

func request(w *Worker, b Binding, op, id string, args any) Request {
	data, _ := json.Marshal(args)
	return Request{Version: 1, ID: id, OperationID: id, Binding: b, Token: w.Token, Operation: op, Arguments: data}
}

func TestWorkerWriteReplayAndAuthority(t *testing.T) {
	w, b, root := workerFixture(t)
	ctx := context.Background()
	r := request(w, b, "file_write", "write-one", map[string]any{"path": "new.txt", "data": []byte("café 日本語")})
	first := w.Handle(ctx, r)
	if first.Error != "" || first.Receipt == nil || first.Receipt.State != "complete" {
		t.Fatal(first)
	}
	put(t, root, "new.txt", "manual change")
	replayed := w.Handle(ctx, r)
	if replayed.Error != "" || string(replayed.Receipt.Result) != string(first.Receipt.Result) {
		t.Fatal(replayed)
	}
	read, err := w.Session.Read(ctx, b, "new.txt", 0, MaxPageBytes, "")
	if err != nil || string(read.Data) != "manual change" {
		t.Fatal(read, err)
	}
	r.Arguments = json.RawMessage(`{"path":"different.txt","data":"YQ=="}`)
	if got := w.Handle(ctx, r); got.Error != ErrConflict.Error() {
		t.Fatal(got)
	}
	r.Token = "incorrect"
	if got := w.Handle(ctx, r); got.Error != ErrAuthority.Error() {
		t.Fatal(got)
	}
	r.Token = w.Token
	r.Binding.Actor = "bob"
	if got := w.Handle(ctx, r); got.Error != ErrAuthority.Error() {
		t.Fatal(got)
	}
}

func TestWorkerRejectsUnreviewedProcessesAndMalformedArguments(t *testing.T) {
	w, b, _ := workerFixture(t)
	ctx := context.Background()
	for _, r := range []Request{
		request(w, b, "process_start", "process", testCommand("echo should-not-run")),
		request(w, b, "workspace_info", "unknown-field", map[string]any{"extra": true}),
		request(w, b, "arbitrary-script", "unknown-op", map[string]any{}),
	} {
		if got := w.Handle(ctx, r); got.Error == "" {
			t.Fatal(got)
		}
	}
	r := request(w, b, "file_read", "null", nil)
	if got := w.Handle(ctx, r); got.Error == "" {
		t.Fatal(got)
	}
}

func TestSupervisedWorkerFramesAndCancellation(t *testing.T) {
	w, b, root := workerFixture(t)
	put(t, root, "hello.txt", "verified")
	input, send := io.Pipe()
	receive, output := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- w.Serve(ctx, input, output) }()
	var response Response
	if err := ReadFrame(receive, &response); err != nil || response.ID != "startup" {
		t.Fatal(response, err)
	}
	if err := WriteFrame(send, request(w, b, "file_list", "read-one", map[string]any{"path": "."})); err != nil {
		t.Fatal(err)
	}
	if err := ReadFrame(receive, &response); err != nil || response.Error != "" || !strings.Contains(string(response.Receipt.Result), "hello.txt") {
		t.Fatal(response, err)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker cancellation blocked on input")
	}
	send.Close()
	receive.Close()
	output.Close()
}
