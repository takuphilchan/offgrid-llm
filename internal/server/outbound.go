package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func validateOutboundURL(rawURL string) (*url.URL, error) {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("URL scheme must be http or https")
	}
	if parsed.Hostname() == "" || parsed.User != nil {
		return nil, fmt.Errorf("URL must contain a host and no credentials")
	}
	if ip := net.ParseIP(strings.Trim(parsed.Hostname(), "[]")); ip != nil && isDisallowedOutboundIP(ip) {
		return nil, fmt.Errorf("URL resolves to a non-public address")
	}
	return parsed, nil
}

func isDisallowedOutboundIP(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		// Carrier-grade NAT space is not considered private by net.IP.IsPrivate,
		// but it is not an appropriate destination for user-supplied URLs.
		if ipv4[0] == 100 && ipv4[1]&0xC0 == 64 {
			return true
		}
	}
	return false
}

func safeOutboundHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, fmt.Errorf("invalid destination address: %w", err)
			}
			addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("resolve destination: %w", err)
			}
			if len(addresses) == 0 {
				return nil, fmt.Errorf("destination has no IP addresses")
			}
			for _, address := range addresses {
				if isDisallowedOutboundIP(address.IP) {
					return nil, fmt.Errorf("destination resolves to non-public address %s", address.IP)
				}
			}
			var lastErr error
			for _, address := range addresses {
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(address.IP.String(), port))
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			return nil, lastErr
		},
		TLSHandshakeTimeout: 10 * time.Second,
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			_, err := validateOutboundURL(req.URL.String())
			return err
		},
	}
}
