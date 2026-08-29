package inference

import "testing"

func TestLlamaHTTPPortUsesExplicitIPv4Loopback(t *testing.T) {
	engine := NewLlamaHTTPEngine("")
	if engine.baseURL != "http://127.0.0.1:42382" {
		t.Fatalf("default base URL = %q", engine.baseURL)
	}
	engine.SetPort(43123)
	if engine.baseURL != "http://127.0.0.1:43123" {
		t.Fatalf("base URL after SetPort = %q", engine.baseURL)
	}
}
