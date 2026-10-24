package server

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/users"
)

// requirePermissionWhenAuthEnabled preserves local single-user mode while
// enforcing role permissions whenever authentication is enabled.
func (s *Server) requirePermissionWhenAuthEnabled(permission users.Permission, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.config != nil && s.config.RequireAuth {
			user := users.GetUser(r)
			if user == nil || !user.HasPermission(permission) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
		}
		next(w, r)
	}
}

const maxRequestBodyBytes int64 = 64 << 20

func requestBodyLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			limit := maxRequestBodyBytes
			if r.Method == http.MethodPost && r.URL.Path == modelPackagePath {
				limit = maxModelPackageRequestBytes
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
		}
		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs all HTTP requests
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Browser hardening for both the web and Electron-hosted React application.
		// Framing policy must be delivered as a response header, not an HTML meta tag.
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self'; connect-src 'self' ws: wss:; media-src 'self' blob:; object-src 'none'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), geolocation=(), payment=(), usb=()")

		// Generate request ID for tracing (if enabled)
		requestID := ""
		if s.config.EnableRequestID {
			// Check if client provided one, otherwise generate
			requestID = r.Header.Get("X-Request-ID")
			if requestID == "" {
				idBytes := make([]byte, 8)
				rand.Read(idBytes)
				requestID = hex.EncodeToString(idBytes)
			}
			w.Header().Set("X-Request-ID", requestID)
		}

		// Set CORS headers for browser access
		// If CORS origins configured, validate; otherwise allow all (simple mode)
		origin := r.Header.Get("Origin")
		if s.config.CORSOrigins != "" {
			// Enterprise mode: validate against allowed origins
			allowed := false
			for _, allowedOrigin := range strings.Split(s.config.CORSOrigins, ",") {
				if strings.TrimSpace(allowedOrigin) == origin {
					allowed = true
					break
				}
			}
			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
		} else {
			// Simple mode: allow all origins (default for local/edge use)
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Track in-flight requests
		s.offgridMetrics.RequestsInFlight.Inc()
		defer s.offgridMetrics.RequestsInFlight.Dec()

		// Wrap response writer to capture status code
		wrapped := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		start := time.Now()
		next.ServeHTTP(wrapped, r)
		duration := time.Since(start)

		// Record metrics
		s.offgridMetrics.RequestsTotal.Inc(r.Method, r.URL.Path, fmt.Sprintf("%d", wrapped.statusCode))
		s.offgridMetrics.RequestDuration.Observe(duration.Seconds(), r.Method, r.URL.Path)

		// Track errors
		if wrapped.statusCode >= 400 {
			s.offgridMetrics.ErrorsTotal.Inc(r.Method, r.URL.Path, fmt.Sprintf("%d", wrapped.statusCode))
			atomic.AddInt64(&s.errorsTotal, 1)
		}

		// Increment request count and latency tracking (atomic)
		atomic.AddInt64(&s.requestCount, 1)
		atomic.AddInt64(&s.totalLatencyMicros, duration.Microseconds())

		// Log with request ID if enabled
		if requestID != "" {
			log.Printf("[%s] %s %s %d · %.2fms", requestID, r.Method, r.URL.Path, wrapped.statusCode, float64(duration.Microseconds())/1000)
		} else {
			log.Printf("%s %s %d · %.2fms", r.Method, r.URL.Path, wrapped.statusCode, float64(duration.Microseconds())/1000)
		}
	})
}

// statusResponseWriter wraps http.ResponseWriter to capture status code
type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

// Preserve ResponseController deadlines/errors through the logging wrapper.
func (w *statusResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *statusResponseWriter) FlushError() error {
	return http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher to support streaming
func (w *statusResponseWriter) Flush() {
	_ = w.FlushError()
}
