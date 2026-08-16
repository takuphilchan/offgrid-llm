package server

import "testing"

func TestClientIPIgnoresSourcePort(t *testing.T) {
	tests := map[string]string{
		"192.0.2.10:12345":  "192.0.2.10",
		"192.0.2.10:54321":  "192.0.2.10",
		"[2001:db8::1]:443": "2001:db8::1",
		"192.0.2.10":        "192.0.2.10",
	}
	for remoteAddr, want := range tests {
		if got := clientIP(remoteAddr); got != want {
			t.Errorf("clientIP(%q) = %q, want %q", remoteAddr, got, want)
		}
	}
}
