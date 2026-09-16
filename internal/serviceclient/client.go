// Package serviceclient is the authenticated transport used by OffGrid clients.
// It deliberately does not install a global HTTP transport: registry/model
// downloads must never inherit the user's OffGrid credentials.
package serviceclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Error struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
	RequestID string `json:"request_id,omitempty"`
	Status    int    `json:"-"`
}

func (e *Error) Error() string { return e.Message }

type Client struct {
	base *url.URL
	key  string
	http *http.Client
}

func New(baseURL, key string, transport *http.Client) (*Client, error) {
	base, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || base == nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" || (base.Path != "" && base.Path != "/") {
		return nil, &Error{Code: "invalid_server", Message: "Server must be an HTTP(S) origin without credentials, a path, or a query."}
	}
	if transport == nil {
		transport = &http.Client{Timeout: 30 * time.Second}
	}
	client := *transport
	if client.Timeout == 0 {
		client.Timeout = 30 * time.Second
	}
	// No redirect is needed by the product API. Even same-host redirects can
	// turn an approved operation into a different action or leak its body.
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{base: base, key: strings.TrimSpace(key), http: &client}, nil
}

func (c *Client) Do(ctx context.Context, method, path, contentType string, body io.Reader) (*http.Response, error) {
	rel, err := url.Parse(path)
	if err != nil || rel.IsAbs() || rel.Host != "" || !strings.HasPrefix(rel.Path, "/") || strings.HasPrefix(rel.Path, "//") || rel.Fragment != "" {
		return nil, &Error{Code: "invalid_endpoint", Message: "Invalid service endpoint."}
	}
	endpoint := *c.base
	endpoint.Path, endpoint.RawPath, endpoint.RawQuery = rel.Path, rel.RawPath, rel.RawQuery
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.key != "" {
		req.Header.Set("Authorization", "Bearer "+c.key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var timed net.Error
		if errors.As(err, &timed) && timed.Timeout() {
			return nil, &Error{Code: "timeout", Message: "The service request timed out. Inspect its state before retrying a mutation.", Retryable: false}
		}
		return nil, &Error{Code: "service_unreachable", Message: "Cannot reach OffGrid. Check the server address and whether the service is running."}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		// Do not echo arbitrary response bodies: old servers and proxies can
		// include secrets, file paths, terminal escapes, or full HTML pages.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		message := fmt.Sprintf("OffGrid returned HTTP %d.", resp.StatusCode)
		code := "service_error"
		switch resp.StatusCode {
		case 401:
			code, message = "unauthenticated", "Authentication required. Set OFFGRID_API_KEY to a valid service API key."
		case 403:
			code, message = "forbidden", "Your account does not have permission for this operation."
		case 404:
			code, message = "not_found", "The requested resource or service endpoint was not found. Check the ID and server version."
		case 409:
			code, message = "conflict", "The resource state changed. Refresh it before continuing."
		case 410:
			code, message = "upgrade_required", "This endpoint has been retired. Update the OffGrid client."
		case 429:
			code, message = "busy", "OffGrid is busy. Wait before submitting another request."
		case 503:
			code, message = "unavailable", "The requested capability is unavailable. Check service diagnostics."
		}
		// Only rejection before admission is advertised as safe to retry.
		return nil, &Error{Status: resp.StatusCode, Code: code, Message: message, Retryable: resp.StatusCode == 429, RequestID: safeRequestID(resp.Header.Get("X-Request-ID"))}
	}
	return resp, nil
}

func (c *Client) JSON(ctx context.Context, method, path string, input, result any) error {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	resp, err := c.Do(ctx, method, path, "application/json", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if result == nil {
		_, err = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return err
	}
	return DecodeContext(ctx, resp.Body, result)
}

func Decode(body io.Reader, result any) error {
	return DecodeContext(context.Background(), body, result)
}

func DecodeContext(ctx context.Context, body io.Reader, result any) error {
	data, err := io.ReadAll(io.LimitReader(body, (16<<20)+1))
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var timeout net.Error
	if errors.As(err, &timeout) && timeout.Timeout() {
		return &Error{Code: "timeout", Message: "The service response timed out. Inspect its state before retrying a mutation."}
	}
	if err != nil || len(data) > 16<<20 || bytes.Equal(bytes.TrimSpace(data), []byte("null")) || json.Unmarshal(data, result) != nil {
		return &Error{Code: "invalid_response", Message: "OffGrid returned an invalid or oversized JSON response."}
	}
	return nil
}

func safeRequestID(value string) string {
	if len(value) > 128 {
		return ""
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return ""
		}
	}
	return value
}
