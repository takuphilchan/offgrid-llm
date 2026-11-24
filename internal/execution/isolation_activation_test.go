package execution

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type activationTransport func(*http.Request) (*http.Response, error)

func (f activationTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// The daemon fixture uses the production journal/worker but never launches a
// process or contacts Docker. The attach handshake is held before any start.
func TestIsolationActivationCancellation(t *testing.T) {
	for _, mode := range []string{"revoke", "cancel", "revoke-cleanup-failure", "cancel-cleanup-failure", "revoke-foreign-owner", "revoke-forget-failure"} {
		t.Run(mode, func(t *testing.T) {
			w, binding, starts := isolatedWorkerFixture(t, "")
			i := w.Processes.isolation
			next := i.client.Transport
			entered, released, cancelled := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var interrupted atomic.Bool
			var deletes atomic.Int32
			if mode == "revoke-forget-failure" {
				if _, err := w.Journal.db.Exec(`CREATE TRIGGER fixture_retain_container BEFORE DELETE ON execution_containers BEGIN SELECT RAISE(FAIL,'fixture journal unavailable'); END;`); err != nil {
					t.Fatal(err)
				}
			}
			i.client.Transport = activationTransport(func(r *http.Request) (*http.Response, error) {
				if strings.HasSuffix(r.URL.Path, "/attach") {
					close(entered)
					select {
					case <-r.Context().Done():
						close(cancelled)
						return nil, r.Context().Err()
					case <-released:
						return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
					}
				}
				if interrupted.Load() && strings.HasSuffix(mode, "cleanup-failure") && strings.HasSuffix(r.URL.Path, "/json") {
					return &http.Response{StatusCode: 503, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
				}
				if r.Method == http.MethodDelete {
					deletes.Add(1)
				}
				response, err := next.RoundTrip(r)
				if err == nil && interrupted.Load() && mode == "revoke-foreign-owner" && strings.HasSuffix(r.URL.Path, "/json") {
					var observed containerInspection
					err = json.NewDecoder(response.Body).Decode(&observed)
					response.Body.Close()
					if err != nil {
						return nil, err
					}
					observed.Config.Labels["io.offgrid.executor.ownership"] = strings.Repeat("d", 64)
					data, _ := json.Marshal(observed)
					response.Body = io.NopCloser(strings.NewReader(string(data)))
				}
				return response, err
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			r := request(w, binding, "process_start", "activation-once", ProcessSpec{Program: "go", Arguments: []string{"test", "."}})
			done := make(chan Response, 1)
			go func() { done <- w.Handle(ctx, r) }()
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("activation did not reach attach")
			}
			interrupted.Store(true)
			if strings.HasPrefix(mode, "revoke") {
				w.Session.Revoke()
			} else {
				cancel()
			}
			select {
			case <-cancelled:
			case <-time.After(time.Second):
				t.Error("held attach did not observe activation cancellation")
			}
			close(released)
			var response Response
			select {
			case response = <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("activation did not finish after cancellation")
			}
			w.finishing.Wait()
			if *starts != 0 {
				t.Errorf("started %d containers after activation cancellation", *starts)
			}
			retained, err := w.Journal.ownedContainers(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if mode != "revoke" && mode != "cancel" {
				wantDeletes := int32(0)
				if mode == "revoke-forget-failure" {
					wantDeletes = 1
				}
				if len(retained) != 1 || retained[0].ID == "" || deletes.Load() != wantDeletes || response.Receipt.State != "uncertain" {
					t.Errorf("lost uncertain owned cleanup: retained=%+v deletes=%d response=%+v", retained, deletes.Load(), response)
				}
			} else if len(retained) != 0 || deletes.Load() != 1 {
				t.Errorf("created container not reconciled: retained=%+v deletes=%d response=%+v", retained, deletes.Load(), response)
			}
			previous, fresh, err := w.Journal.Claim(context.Background(), r.OperationID, r.Payload())
			if err != nil || fresh || previous.State == "dispatched" {
				t.Errorf("cancelled activation became replayable: fresh=%t receipt=%+v err=%v", fresh, previous, err)
			}
			_ = w.Handle(context.Background(), r)
			if *starts != 0 {
				t.Error("duplicate request started cancelled activation")
			}
			// Restore inspection solely for fixture cleanup; the earlier failure
			// and retained recovery record were asserted before this change.
			interrupted.Store(false)
			if mode == "revoke-forget-failure" {
				if _, err := w.Journal.db.Exec(`DROP TRIGGER fixture_retain_container`); err != nil {
					t.Fatal(err)
				}
			}
			if err := i.recover(context.Background(), w.Journal); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestIsolationActivationRechecksAttachAcknowledgment(t *testing.T) {
	w, binding, starts := isolatedWorkerFixture(t, "")
	i := w.Processes.isolation
	next := i.client.Transport
	var deletes int
	i.client.Transport = activationTransport(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/attach") {
			// Simulate an acknowledgment already received as authority is
			// revoked. The dispatch check must not rely on watcher scheduling.
			w.Session.Revoke()
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
		}
		if r.Method == http.MethodDelete {
			deletes++
		}
		return next.RoundTrip(r)
	})
	r := request(w, binding, "process_start", "activation-ack", ProcessSpec{Program: "go", Arguments: []string{"test", "."}})
	response := w.Handle(context.Background(), r)
	w.finishing.Wait()
	if *starts != 0 || deletes != 1 || response.Receipt == nil || response.Receipt.State != "rejected" {
		t.Fatalf("revoked attach acknowledgment activated: starts=%d deletes=%d response=%+v", *starts, deletes, response)
	}
	retained, err := w.Journal.ownedContainers(context.Background())
	if err != nil || len(retained) != 0 {
		t.Fatal("owned created container not reconciled", retained, err)
	}
}
