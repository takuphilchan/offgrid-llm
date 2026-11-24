package inference

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestHTTPProviderPolicyRevocationRejectsBufferedTerminalAfterCallback(t *testing.T) {
	endpoint, _ := httpProviderFixture(t, ProviderOllama, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"model":"fixture-model","message":{"content":"provisional"},"done":false}`)
		fmt.Fprintln(w, `{"model":"fixture-model","message":{"content":""},"done":true,"done_reason":"stop","prompt_eval_count":1,"eval_count":1}`)
	})
	policy, revoke := context.WithCancel(context.Background())
	defer revoke()
	admissions := make(chan context.Context, 16)
	endpoint.AcquireRequest = func(ctx context.Context) (context.Context, func(), error) {
		bound, cancel := context.WithCancel(ctx)
		stop := context.AfterFunc(policy, cancel)
		admissions <- bound
		return bound, func() { stop(); cancel() }, nil
	}
	provider, err := NewHTTPProvider(context.Background(), endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer provider.Close()
	for len(admissions) > 0 {
		<-admissions
	}
	entered, unblock := make(chan struct{}), make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(unblock) }) }
	defer release()
	done := make(chan error, 1)
	terminal := false
	go func() {
		done <- provider.Generate(context.Background(), httpProviderRequest(t, endpoint, nil), func(event ProviderEvent) error {
			if event.Type == ProviderEventDelta {
				close(entered)
				<-unblock
			}
			terminal = terminal || event.Type == ProviderEventDone || event.Type == ProviderEventTool
			return nil
		})
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("callback did not pause")
	}
	generationContext := <-admissions
	if generationContext.Err() != nil {
		t.Fatal("generation-wide authority was released before its callback completed")
	}
	revoke()
	// Wait for the actual admitted generation context to be canceled before
	// releasing its consumer, independently of asynchronous cancellation delivery.
	select {
	case <-generationContext.Done():
	case <-time.After(time.Second):
		t.Fatal("policy did not cancel generation authority")
	}
	release()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || terminal {
			t.Fatal("canceled stream published terminal success", err, terminal)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled generation did not return")
	}
}
