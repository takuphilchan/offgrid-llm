package execution

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func localIsolationClient(socket string) (*http.Client, error) {
	resolved, err := filepath.EvalSymlinks(socket)
	if err != nil {
		return nil, ErrHostUnavailable
	}
	info, err := os.Stat(resolved)
	if err != nil || info.Mode()&os.ModeSocket == 0 {
		return nil, ErrAuthority
	}
	transport := &http.Transport{Proxy: nil, MaxConnsPerHost: 64, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, "unix", resolved)
	}}
	return &http.Client{Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, nil
}
