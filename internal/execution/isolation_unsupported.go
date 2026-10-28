//go:build !linux

package execution

import "net/http"

func localIsolationClient(string) (*http.Client, error) { return nil, ErrUnsupported }
