package server

import (
	"fmt"
	"net"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/config"
)

func serverListenAddress(host string, port int) string {
	if strings.TrimSpace(host) == "" {
		host = "localhost"
	}
	return net.JoinHostPort(host, fmt.Sprintf("%d", port))
}

func isLoopbackHost(host string) bool {
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

func validateServerExposure(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("server configuration is required")
	}
	if !cfg.RequireAuth && !cfg.AllowUnauthenticatedRemote && !isLoopbackHost(cfg.ServerHost) {
		return fmt.Errorf("refusing unauthenticated non-loopback listener %q; enable authentication or explicitly set OFFGRID_ALLOW_UNAUTHENTICATED_REMOTE=true", cfg.ServerHost)
	}
	return nil
}
