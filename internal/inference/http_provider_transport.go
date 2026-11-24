package inference

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"
)

// endpointHTTPClient dials only installation-pinned IPs, with ordinary TLS
// hostname verification. No DNS, environment proxy or redirect can expand scope.
func endpointHTTPClient(endpoint HTTPEndpoint) (*http.Client, error) {
	u, err := ParseHTTPEndpointOrigin(endpoint.URL)
	if err != nil {
		return nil, err
	}
	if endpoint.Remote && u.Scheme != "https" {
		return nil, errors.New("remote inference requires HTTPS")
	}
	if len(endpoint.AllowedIPs) == 0 || len(endpoint.AllowedIPs) > 16 {
		return nil, errors.New("HTTP inference requires bounded pinned dial addresses")
	}
	addresses := make([]string, 0, len(endpoint.AllowedIPs))
	for _, raw := range endpoint.AllowedIPs {
		ip := net.ParseIP(raw)
		if ip == nil || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || endpoint.Remote && ip.IsLoopback() || !endpoint.Remote && !ip.IsLoopback() {
			return nil, errors.New("HTTP inference address does not match endpoint scope")
		}
		addresses = append(addresses, ip.String())
	}
	if endpoint.Remote {
		if err := verifyRemoteEndpointPlacement(addresses); err != nil {
			return nil, err
		}
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil {
		found := false
		for _, candidate := range addresses {
			if candidate == ip.String() {
				found = true
			}
		}
		if !found {
			return nil, errors.New("literal endpoint address is not pinned")
		}
	} else if !endpoint.Remote && u.Hostname() != "localhost" {
		return nil, errors.New("local endpoint hostname must be localhost")
	}
	port := u.Port()
	if port == "" {
		port = "443"
		if u.Scheme == "http" {
			port = "80"
		}
	}
	expected := net.JoinHostPort(u.Hostname(), port)
	transport := &http.Transport{Proxy: nil, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 60 * time.Second, MaxResponseHeaderBytes: 64 << 10, MaxConnsPerHost: 4,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			if network != "tcp" || !strings.EqualFold(address, expected) {
				return nil, errors.New("HTTP inference dial escaped enrolled origin")
			}
			if endpoint.Remote {
				if err := verifyRemoteEndpointPlacement(addresses); err != nil {
					return nil, err
				}
			}
			for _, ip := range addresses {
				conn, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(ip, port))
				if err == nil {
					if endpoint.VerifyConnection != nil {
						if err := endpoint.VerifyConnection(ctx, conn); err != nil {
							conn.Close()
							return nil, errors.New("HTTP inference connection ownership is unavailable")
						}
					}
					return conn, nil
				}
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
			}
			return nil, errors.New("HTTP inference endpoint is unreachable")
		},
	}
	return &http.Client{Transport: transport, Timeout: 15 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, nil
}

func verifyRemoteEndpointPlacement(addresses []string) error {
	local, err := net.InterfaceAddrs()
	if err != nil {
		return errors.New("cannot establish remote inference resource placement")
	}
	return rejectLocalEndpointAddresses(addresses, local)
}

func rejectLocalEndpointAddresses(addresses []string, local []net.Addr) error {
	for _, address := range addresses {
		remote := net.ParseIP(address)
		for _, bound := range local {
			var ip net.IP
			switch value := bound.(type) {
			case *net.IPNet:
				ip = value.IP
			case *net.IPAddr:
				ip = value.IP
			default:
				ip, _, _ = net.ParseCIDR(bound.String())
			}
			if ip != nil && ip.Equal(remote) {
				return errors.New("local interface inference requires an owned resource lease")
			}
		}
	}
	return nil
}
