package serviceclient

import (
	"context"
	"io"
	"net/http"
	"time"
)

// Events observes one existing run; it never submits or retries work. Ordinary
// request timeouts cover the entire response body and must not terminate a
// healthy multi-step stream. Keep a bounded startup and caller cancellation,
// with the same authenticated origin, transport and no-redirect policy.
func (c *Client) Events(ctx context.Context, path string) (*http.Response, error) {
	return c.events(ctx, path, 30*time.Second)
}

func (c *Client) events(ctx context.Context, path string, startup time.Duration) (*http.Response, error) {
	streamctx, cancel := context.WithCancel(ctx)
	timer := time.AfterFunc(startup, cancel)
	httpClient := *c.http
	httpClient.Timeout = 0
	stream := *c
	stream.http = &httpClient
	response, err := stream.Do(streamctx, http.MethodGet, path, "", nil)
	timer.Stop()
	if err != nil {
		startupErr := streamctx.Err()
		cancel()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if startupErr != nil {
			return nil, &Error{Code: "timeout", Message: "The event stream did not start in time. Inspect the saved run before continuing."}
		}
		return nil, err
	}
	response.Body = &eventBody{ReadCloser: response.Body, cancel: cancel}
	return response, nil
}

type eventBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *eventBody) Close() error { b.cancel(); return b.ReadCloser.Close() }
