package execution

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHostReplyIsNotAcknowledgedBeforeServiceReceiptPersistence(t *testing.T) {
	w, b, root := workerFixture(t)
	endpoint := NewLocalEndpoint(w)
	defer endpoint.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := endpoint.Admit(ctx, b); err != nil {
		t.Fatal(err)
	}
	config, err := endpoint.Check(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("b", 64)
	peer, err := newPeer(config, token, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	check, err := peer.Poll(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = peer.Reply(token, DispatchBridgeMessage(ctx, endpoint, config, check)); err != nil {
		t.Fatal(err)
	}
	peer.RecordReceipt = func(context.Context, Request, Response) error { return errors.New("fixture storage unavailable") }
	args, _ := json.Marshal(map[string]any{"path": "saved-once.txt", "data": []byte("saved")})
	original, err := NewIntent(b, "write-once", "file_write", args, false)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := peer.Call(ctx, original); done <- err }()
	message, err := peer.Poll(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	reply := DispatchBridgeMessage(ctx, endpoint, config, message)
	if reply.Response == nil || reply.Response.Receipt == nil || reply.Response.Receipt.State != "complete" {
		t.Fatal(reply)
	}
	if _, err = peer.Reply(token, reply); err == nil {
		t.Fatal("receipt storage failure acknowledged")
	}
	select {
	case err = <-done:
		t.Fatal("caller received unpersisted receipt", err)
	default:
	}
	peer.Close()
	if err = <-done; err == nil {
		t.Fatal("lost persistence treated as success")
	}
	if data, _ := os.ReadFile(filepath.Join(root, "saved-once.txt")); string(data) != "saved" {
		t.Fatal("host effect lost or repeated")
	}
	stored, err := endpoint.InspectReceipt(ctx, original)
	if err != nil || stored.Receipt == nil || stored.Receipt.State != "complete" {
		t.Fatal("native evidence unavailable", stored, err)
	}
}

func TestJournalClaimPersistenceFailurePreventsFilesystemDispatch(t *testing.T) {
	w, b, root := workerFixture(t)
	if err := w.initialize(); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Journal.db.Exec(`CREATE TRIGGER claim_failure BEFORE INSERT ON execution_operations BEGIN SELECT RAISE(FAIL,'fixture'); END;`); err != nil {
		t.Fatal(err)
	}
	response := w.Handle(context.Background(), request(w, b, "file_write", "claim-first", map[string]any{"path": "must-not-exist.txt", "data": []byte("never dispatched")}))
	if response.Error == "" || response.Receipt != nil {
		t.Fatal(response)
	}
	if _, err := os.Stat(filepath.Join(root, "must-not-exist.txt")); !os.IsNotExist(err) {
		t.Fatal("dispatched without a durable claim")
	}
}

func TestReceiptInspectionCannotClaimOrReplayAndWorksAfterJobCancel(t *testing.T) {
	w, b, root := workerFixture(t)
	endpoint := NewLocalEndpoint(w)
	defer endpoint.Close()
	ctx := context.Background()
	if err := endpoint.Admit(ctx, b); err != nil {
		t.Fatal(err)
	}
	args, _ := json.Marshal(map[string]any{"path": "once.txt", "data": []byte("original")})
	original, err := NewIntent(b, "exact-call", "file_write", args, false)
	if err != nil {
		t.Fatal(err)
	}
	missing, err := endpoint.InspectReceipt(ctx, original)
	if err != nil || missing.Receipt == nil || missing.Receipt.State != "uncertain" {
		t.Fatal(missing, err)
	}
	if _, err = os.Stat(filepath.Join(root, "once.txt")); !os.IsNotExist(err) {
		t.Fatal("inspection wrote a file")
	}
	response, err := endpoint.Call(ctx, original)
	if err != nil || response.Receipt == nil || response.Receipt.State != "complete" {
		t.Fatal(response, err)
	}
	put(t, root, "once.txt", "later manual change")
	if err = endpoint.Cancel(ctx, b); err != nil {
		t.Fatal(err)
	}
	inspected, err := endpoint.InspectReceipt(ctx, original)
	if err != nil || inspected.Receipt == nil || string(inspected.Receipt.Result) != string(response.Receipt.Result) {
		t.Fatal(inspected, err)
	}
	if data, _ := os.ReadFile(filepath.Join(root, "once.txt")); string(data) != "later manual change" {
		t.Fatal("receipt lookup replayed write")
	}
	original.Binding.Actor = "foreign"
	if _, err = endpoint.InspectReceipt(ctx, original); err == nil {
		t.Fatal("cross-actor receipt read")
	}
}

func TestEnrolledReceiptReadUsesSameIntentWithoutRepeatingMutation(t *testing.T) {
	e, endpoint, config, pack, token := enrollmentFixture(t)
	ref := onlineEnrollment(t, e, endpoint, config, pack, token)
	serveEnrollmentFixture(t, e, endpoint, config, token)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := e.hosts.Admit(ctx, config.Binding.Actor, config.Binding.Workspace, "job", ref); err != nil {
		t.Fatal(err)
	}
	b := config.Binding
	b.Job = "job"
	args, _ := json.Marshal(map[string]any{"path": "once.txt", "data": []byte("original")})
	original, err := NewIntent(b, "original-call", "file_write", args, false)
	if err != nil {
		t.Fatal(err)
	}
	first, err := e.hosts.Call(ctx, b.Actor, b.Workspace, b.Job, original.ID, ref, original.Operation, args)
	if err != nil || first.Receipt == nil {
		t.Fatal(first, err)
	}
	put(t, config.Root, "once.txt", "do not replace")
	if err = e.hosts.Cancel(ctx, b.Actor, b.Workspace, b.Job, ref); err != nil {
		t.Fatal(err)
	}
	saved, err := e.hosts.InspectReceipt(ctx, b.Actor, b.Workspace, ref, original)
	if err != nil || saved.Receipt == nil || string(saved.Receipt.Result) != string(first.Receipt.Result) {
		t.Fatal(saved, err)
	}
	if data, _ := os.ReadFile(filepath.Join(config.Root, "once.txt")); string(data) != "do not replace" {
		t.Fatal("inspection changed document")
	}
}

func TestProcessSettlementPersistenceFailureRevokesFurtherDispatch(t *testing.T) {
	w, b, _ := workerFixture(t)
	w.Session.config.Permissions.Processes = true
	w.AuthorizeProcess = TrustedProcessPolicy
	if err := w.initialize(); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Journal.db.Exec(`CREATE TRIGGER settlement_failure BEFORE UPDATE ON execution_operations WHEN NEW.state='complete' BEGIN SELECT RAISE(FAIL,'fixture'); END;`); err != nil {
		t.Fatal(err)
	}
	r := request(w, b, "process_start", "one-command", testCommand("exit 0"))
	r.Approved = true
	first := w.Handle(context.Background(), r)
	if first.Receipt == nil || first.Receipt.State != "dispatched" {
		t.Fatal(first)
	}
	w.finishing.Wait()
	if _, err := w.Session.Snapshot(context.Background(), b); err == nil {
		t.Fatal("settlement failure kept grant live")
	}
}

func TestProcessPresentationIsBoundedWithoutChangingHostReceipt(t *testing.T) {
	output := []byte(strings.Repeat("exact", MaxPageBytes))
	encoded, _ := json.Marshal(ProcessResult{Handle: "owned", State: "running", Output: output})
	receipt := Receipt{State: "dispatched", Result: encoded}
	value, err := PresentReceipt(Reference{Host: "host", Session: "session"}, "process_start", receipt)
	if err != nil {
		t.Fatal(err)
	}
	var presented struct {
		Receipt struct {
			Result ProcessPage `json:"result"`
		} `json:"receipt"`
		Text string `json:"text"`
	}
	if json.Unmarshal([]byte(value), &presented) != nil || len(presented.Text) != MaxPageBytes || presented.Receipt.Result.NextOffset == nil || presented.Receipt.Result.CapturedBytes != len(output) || presented.Receipt.Result.Complete {
		t.Fatal("unbounded/inaccurate model projection")
	}
	var original ProcessResult
	if json.Unmarshal(receipt.Result, &original) != nil || len(original.Output) != len(output) {
		t.Fatal("raw evidence rewritten")
	}
}
