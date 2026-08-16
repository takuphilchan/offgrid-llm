package server

import (
	"net"
	"testing"
)

func TestValidateOutboundURL(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{url: "https://example.com/document"},
		{url: "http://8.8.8.8/document"},
		{url: "file:///etc/passwd", wantErr: true},
		{url: "http://127.0.0.1/private", wantErr: true},
		{url: "http://169.254.169.254/latest/meta-data", wantErr: true},
		{url: "http://10.0.0.1/private", wantErr: true},
		{url: "http://100.64.0.1/private", wantErr: true},
		{url: "http://[::1]/private", wantErr: true},
		{url: "https://user:password@example.com", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			_, err := validateOutboundURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateOutboundURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestIsDisallowedOutboundIP(t *testing.T) {
	for _, rawIP := range []string{"0.0.0.0", "127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.1.1", "169.254.1.1", "100.64.0.1", "::1", "fc00::1", "fe80::1"} {
		if !isDisallowedOutboundIP(net.ParseIP(rawIP)) {
			t.Errorf("expected %s to be disallowed", rawIP)
		}
	}
	if isDisallowedOutboundIP(net.ParseIP("8.8.8.8")) {
		t.Error("expected public address to be allowed")
	}
}
